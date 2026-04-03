package core

import (
	"testing"
)

func TestGetDiskSpace_currentDir(t *testing.T) {
	info, err := GetDiskSpace(".")
	if err != nil {
		t.Fatalf("GetDiskSpace: %v", err)
	}
	if info.Total <= 0 {
		t.Errorf("total should be positive, got %d", info.Total)
	}
	if info.Available < 0 {
		t.Errorf("available should be non-negative, got %d", info.Available)
	}
	if info.Available > info.Total {
		t.Errorf("available (%d) > total (%d)", info.Available, info.Total)
	}
}

func TestGetDiskSpace_invalidPath(t *testing.T) {
	_, err := GetDiskSpace("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestDiskSpaceInfo_HasEnoughSpace(t *testing.T) {
	info := DiskSpaceInfo{Available: 1000, Total: 2000}
	if !info.HasEnoughSpace(500) {
		t.Error("should have enough space for 500")
	}
	if !info.HasEnoughSpace(1000) {
		t.Error("should have enough space for exactly 1000")
	}
	if info.HasEnoughSpace(1001) {
		t.Error("should NOT have enough space for 1001")
	}
}

func TestDiskSpaceInfo_FormatAvailable(t *testing.T) {
	info := DiskSpaceInfo{Available: 1073741824}
	if info.FormatAvailable() != "1.00 GB" {
		t.Errorf("formatted = %q", info.FormatAvailable())
	}
}

func TestCheckDiskSpaceFor_sufficient(t *testing.T) {
	err := CheckDiskSpaceFor(".", 1)
	if err != nil {
		t.Errorf("should have enough space for 1 byte: %v", err)
	}
}
