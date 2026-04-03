package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplit_happyPath(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "BigGame.pkg")

	data := make([]byte, 10240)
	copy(data, []byte{0x7F, 0x43, 0x4E, 0x54})
	os.WriteFile(srcPath, data, 0644)

	outDir := filepath.Join(dir, "parts")
	os.MkdirAll(outDir, 0755)

	cancel := make(chan struct{})
	parts, err := Split(SplitOptions{
		SourcePath: srcPath,
		OutputDir:  outDir,
		ChunkSize:  3000,
		Format:     SplitPkgpart,
		BufferSize: 512,
		Cancel:     cancel,
	})
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	expectedParts := 4
	if len(parts) != expectedParts {
		t.Fatalf("expected %d parts, got %d", expectedParts, len(parts))
	}

	if filepath.Base(parts[0]) != "BigGame_001.pkgpart" {
		t.Fatalf("expected BigGame_001.pkgpart, got %s", filepath.Base(parts[0]))
	}

	var totalSize int64
	for _, p := range parts {
		info, _ := os.Stat(p)
		totalSize += info.Size()
	}
	if totalSize != 10240 {
		t.Fatalf("total %d != 10240", totalSize)
	}

	valid, _ := ValidatePKG(parts[0])
	if !valid {
		t.Fatal("first part should have valid PKG header")
	}
}

func TestSplit_allFormats(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "Test.pkg")
	os.WriteFile(srcPath, make([]byte, 5000), 0644)

	cases := []struct {
		format    SplitFormat
		firstName string
	}{
		{SplitPkgpart, "Test_001.pkgpart"},
		{SplitPkgUnderN, "Test.pkg_0"},
		{SplitPkgDotNNN, "Test.pkg.001"},
	}

	for _, c := range cases {
		outDir := filepath.Join(dir, c.firstName)
		os.MkdirAll(outDir, 0755)

		cancel := make(chan struct{})
		parts, err := Split(SplitOptions{
			SourcePath: srcPath,
			OutputDir:  outDir,
			ChunkSize:  2000,
			Format:     c.format,
			BufferSize: 512,
			Cancel:     cancel,
		})
		if err != nil {
			t.Fatalf("format %d: %v", c.format, err)
		}
		if filepath.Base(parts[0]) != c.firstName {
			t.Errorf("format %d: first part = %s, want %s", c.format, filepath.Base(parts[0]), c.firstName)
		}
	}
}

func TestSplit_cancellation(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "Game.pkg")
	os.WriteFile(srcPath, make([]byte, 10000), 0644)

	cancel := make(chan struct{})
	close(cancel)

	_, err := Split(SplitOptions{
		SourcePath: srcPath,
		OutputDir:  dir,
		ChunkSize:  3000,
		Format:     SplitPkgpart,
		BufferSize: 512,
		Cancel:     cancel,
	})
	if err == nil || err.Error() != "cancelled" {
		t.Fatalf("expected cancelled, got %v", err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "Game.pkg" {
			t.Errorf("leftover file: %s", e.Name())
		}
	}
}

func TestSplit_thenMergeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "Original.pkg")

	data := make([]byte, 8192)
	copy(data, []byte{0x7F, 0x43, 0x4E, 0x54})
	for i := 4; i < len(data); i++ {
		data[i] = byte(i % 251)
	}
	os.WriteFile(srcPath, data, 0644)

	splitDir := filepath.Join(dir, "split")
	os.MkdirAll(splitDir, 0755)
	cancel := make(chan struct{})
	parts, err := Split(SplitOptions{
		SourcePath: srcPath,
		OutputDir:  splitDir,
		ChunkSize:  3000,
		Format:     SplitPkgUnderN,
		BufferSize: 1024,
		Cancel:     cancel,
	})
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	detected, outputName := DetectParts(parts[1])
	if len(detected) != len(parts) {
		t.Fatalf("detected %d parts, expected %d", len(detected), len(parts))
	}

	mergePath := filepath.Join(dir, outputName)
	cancel2 := make(chan struct{})
	if err := Merge(MergeOptions{
		Parts:      detected,
		OutputPath: mergePath,
		BufferSize: 1024,
		Cancel:     cancel2,
	}); err != nil {
		t.Fatalf("merge: %v", err)
	}

	merged, _ := os.ReadFile(mergePath)
	if len(merged) != len(data) {
		t.Fatalf("size mismatch: %d vs %d", len(merged), len(data))
	}
	for i := range data {
		if merged[i] != data[i] {
			t.Fatalf("byte %d differs: %02x vs %02x", i, merged[i], data[i])
		}
	}

	valid, _ := ValidatePKG(mergePath)
	if !valid {
		t.Fatal("merged file should have valid PKG header")
	}
}
