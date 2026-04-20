package webrtc

import (
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// V4L2Capture captures video from a V4L2 device and encodes it to H.264
type V4L2Capture struct {
	device    string
	width     int
	height    int
	frameRate float64
	cmd       *exec.Cmd
	stdout    io.ReadCloser
	encoder   *FFmpegEncoder
	janus     *JanusClient
	stopChan  chan struct{}
	stopped   bool
	mu        sync.Mutex
	wg        sync.WaitGroup
}

// NewV4L2Capture creates a new V4L2 capture instance
func NewV4L2Capture(device string, width, height int, frameRate float64, janus *JanusClient) *V4L2Capture {
	return &V4L2Capture{
		device:    device,
		width:     width,
		height:    height,
		frameRate: frameRate,
		janus:     janus,
		stopChan:  make(chan struct{}),
	}
}

// Start begins capturing from the V4L2 device
func (v *V4L2Capture) Start() error {
	fmt.Printf("Starting V4L2 capture from %s (%dx%d @ %.2f fps)\n", v.device, v.width, v.height, v.frameRate)

	// Set up FFmpeg encoder for MJPEG -> H.264
	v.encoder = NewFFmpegEncoder(v.width, v.height, v.frameRate)
	if err := v.encoder.Start(); err != nil {
		return fmt.Errorf("failed to start encoder: %w", err)
	}

	// FFmpeg command to capture from V4L2 and output MJPEG
	v.cmd = exec.Command("ffmpeg",
		"-f", "v4l2",
		"-framerate", fmt.Sprintf("%.2f", v.frameRate),
		"-video_size", fmt.Sprintf("%dx%d", v.width, v.height),
		"-input_format", "mjpeg",
		"-i", v.device,
		"-c:v", "copy",
		"-f", "mjpeg",
		"pipe:1",
	)

	var err error
	v.stdout, err = v.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := v.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Log stderr in background
	v.wg.Add(1)
	go func() {
		defer v.wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			fmt.Printf("[v4l2] %s", string(buf[:n]))
		}
	}()

	if err := v.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Start the capture loop
	v.wg.Add(1)
	go v.captureLoop()

	return nil
}

func (v *V4L2Capture) captureLoop() {
	defer func() {
		v.wg.Done()
		fmt.Println("V4L2 capture loop ended")
	}()

	frameDuration := Duration(v.frameRate)
	buf := make([]byte, 65536)

	for {
		select {
		case <-v.stopChan:
			return
		default:
		}

		// Set read deadline for non-blocking read
		if deadliner, ok := v.stdout.(interface {
			SetReadDeadline(time.Time) error
		}); ok {
			deadliner.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		}

		n, err := v.stdout.Read(buf)
		if err != nil {
			if err == io.EOF {
				return
			}
			// Timeout - continue loop
			continue
		}

		if n > 0 {
			// Encode the MJPEG frame to H.264
			h264Data, err := v.encoder.EncodeFrame(buf[:n])
			if err != nil {
				fmt.Printf("V4L2 encoding error: %v\n", err)
				continue
			}

			if h264Data != nil && len(h264Data) > 0 {
				// Write to Janus
				if err := v.janus.WriteVideo(h264Data, frameDuration); err != nil {
					fmt.Printf("V4L2 failed to write video: %v\n", err)
				}
			}
		}
	}
}

// Stop stops the V4L2 capture
func (v *V4L2Capture) Stop() error {
	v.mu.Lock()
	if v.stopped {
		v.mu.Unlock()
		return nil
	}
	v.stopped = true
	v.mu.Unlock()

	close(v.stopChan)

	if v.stdout != nil {
		v.stdout.Close()
	}

	if v.cmd != nil && v.cmd.Process != nil {
		v.cmd.Process.Kill()
		v.cmd.Wait()
	}

	if v.encoder != nil {
		v.encoder.Stop()
	}

	v.wg.Wait()
	fmt.Println("V4L2 capture stopped")
	return nil
}
