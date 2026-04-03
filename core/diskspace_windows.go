//go:build windows

package core

import (
	"syscall"
	"unsafe"
)

// GetDiskSpace returns available and total disk space for the volume containing path.
func GetDiskSpace(path string) (DiskSpaceInfo, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetDiskFreeSpaceExW")

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return DiskSpaceInfo{}, err
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	r, _, callErr := proc.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if r == 0 {
		return DiskSpaceInfo{}, callErr
	}
	return DiskSpaceInfo{
		Available: int64(freeBytesAvailable),
		Total:     int64(totalBytes),
		Path:      path,
	}, nil
}
