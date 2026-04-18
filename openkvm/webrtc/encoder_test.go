package webrtc

import (
	"testing"
	"time"
)

func TestFFmpegEncoderStart(t *testing.T) {
	encoder := NewFFmpegEncoder(1280, 720, 24)
	if encoder == nil {
		t.Fatal("expected non-nil encoder")
	}

	err := encoder.Start()
	if err != nil {
		t.Fatal("failed to start encoder:", err)
	}

	// Give FFmpeg time to initialize
	time.Sleep(500 * time.Millisecond)

	// Stop should work
	err = encoder.Stop()
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}
}

func TestFFmpegEncoderNotStarted(t *testing.T) {
	encoder := NewFFmpegEncoder(640, 480, 30)

	// Try to encode without starting
	_, err := encoder.EncodeFrame([]byte("test"))
	if err == nil {
		t.Fatal("expected error when encoding before start")
	}
	if err.Error() != "encoder not started" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFFmpegEncoderStop(t *testing.T) {
	encoder := NewFFmpegEncoder(640, 480, 30)

	// Stop without starting should not panic
	err := encoder.Stop()
	if err != nil {
		t.Fatalf("stop without start failed: %v", err)
	}

	// Start then stop
	err = encoder.Start()
	if err != nil {
		t.Fatal("failed to start:", err)
	}

	err = encoder.Stop()
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	// Double stop should not panic
	err = encoder.Stop()
	if err != nil {
		t.Fatalf("double stop failed: %v", err)
	}
}

func TestFFmpegEncoderDuration(t *testing.T) {
	d := Duration(24)
	expected := time.Second / 24
	if d != expected {
		t.Fatalf("expected %v, got %v", expected, d)
	}
}
