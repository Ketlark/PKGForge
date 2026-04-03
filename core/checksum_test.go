package core

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateChecksum_happyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	data := []byte("hello world checksum test data 1234567890")
	os.WriteFile(path, data, 0644)

	expected := fmt.Sprintf("%x", sha256.Sum256(data))

	cancel := make(chan struct{})
	var lastProgress float64
	result, err := CalculateChecksum(path, func(p float64) {
		lastProgress = p
	}, cancel)
	if err != nil {
		t.Fatalf("checksum: %v", err)
	}
	if result.SHA256 != expected {
		t.Errorf("sha256 = %q, want %q", result.SHA256, expected)
	}
	if result.Size != int64(len(data)) {
		t.Errorf("size = %d, want %d", result.Size, len(data))
	}
	if lastProgress != 100 {
		t.Errorf("lastProgress = %f, want 100", lastProgress)
	}
	if result.Duration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestCalculateChecksum_cancellation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.bin")
	os.WriteFile(path, make([]byte, 10<<20), 0644) // 10 MB

	cancel := make(chan struct{})
	close(cancel)

	_, err := CalculateChecksum(path, nil, cancel)
	if err == nil || err.Error() != "cancelled" {
		t.Fatalf("expected cancelled, got %v", err)
	}
}

func TestCalculateChecksum_missingFile(t *testing.T) {
	cancel := make(chan struct{})
	_, err := CalculateChecksum("/nonexistent/file", nil, cancel)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
