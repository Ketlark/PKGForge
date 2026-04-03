package core

import "time"

const minBufferSize = 4096

// SpeedETA computes throughput and remaining time from bytes processed,
// total bytes, and elapsed wall-clock time.
func SpeedETA(processed, total int64, start time.Time) (speedBPS, etaSeconds float64) {
	elapsed := time.Since(start).Seconds()
	if elapsed > 0 {
		speedBPS = float64(processed) / elapsed
	}
	if speedBPS > 0 {
		etaSeconds = float64(total-processed) / speedBPS
	}
	return
}

// ClampBuffer ensures the buffer is at least minBufferSize to prevent zero-length reads.
func ClampBuffer(size int) int {
	if size < minBufferSize {
		return minBufferSize
	}
	return size
}

// MergeProgress reports the current state of a merge operation.
type MergeProgress struct {
	CurrentFileIndex int     `json:"currentFileIndex"`
	TotalFiles       int     `json:"totalFiles"`
	BytesProcessed   int64   `json:"bytesProcessed"`
	TotalBytes       int64   `json:"totalBytes"`
	CurrentFileName  string  `json:"currentFileName"`
	SpeedBPS         float64 `json:"speedBPS"`
	ETASeconds       float64 `json:"etaSeconds"`
}

// Percentage returns merge completion as 0–100.
func (p MergeProgress) Percentage() float64 {
	if p.TotalBytes == 0 {
		return 0
	}
	return float64(p.BytesProcessed) / float64(p.TotalBytes) * 100
}

// SplitProgress reports the current state of a split operation.
type SplitProgress struct {
	CurrentPart  int     `json:"currentPart"`
	TotalParts   int     `json:"totalParts"`
	BytesWritten int64   `json:"bytesWritten"`
	TotalBytes   int64   `json:"totalBytes"`
	SpeedBPS     float64 `json:"speedBPS"`
	ETASeconds   float64 `json:"etaSeconds"`
}

// Percentage returns split completion as 0–100.
func (p SplitProgress) Percentage() float64 {
	if p.TotalBytes == 0 {
		return 0
	}
	return float64(p.BytesWritten) / float64(p.TotalBytes) * 100
}
