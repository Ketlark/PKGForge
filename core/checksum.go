package core

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"
)

// ChecksumResult holds the SHA-256 hash and file size.
type ChecksumResult struct {
	SHA256   string  `json:"sha256"`
	Size     int64   `json:"size"`
	Duration float64 `json:"duration"`
}

// CalculateChecksum computes the SHA-256 hash of a file with progress reporting.
func CalculateChecksum(path string, onProgress func(float64), cancel <-chan struct{}) (ChecksumResult, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return ChecksumResult{}, fmt.Errorf("cannot access file: %w", err)
	}
	totalBytes := stat.Size()

	f, err := os.Open(path)
	if err != nil {
		return ChecksumResult{}, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 4<<20) // 4 MB buffer
	var processed int64
	start := time.Now()

	for {
		select {
		case <-cancel:
			return ChecksumResult{}, fmt.Errorf("cancelled")
		default:
		}

		n, readErr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			processed += int64(n)

			if onProgress != nil && totalBytes > 0 {
				onProgress(float64(processed) / float64(totalBytes) * 100)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return ChecksumResult{}, fmt.Errorf("read error: %w", readErr)
		}
	}

	return ChecksumResult{
		SHA256:   fmt.Sprintf("%x", h.Sum(nil)),
		Size:     totalBytes,
		Duration: time.Since(start).Seconds(),
	}, nil
}
