//go:build !windows

package core

import "syscall"

// GetDiskSpace returns available and total disk space for the volume containing path.
func GetDiskSpace(path string) (DiskSpaceInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return DiskSpaceInfo{}, err
	}
	return DiskSpaceInfo{
		Available: int64(stat.Bavail) * int64(stat.Bsize),
		Total:     int64(stat.Blocks) * int64(stat.Bsize),
		Path:      path,
	}, nil
}
