package webrtc

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestFFmpegEncoderEndToEnd tests the FFmpeg encoder with actual video frames
// extracted from a WebM file using FFmpeg
func TestFFmpegEncoderEndToEnd(t *testing.T) {
	// Path to sample video - relative to openkvm directory
	sampleVideo := "../../sample.webm"

	if _, err := os.Stat(sampleVideo); os.IsNotExist(err) {
		t.Skip("sample video not found, skipping end-to-end test")
	}

	// Create a temporary file for extracted frames
	tmpDir, err := os.MkdirTemp("", "encoder-test")
	if err != nil {
		t.Fatal("failed to create temp dir:", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract a few frames from the video using FFmpeg
	frameFiles := make([]string, 0)
	for i := 0; i < 5; i++ {
		frameFile := fmt.Sprintf("%s/frame_%02d.jpg", tmpDir, i)
		cmd := exec.Command("ffmpeg",
			"-y",
			"-ss", fmt.Sprintf("%.2f", float64(i)/10.0), // Seek to different positions
			"-i", sampleVideo,
			"-frames:v", "1",
			"-q:v", "5", // Quality setting for JPEG
			frameFile,
		)
		if err := cmd.Run(); err != nil {
			t.Skip("ffmpeg frame extraction failed:", err)
		}
		frameFiles = append(frameFiles, frameFile)
	}

	if len(frameFiles) == 0 {
		t.Skip("no frames extracted")
	}

	// Start encoder
	encoder := NewFFmpegEncoder(426, 240, 30) // Match sample video dimensions
	if err := encoder.Start(); err != nil {
		t.Fatal("failed to start encoder:", err)
	}
	defer encoder.Stop()

	// Give encoder time to initialize
	time.Sleep(200 * time.Millisecond)

	// Encode each frame
	for i, frameFile := range frameFiles {
		// Read JPEG frame
		jpegData, err := os.ReadFile(frameFile)
		if err != nil {
			t.Fatal("failed to read frame:", err)
		}

		// Optionally decode and verify
		img, err := jpeg.Decode(bytes.NewReader(jpegData))
		if err != nil {
			t.Log("warning: could not decode frame:", err)
			continue
		}

		bounds := img.Bounds()
		t.Logf("frame %d: extracted JPEG size=%d, dimensions=%dx%d",
			i, len(jpegData), bounds.Dx(), bounds.Dy())

		// Encode to H.264
		h264Data, err := encoder.EncodeFrame(jpegData)
		if err != nil {
			t.Fatal("encode frame failed:", err)
		}

		if len(h264Data) > 0 {
			t.Logf("frame %d: encoded to %d bytes of H.264", i, len(h264Data))
		} else {
			t.Logf("frame %d: no H.264 output yet (buffering)", i)
		}
	}
}

// TestVP9ToH264Transcode tests transcoding VP9 to H.264 using FFmpeg directly
// This verifies that FFmpeg can handle the transcoding we need
func TestVP9ToH264Transcode(t *testing.T) {
	sampleVideo := "../../sample.webm"

	if _, err := os.Stat(sampleVideo); os.IsNotExist(err) {
		t.Skip("sample video not found")
	}

	// Create a temporary output file
	tmpFile, err := os.CreateTemp("", "h264_output.mp4")
	if err != nil {
		t.Fatal("failed to create temp file:", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Transcode VP9 to H.264
	cmd := exec.Command("ffmpeg",
		"-y",
		"-i", sampleVideo,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-b:v", "1000k",
		"-frames:v", "30", // Just first 30 frames
		"-an", // No audio
		"-f", "mp4",
		"-flush_packets", "1",
		tmpFile.Name(),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Log("FFmpeg output:", string(output))
		t.Fatal("transcode failed:", err)
	}

	// Check output file
	info, err := os.Stat(tmpFile.Name())
	if err != nil {
		t.Fatal("failed to stat output:", err)
	}

	t.Logf("transcoded %d bytes (VP9 -> H.264)", info.Size())
}

// TestMJPEGToH264Direct tests MJPEG to H.264 conversion with real MJPEG data
func TestMJPEGToH264Direct(t *testing.T) {
	// Generate a simple test pattern as MJPEG
	// Using FFmpeg to create a test pattern directly
	cmd := exec.Command("ffmpeg",
		"-y",
		"-f", "lavfi",
		"-i", "testsrc=size=320x240:rate=10",
		"-c:v", "mjpeg",
		"-frames:v", "10",
		"-f", "image2pipe",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal("failed to create stdout pipe:", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatal("failed to start FFmpeg:", err)
	}

	// Read MJPEG frames
	var frames [][]byte
	buf := make([]byte, 65536)
	for {
		n, err := stdout.Read(buf)
		if err != nil {
			break
		}
		if n > 0 {
			frames = append(frames, make([]byte, n))
			copy(frames[len(frames)-1], buf[:n])
		}
	}
	cmd.Wait()

	if len(frames) == 0 {
		t.Skip("no MJPEG frames generated")
	}

	t.Logf("generated %d MJPEG frames", len(frames))

	// Now test our encoder with these frames
	encoder := NewFFmpegEncoder(320, 240, 10)
	if err := encoder.Start(); err != nil {
		t.Fatal("failed to start encoder:", err)
	}
	defer encoder.Stop()

	time.Sleep(200 * time.Millisecond)

	for i, frame := range frames {
		h264Data, err := encoder.EncodeFrame(frame)
		if err != nil {
			t.Fatal("encode failed:", err)
		}
		if len(h264Data) > 0 {
			t.Logf("frame %d: encoded %d bytes H.264", i, len(h264Data))
		}
	}
}

// Helper function to create a simple JPEG image
func createTestJPEG(width, height int, r, g, b uint8) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
