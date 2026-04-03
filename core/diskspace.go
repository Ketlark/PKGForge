package core

import "fmt"

// DiskSpaceInfo holds available and total space for a path.
type DiskSpaceInfo struct {
	Available int64  `json:"available"`
	Total     int64  `json:"total"`
	Path      string `json:"path"`
}

// HasEnoughSpace returns true if available space exceeds needed bytes.
func (d DiskSpaceInfo) HasEnoughSpace(needed int64) bool {
	return d.Available >= needed
}

// FormatAvailable returns a human-readable available space string.
func (d DiskSpaceInfo) FormatAvailable() string {
	return FormatSize(d.Available)
}

// CheckDiskSpaceFor verifies there is enough room and returns an error if not.
func CheckDiskSpaceFor(path string, neededBytes int64) error {
	info, err := GetDiskSpace(path)
	if err != nil {
		return fmt.Errorf("cannot check disk space: %w", err)
	}
	if !info.HasEnoughSpace(neededBytes) {
		return fmt.Errorf("insufficient disk space: need %s, have %s",
			FormatSize(neededBytes), FormatSize(info.Available))
	}
	return nil
}
