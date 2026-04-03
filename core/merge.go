package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// MergeOptions configures a merge operation.
type MergeOptions struct {
	Parts      []string
	OutputPath string
	BufferSize int
	OnProgress func(MergeProgress)
	Cancel     <-chan struct{}
}

// Merge concatenates split PKG parts into a single output file.
// It validates total size after writing and cleans up on failure.
func Merge(opts MergeOptions) error {
	var totalBytes int64
	for _, p := range opts.Parts {
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("cannot access %s: %w", filepath.Base(p), err)
		}
		totalBytes += info.Size()
	}

	out, err := os.Create(opts.OutputPath)
	if err != nil {
		return fmt.Errorf("cannot create output: %w", err)
	}

	cleanup := func() {
		out.Close()
		os.Remove(opts.OutputPath)
	}

	buf := make([]byte, ClampBuffer(opts.BufferSize))
	var processed int64
	start := time.Now()

	for i, partPath := range opts.Parts {
		f, err := os.Open(partPath)
		if err != nil {
			cleanup()
			return fmt.Errorf("cannot open %s: %w", filepath.Base(partPath), err)
		}

		for {
			select {
			case <-opts.Cancel:
				f.Close()
				cleanup()
				return fmt.Errorf("cancelled")
			default:
			}

			n, readErr := f.Read(buf)
			if n > 0 {
				if _, wErr := out.Write(buf[:n]); wErr != nil {
					f.Close()
					cleanup()
					return fmt.Errorf("write error: %w", wErr)
				}
				processed += int64(n)

				if opts.OnProgress != nil {
					speed, eta := SpeedETA(processed, totalBytes, start)
					opts.OnProgress(MergeProgress{
						CurrentFileIndex: i,
						TotalFiles:       len(opts.Parts),
						BytesProcessed:   processed,
						TotalBytes:       totalBytes,
						CurrentFileName:  filepath.Base(partPath),
						SpeedBPS:         speed,
						ETASeconds:       eta,
					})
				}
			}

			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				f.Close()
				cleanup()
				return fmt.Errorf("read error on %s: %w", filepath.Base(partPath), readErr)
			}
		}
		f.Close()
	}

	if err := out.Close(); err != nil {
		os.Remove(opts.OutputPath)
		return fmt.Errorf("error finalizing output: %w", err)
	}

	info, err := os.Stat(opts.OutputPath)
	if err != nil {
		return fmt.Errorf("cannot verify output: %w", err)
	}
	if info.Size() != totalBytes {
		return fmt.Errorf("size mismatch: expected %s, got %s",
			FormatSize(totalBytes), FormatSize(info.Size()))
	}

	return nil
}
