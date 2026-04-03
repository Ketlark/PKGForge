package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SplitOptions configures a split operation.
type SplitOptions struct {
	SourcePath string
	OutputDir  string
	ChunkSize  int64
	Format     SplitFormat
	BufferSize int
	OnProgress func(SplitProgress)
	Cancel     <-chan struct{}
}

// Split divides a PKG file into numbered chunks.
func Split(opts SplitOptions) ([]string, error) {
	srcInfo, err := os.Stat(opts.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("cannot access source: %w", err)
	}
	totalBytes := srcInfo.Size()

	if opts.ChunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be positive")
	}
	if opts.ChunkSize >= totalBytes {
		return nil, fmt.Errorf("chunk size (%s) >= file size (%s)",
			FormatSize(opts.ChunkSize), FormatSize(totalBytes))
	}

	totalParts := int((totalBytes + opts.ChunkSize - 1) / opts.ChunkSize)
	baseName := filepath.Base(opts.SourcePath)
	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)

	src, err := os.Open(opts.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open source: %w", err)
	}
	defer src.Close()

	buf := make([]byte, ClampBuffer(opts.BufferSize))
	var written int64
	var outputParts []string
	start := time.Now()

	cleanupAll := func() {
		for _, p := range outputParts {
			os.Remove(p)
		}
	}

	for part := 0; part < totalParts; part++ {
		name := splitPartName(stem, ext, part, opts.Format)
		partPath := filepath.Join(opts.OutputDir, name)
		outputParts = append(outputParts, partPath)

		out, err := os.Create(partPath)
		if err != nil {
			cleanupAll()
			return nil, fmt.Errorf("cannot create %s: %w", name, err)
		}

		bytesForPart := opts.ChunkSize
		if part == totalParts-1 {
			bytesForPart = totalBytes - written
		}

		var partWritten int64
		for partWritten < bytesForPart {
			select {
			case <-opts.Cancel:
				out.Close()
				cleanupAll()
				return nil, fmt.Errorf("cancelled")
			default:
			}

			toRead := bytesForPart - partWritten
			if toRead > int64(opts.BufferSize) {
				toRead = int64(opts.BufferSize)
			}

			n, readErr := src.Read(buf[:toRead])
			if n > 0 {
				if _, wErr := out.Write(buf[:n]); wErr != nil {
					out.Close()
					cleanupAll()
					return nil, fmt.Errorf("write error: %w", wErr)
				}
				partWritten += int64(n)
				written += int64(n)

				if opts.OnProgress != nil {
					speed, eta := SpeedETA(written, totalBytes, start)
					opts.OnProgress(SplitProgress{
						CurrentPart:  part,
						TotalParts:   totalParts,
						BytesWritten: written,
						TotalBytes:   totalBytes,
						SpeedBPS:     speed,
						ETASeconds:   eta,
					})
				}
			}
			if readErr != nil {
				out.Close()
				if readErr == io.EOF {
					break
				}
				cleanupAll()
				return nil, fmt.Errorf("read error: %w", readErr)
			}
		}
		out.Close()
	}

	return outputParts, nil
}

func splitPartName(stem, ext string, index int, format SplitFormat) string {
	switch format {
	case SplitPkgpart:
		return fmt.Sprintf("%s_%03d.pkgpart", stem, index+1)
	case SplitPkgUnderN:
		return fmt.Sprintf("%s%s_%d", stem, ext, index)
	case SplitPkgDotNNN:
		return fmt.Sprintf("%s%s.%03d", stem, ext, index+1)
	}
	return fmt.Sprintf("%s_%03d", stem+ext, index)
}
