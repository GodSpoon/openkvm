package factory

import (
	"github.com/allape/openkvm/config"
	"github.com/allape/openkvm/kvm/video"
	"github.com/allape/openkvm/webrtc"
)

func WebRTCStreamerFromConfig(conf config.Config, v video.Driver) (*webrtc.Streamer, error) {
	if !conf.WebRTC.Enabled {
		l.Info().Println("WebRTC is disabled")
		return nil, nil
	}

	janus, err := webrtc.NewJanusClient(conf.WebRTC)
	if err != nil {
		return nil, err
	}

	streamer := webrtc.NewStreamer(janus, v, conf.Video.FrameRate)

	return streamer, nil
}
