package webrtc

import (
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type FFmpegEncoder struct {
	width     int
	height    int
	frameRate float64
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stopChan  chan struct{}
	stopped   bool
	mu        sync.Mutex
	wg        sync.WaitGroup
}

func NewFFmpegEncoder(width, height int, frameRate float64) *FFmpegEncoder {
	return &FFmpegEncoder{
		width:     width,
		height:    height,
		frameRate: frameRate,
		stopChan:  make(chan struct{}),
	}
}

func (e *FFmpegEncoder) Start() error {
	// FFmpeg command to encode MJPEG from stdin to H.264
	// Using libx264 with ultrafast settings for low latency
	e.cmd = exec.Command("ffmpeg",
		"-re",                      // Read input at native frame rate
		"-fflags", "nobuffer",     // Disable buffering for low latency
		"-flags", "low_delay",     // Low delay mode
		"-f", "mjpeg",             // Input format - MJPEG stream
		"-i", "pipe:0",           // Read from stdin
		"-c:v", "libx264",        // H.264 codec
		"-preset", "ultrafast",    // Low latency preset
		"-tune", "zerolatency",   // Zero latency tuning
		"-b:v", "2500k",          // Bitrate
		"-max_delay", "500000",    // Max demux delay (500ms)
		"-bufsize", "5000k",      // Buffer size
		"-an",                     // No audio
		"-f", "h264",             // Output format
		"-flush_packets", "1",     // Flush packets immediately
		"pipe:1",                  // Write to stdout
	)

	var err error
	e.stdin, err = e.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	e.stdout, err = e.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := e.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Log stderr in background - don't block
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
				fmt.Printf("[encoder] %s", string(buf[:n]))
		}
	}()

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	fmt.Printf("FFmpeg H.264 encoder started (%dx%d @ %.2f fps)\n", e.width, e.height, e.frameRate)
	return nil
}

func (e *FFmpegEncoder) EncodeFrame(mjpegData []byte) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stdin == nil || e.stopped {
		return nil, fmt.Errorf("encoder not started")
	}

	// Write MJPEG frame to FFmpeg stdin
	_, err := e.stdin.Write(mjpegData)
	if err != nil {
		return nil, fmt.Errorf("failed to write frame: %w", err)
	}

	// Read H.264 output with timeout
	// Use SetReadDeadline for non-blocking read
	if deadliner, ok := e.stdout.(interface {
		SetReadDeadline(time.Time) error
	}); ok {
		deadliner.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	}

	buf := make([]byte, 65536)
	n, err := e.stdout.Read(buf)
	if err != nil {
		if err == io.EOF {
			return nil, nil
		}
		// Timeout or other error - this is expected if FFmpeg is still encoding
		return nil, nil
	}

	if n > 0 {
		return buf[:n], nil
	}

	return nil, nil
}

func (e *FFmpegEncoder) Stop() error {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return nil
	}
	e.stopped = true
	e.mu.Unlock()

	close(e.stopChan)

	if e.stdin != nil {
		e.stdin.Close()
	}

	if e.stdout != nil {
		e.stdout.Close()
	}

	if e.cmd != nil && e.cmd.Process != nil {
		e.cmd.Process.Kill()
		e.cmd.Wait()
	}

	e.wg.Wait()

	fmt.Println("FFmpeg encoder stopped")
	return nil
}

// Duration returns the duration for a frame at the given frame rate
func Duration(frameRate float64) time.Duration {
	return time.Duration(float64(time.Second) / frameRate)
}
