package webrtc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/allape/openkvm/config"
	janus "github.com/notedit/janus-go"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

type JanusClient struct {
	gateway    *janus.Gateway
	session    *janus.Session
	handle     *janus.Handle
	peerConn   *webrtc.PeerConnection
	videoTrack *webrtc.TrackLocalStaticSample
	audioTrack *webrtc.TrackLocalStaticSample
	config     config.WebRTC
	events     chan interface{}
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewJanusClient(cfg config.WebRTC) (*JanusClient, error) {
	c := &JanusClient{
		config: cfg,
		events: make(chan interface{}),
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())

	gateway, err := janus.Connect(cfg.JanusURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Janus: %w", err)
	}
	c.gateway = gateway

	session, err := gateway.Create()
	if err != nil {
		return nil, fmt.Errorf("failed to create Janus session: %w", err)
	}
	c.session = session

	handle, err := session.Attach("janus.plugin.videoroom")
	if err != nil {
		return nil, fmt.Errorf("failed to attach to videoroom plugin: %w", err)
	}
	c.handle = handle

	return c, nil
}

func (c *JanusClient) Start() error {
	c.wg.Add(1)
	go c.watchHandle()

	c.wg.Add(1)
	go c.keepAlive()

	_, err := c.handle.Message(map[string]any{
		"request": "join",
		"ptype":   "publisher",
		"room":    c.config.Room,
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to join room: %w", err)
	}

	if err := c.createPeerConnection(); err != nil {
		return err
	}

	if err := c.createMediaTracks(); err != nil {
		return err
	}

	return c.publish()
}

func (c *JanusClient) createPeerConnection() error {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
	})
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}
	c.peerConn = peerConnection

	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State: %s\n", state.String())
	})

	return nil
}

func (c *JanusClient) createMediaTracks() error {
	var err error
	c.videoTrack, err = webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: "video/h264"},
		"video",
		"pion",
	)
	if err != nil {
		return fmt.Errorf("failed to create video track: %w", err)
	}

	if _, err = c.peerConn.AddTrack(c.videoTrack); err != nil {
		return fmt.Errorf("failed to add video track: %w", err)
	}

	c.audioTrack, err = webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: "audio/opus"},
		"audio",
		"pion",
	)
	if err != nil {
		return fmt.Errorf("failed to create audio track: %w", err)
	}

	if _, err = c.peerConn.AddTrack(c.audioTrack); err != nil {
		return fmt.Errorf("failed to add audio track: %w", err)
	}

	return nil
}

func (c *JanusClient) publish() error {
	offer, err := c.peerConn.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(c.peerConn)
	if err = c.peerConn.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("failed to set local description: %w", err)
	}
	<-gatherComplete

	msg, err := c.handle.Message(map[string]any{
		"request": "publish",
		"audio":   true,
		"video":   true,
		"data":    false,
	}, map[string]any{
		"type":    "offer",
		"sdp":     c.peerConn.LocalDescription().SDP,
		"trickle": false,
	})
	if err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	if msg.Jsep != nil {
		sdpVal, ok := msg.Jsep["sdp"].(string)
		if !ok {
			return fmt.Errorf("failed to cast SDP")
		}
		err = c.peerConn.SetRemoteDescription(webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer,
			SDP:  sdpVal,
		})
		if err != nil {
			return fmt.Errorf("failed to set remote description: %w", err)
		}
	}

	return nil
}

func (c *JanusClient) watchHandle() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		case msg := <-c.handle.Events:
			switch m := msg.(type) {
			case *janus.WebRTCUpMsg:
				fmt.Printf("WebRTCUp: session=%d handle=%d\n", m.Session, m.Handle)
			case *janus.SlowLinkMsg:
				fmt.Printf("SlowLink: uplink=%v lost=%d\n", m.Uplink, m.Lost)
			case *janus.MediaMsg:
				fmt.Printf("Media: %s receiving=%v\n", m.Type, m.Receiving)
			case *janus.HangupMsg:
				fmt.Printf("Hangup: session=%d handle=%d reason=%s\n", m.Session, m.Handle, m.Reason)
			case *janus.EventMsg:
				fmt.Printf("Event: %+v\n", m.Plugindata.Data)
			}
		}
	}
}

func (c *JanusClient) keepAlive() {
	defer c.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if _, err := c.session.KeepAlive(); err != nil {
				fmt.Printf("KeepAlive error: %v\n", err)
			}
		}
	}
}

func (c *JanusClient) Stop() error {
	c.cancel()
	c.wg.Wait()

	if c.peerConn != nil {
		c.peerConn.Close()
	}
	if c.handle != nil {
		c.handle.Detach()
	}

	return nil
}

func (c *JanusClient) WriteVideo(data []byte, duration time.Duration) error {
	if c.videoTrack == nil {
		return nil
	}
	return c.videoTrack.WriteSample(media.Sample{Data: data, Duration: duration})
}

func (c *JanusClient) WriteAudio(data []byte, duration time.Duration) error {
	if c.audioTrack == nil {
		return nil
	}
	return c.audioTrack.WriteSample(media.Sample{Data: data, Duration: duration})
}
