package fpkg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPS2DiscImageReadsCueReferencedBin(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "GAME.BIN")
	cuePath := filepath.Join(dir, "GAME.CUE")

	binData := bytes.Repeat([]byte{0x12, 0x34, 0x56, 0x78}, 2352/4*2)
	if err := os.WriteFile(binPath, binData, 0644); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	cue := `FILE "GAME.BIN" BINARY
  TRACK 01 MODE2/2352
    INDEX 01 00:00:00
`
	if err := os.WriteFile(cuePath, []byte(cue), 0644); err != nil {
		t.Fatalf("write cue: %v", err)
	}

	resolvedCue, err := companionCuePath(binPath)
	if err != nil {
		t.Fatalf("resolve cue: %v", err)
	}
	resolvedInfo, err := os.Stat(resolvedCue)
	if err != nil {
		t.Fatalf("stat resolved cue: %v", err)
	}
	expectedInfo, err := os.Stat(cuePath)
	if err != nil {
		t.Fatalf("stat expected cue: %v", err)
	}
	if !os.SameFile(resolvedInfo, expectedInfo) {
		t.Fatalf("resolved cue mismatch: got %q, want %q", resolvedCue, cuePath)
	}

	data, needsLIMG, sectorSize, err := loadPS2DiscImage(cuePath, 1, filepath.Join(dir, "tmp"))
	if err != nil {
		t.Fatalf("load cue image: %v", err)
	}
	if !needsLIMG {
		t.Fatal("expected CUE/BIN image to require LIMG")
	}
	if sectorSize != 2352 {
		t.Fatalf("sector size mismatch: got %d, want 2352", sectorSize)
	}
	if !bytes.Equal(data, binData) {
		t.Fatal("loaded image data should be the referenced BIN, not the CUE text")
	}
}
