package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSuggestRename_validPKG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "random.pkg")
	header := makePKGHeader("EP9000-CUSA00001_00-GODOFWAR00000000", 0x1A)
	os.WriteFile(path, header, 0644)

	name, info := SuggestRename(path)
	if !info.Valid {
		t.Fatalf("expected valid PKG")
	}
	if name != "CUSA00001-PS4_Game.pkg" {
		t.Errorf("suggested = %q, want CUSA00001-PS4_Game.pkg", name)
	}
}

func TestSuggestRename_DLC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dlc.pkg")
	header := makePKGHeader("UP0001-CUSA12345_00-DLC0000000000001", 0x1C)
	os.WriteFile(path, header, 0644)

	name, _ := SuggestRename(path)
	if name != "CUSA12345-PS4_DLC.pkg" {
		t.Errorf("suggested = %q", name)
	}
}

func TestSuggestRename_invalidPKG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.bin")
	os.WriteFile(path, []byte("not a pkg"), 0644)

	name, info := SuggestRename(path)
	if info.Valid {
		t.Fatal("should be invalid")
	}
	if name != "bad.bin" {
		t.Errorf("should return original name, got %q", name)
	}
}

func TestRenamePKG_happyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "random.pkg")
	header := makePKGHeader("EP9000-CUSA00001_00-GODOFWAR00000000", 0x1A)
	os.WriteFile(path, header, 0644)

	newPath, err := RenamePKG(path)
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if filepath.Base(newPath) != "CUSA00001-PS4_Game.pkg" {
		t.Errorf("newPath = %q", newPath)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("new file should exist: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("old file should not exist")
	}
}

func TestRenamePKG_targetExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "random.pkg")
	header := makePKGHeader("EP9000-CUSA00001_00-GODOFWAR00000000", 0x1A)
	os.WriteFile(path, header, 0644)
	os.WriteFile(filepath.Join(dir, "CUSA00001-PS4_Game.pkg"), []byte("existing"), 0644)

	_, err := RenamePKG(path)
	if err == nil {
		t.Fatal("expected error when target exists")
	}
}
