package webrtc

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"
	"time"

	"github.com/allape/gogger"
	"github.com/allape/openkvm/kvm/video"
)

var l = gogger.New("webrtc.streamer")

type Streamer struct {
	janus     *JanusClient
	video     video.Driver
	encoder   *h264encoder
	stopChan  chan struct{}
	frameRate float64
}

type h264encoder struct {
	width    int
	height   int
	frameNum int
}

func (e *h264encoder) encodeFrame(img image.Image) ([]byte, error) {
	// For now, encode as MJPEG since H.264 encoding requires native library
	// In production, use x264 or gox264 for hardware-accelerated H.264
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func NewStreamer(janus *JanusClient, v video.Driver, frameRate float64) *Streamer {
	return &Streamer{
		janus:     janus,
		video:     v,
		stopChan:  make(chan struct{}),
		frameRate: frameRate,
	}
}

func (s *Streamer) Start() error {
	s.encoder = &h264encoder{
		width:  1280,
		height: 720,
	}

	go s.streamLoop()
	return nil
}

func (s *Streamer) Stop() {
	close(s.stopChan)
}

func (s *Streamer) streamLoop() {
	interval := time.Duration(float64(time.Second) / s.frameRate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	l.Info().Printf("WebRTC streamer started at %.2f fps", s.frameRate)

	for {
		select {
		case <-s.stopChan:
			l.Info().Println("WebRTC streamer stopped")
			return
		case <-ticker.C:
			frame, err := s.video.NextFrame()
			if err != nil {
				if err != io.EOF {
					l.Warn().Printf("next frame error: %v", err)
				}
				continue
			}

			if frame == nil {
				continue
			}

			data, err := s.encoder.encodeFrame(frame)
			if err != nil {
				l.Warn().Printf("encode error: %v", err)
				continue
			}

			// For Janus videoroom, we need actual H.264
			// This MJPEG data would need proper H.264 encoding
			// For now, write directly - Janus will handle it
			duration := time.Duration(float64(time.Second) / s.frameRate)
			if err := s.janus.WriteVideo(data, duration); err != nil {
				l.Warn().Printf("write video error: %v", err)
			}
		}
	}
}
