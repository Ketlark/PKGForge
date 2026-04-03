package core

import (
	"testing"
	"time"
)

func TestClampBuffer(t *testing.T) {
	if ClampBuffer(0) != minBufferSize {
		t.Errorf("ClampBuffer(0) = %d, want %d", ClampBuffer(0), minBufferSize)
	}
	if ClampBuffer(100) != minBufferSize {
		t.Errorf("ClampBuffer(100) = %d, want %d", ClampBuffer(100), minBufferSize)
	}
	if ClampBuffer(1 << 20) != 1<<20 {
		t.Errorf("ClampBuffer(1MB) should pass through")
	}
}

func TestSpeedETA(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	speed, eta := SpeedETA(1000, 2000, start)
	if speed < 400 || speed > 600 {
		t.Errorf("speed = %f, expected ~500", speed)
	}
	if eta < 1.5 || eta > 2.5 {
		t.Errorf("eta = %f, expected ~2", eta)
	}
}

func TestSpeedETA_noElapsed(t *testing.T) {
	speed, eta := SpeedETA(0, 1000, time.Now())
	if speed != 0 {
		t.Errorf("speed should be 0 at start, got %f", speed)
	}
	if eta != 0 {
		t.Errorf("eta should be 0 at start, got %f", eta)
	}
}

func TestFormatSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
	}
	for _, c := range cases {
		if got := FormatSize(c.in); got != c.want {
			t.Errorf("FormatSize(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatTime(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "00:00"},
		{65, "01:05"},
		{3661, "01:01:01"},
		{-1, "--:--"},
		{999999, "--:--"},
	}
	for _, c := range cases {
		if got := FormatTime(c.in); got != c.want {
			t.Errorf("FormatTime(%f) = %q, want %q", c.in, got, c.want)
		}
	}
}
