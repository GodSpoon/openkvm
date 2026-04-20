package webrtc

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
	"time"

	"github.com/allape/openkvm/kvm/video"
)

type Streamer struct {
	janus     *JanusClient
	video     video.Driver
	encoder   *FFmpegEncoder
	stopChan  chan struct{}
	frameRate float64
	width     int
	height    int
}

func NewStreamer(janus *JanusClient, v video.Driver, frameRate float64) *Streamer {
	return &Streamer{
		janus:     janus,
		video:     v,
		stopChan:  make(chan struct{}),
		frameRate: frameRate,
		width:     1280,
		height:    720,
	}
}

func (s *Streamer) Start() error {
	s.encoder = NewFFmpegEncoder(s.width, s.height, s.frameRate)
	if err := s.encoder.Start(); err != nil {
		return err
	}

	go s.streamLoop()
	return nil
}

func (s *Streamer) Stop() {
	close(s.stopChan)
	if s.encoder != nil {
		s.encoder.Stop()
	}
}

func (s *Streamer) streamLoop() {
	defer func() {
		fmt.Println("WebRTC streamer loop ended")
	}()

	interval := time.Duration(float64(time.Second) / s.frameRate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Printf("WebRTC streamer started at %.2f fps\n", s.frameRate)

	for {
		select {
		case <-s.stopChan:
			fmt.Println("WebRTC streamer stopped")
			return
		case <-ticker.C:
			frame, err := s.video.NextFrame()
			if err != nil {
				if err != io.EOF {
					fmt.Printf("next frame error: %v\n", err)
				}
				continue
			}

			if frame == nil {
				continue
			}

			// Encode image to MJPEG first, then FFmpeg encodes to H.264
			var buf bytes.Buffer
			err = jpeg.Encode(&buf, frame, &jpeg.Options{Quality: 85})
			if err != nil {
				fmt.Printf("jpeg encode error: %v\n", err)
				continue
			}

			h264Data, err := s.encoder.EncodeFrame(buf.Bytes())
			if err != nil {
				fmt.Printf("h264 encode error: %v\n", err)
				continue
			}

			if h264Data == nil || len(h264Data) == 0 {
				continue
			}

			duration := time.Duration(float64(time.Second) / s.frameRate)
			if err := s.janus.WriteVideo(h264Data, duration); err != nil {
				fmt.Printf("write video error: %v\n", err)
			}
		}
	}
}
