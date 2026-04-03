package core

import (
	"os"
	"path/filepath"
	"testing"
)

func makePKGHeader(contentID string, contentType uint32) []byte {
	header := make([]byte, 0x100)
	// Magic
	header[0], header[1], header[2], header[3] = 0x7F, 0x43, 0x4E, 0x54
	// Content ID at offset 0x40
	copy(header[0x40:0x64], contentID)
	// Content type at offset 0x74 (big-endian)
	header[0x74] = byte(contentType >> 24)
	header[0x75] = byte(contentType >> 16)
	header[0x76] = byte(contentType >> 8)
	header[0x77] = byte(contentType)
	// DRM type at 0x70 = PS4 (0x01)
	header[0x73] = 0x01
	return header
}

func TestInspectPKG_validPS4Game(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.pkg")
	header := makePKGHeader("EP9000-CUSA00001_00-GODOFWAR00000000", 0x1A)
	os.WriteFile(path, header, 0644)

	info := InspectPKG(path)
	if !info.Valid {
		t.Fatalf("expected valid, got error: %s", info.Error)
	}
	if info.ContentID != "EP9000-CUSA00001_00-GODOFWAR00000000" {
		t.Errorf("contentId = %q", info.ContentID)
	}
	if info.TitleID != "CUSA00001" {
		t.Errorf("titleId = %q", info.TitleID)
	}
	if info.Region != "Europe" {
		t.Errorf("region = %q", info.Region)
	}
	if info.ContentType != "PS4 Game" {
		t.Errorf("contentType = %q", info.ContentType)
	}
	if info.DRMType != "PS4" {
		t.Errorf("drmType = %q", info.DRMType)
	}
}

func TestInspectPKG_USRegion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.pkg")
	header := makePKGHeader("UP0001-CUSA12345_00-TESTGAME12345678", 0x1B)
	os.WriteFile(path, header, 0644)

	info := InspectPKG(path)
	if info.Region != "USA" {
		t.Errorf("region = %q, want USA", info.Region)
	}
	if info.ContentType != "PS4 Game Patch" {
		t.Errorf("contentType = %q", info.ContentType)
	}
}

func TestInspectPKG_DLC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dlc.pkg")
	header := makePKGHeader("JP0001-CUSA99999_00-DLC0000000000001", 0x1C)
	os.WriteFile(path, header, 0644)

	info := InspectPKG(path)
	if info.ContentType != "PS4 DLC" {
		t.Errorf("contentType = %q", info.ContentType)
	}
	if info.Region != "Japan" {
		t.Errorf("region = %q", info.Region)
	}
}

func TestInspectPKG_invalidHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pkg")
	os.WriteFile(path, []byte("not a pkg file"), 0644)

	info := InspectPKG(path)
	if info.Valid {
		t.Fatal("expected invalid")
	}
	if info.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestInspectPKG_missingFile(t *testing.T) {
	info := InspectPKG("/nonexistent/file.pkg")
	if info.Valid {
		t.Fatal("expected invalid for missing file")
	}
}

func TestInspectPKG_tooSmall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.pkg")
	os.WriteFile(path, []byte{0x7F, 0x43}, 0644)

	info := InspectPKG(path)
	if info.Valid {
		t.Fatal("expected invalid for tiny file")
	}
}

func TestInspectPKG_unknownRegion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.pkg")
	header := makePKGHeader("XX0001-CUSA00001_00-UNKNOWN000000000", 0x1A)
	os.WriteFile(path, header, 0644)

	info := InspectPKG(path)
	if info.Region != "XX" {
		t.Errorf("region = %q, want XX (passthrough)", info.Region)
	}
}

func TestInspectPKG_unknownContentType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.pkg")
	header := makePKGHeader("EP9000-CUSA00001_00-GAME000000000000", 0xFF)
	os.WriteFile(path, header, 0644)

	info := InspectPKG(path)
	if info.ContentType == "" {
		t.Error("contentType should have a fallback for unknown types")
	}
}

func TestInspectPKG_shortContentID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.pkg")
	header := makePKGHeader("AB", 0x1A)
	os.WriteFile(path, header, 0644)

	info := InspectPKG(path)
	if !info.Valid {
		t.Fatal("should be valid even with short content ID")
	}
	if info.TitleID != "" {
		t.Errorf("titleId should be empty for short content ID, got %q", info.TitleID)
	}
}

func TestInspectPKG_fileSizeTracked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game.pkg")
	header := makePKGHeader("EP9000-CUSA00001_00-GAME000000000000", 0x1A)
	os.WriteFile(path, header, 0644)

	info := InspectPKG(path)
	if info.FileSize != int64(len(header)) {
		t.Errorf("fileSize = %d, want %d", info.FileSize, len(header))
	}
}
