package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMerge_happyPath(t *testing.T) {
	dir := t.TempDir()

	header := append([]byte{0x7F, 0x43, 0x4E, 0x54}, make([]byte, 1020)...)
	os.WriteFile(filepath.Join(dir, "Game.pkg"), header, 0644)
	os.WriteFile(filepath.Join(dir, "Game.pkg_0"), make([]byte, 2048), 0644)
	os.WriteFile(filepath.Join(dir, "Game.pkg_1"), make([]byte, 2048), 0644)
	os.WriteFile(filepath.Join(dir, "Game.pkg_2"), make([]byte, 1536), 0644)

	parts, _ := DetectParts(filepath.Join(dir, "Game.pkg_1"))
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}

	output := filepath.Join(dir, "merged.pkg")
	cancel := make(chan struct{})
	err := Merge(MergeOptions{
		Parts:      parts,
		OutputPath: output,
		BufferSize: 512,
		Cancel:     cancel,
	})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	info, _ := os.Stat(output)
	if info.Size() != 1024+2048+2048+1536 {
		t.Fatalf("size: %d", info.Size())
	}

	valid, _ := ValidatePKG(output)
	if !valid {
		t.Fatal("expected valid PKG header")
	}
}

func TestMerge_cancellation(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.pkg"), make([]byte, 4096), 0644)
	os.WriteFile(filepath.Join(dir, "b.pkg"), make([]byte, 4096), 0644)

	cancel := make(chan struct{})
	close(cancel)

	output := filepath.Join(dir, "out.pkg")
	err := Merge(MergeOptions{
		Parts:      []string{filepath.Join(dir, "a.pkg"), filepath.Join(dir, "b.pkg")},
		OutputPath: output,
		BufferSize: 512,
		Cancel:     cancel,
	})
	if err == nil || err.Error() != "cancelled" {
		t.Fatalf("expected cancelled, got %v", err)
	}
	if _, e := os.Stat(output); !os.IsNotExist(e) {
		t.Fatal("partial output not cleaned up")
	}
}
