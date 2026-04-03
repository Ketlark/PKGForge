package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectParts_pkgUnderN(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "Game.pkg"), make([]byte, 100), 0644)
	os.WriteFile(filepath.Join(dir, "Game.pkg_0"), make([]byte, 200), 0644)
	os.WriteFile(filepath.Join(dir, "Game.pkg_1"), make([]byte, 200), 0644)
	os.WriteFile(filepath.Join(dir, "Game.pkg_2"), make([]byte, 150), 0644)

	parts, name := DetectParts(filepath.Join(dir, "Game.pkg_1"))
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}
	if name != "Game.pkg" {
		t.Fatalf("expected Game.pkg, got %s", name)
	}
	if filepath.Base(parts[0]) != "Game.pkg" {
		t.Fatalf("expected base file first, got %s", filepath.Base(parts[0]))
	}
}

func TestDetectParts_pkgpart(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 3; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("Detroit_%03d.pkgpart", i)), make([]byte, 100), 0644)
	}

	parts, name := DetectParts(filepath.Join(dir, "Detroit_002.pkgpart"))
	if len(parts) != 3 {
		t.Fatalf("expected 3, got %d", len(parts))
	}
	if name != "Detroit.pkg" {
		t.Fatalf("expected Detroit.pkg, got %s", name)
	}
}

func TestDetectParts_pkgDotNNN(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Game.pkg"), make([]byte, 100), 0644)
	os.WriteFile(filepath.Join(dir, "Game.pkg.001"), make([]byte, 200), 0644)
	os.WriteFile(filepath.Join(dir, "Game.pkg.002"), make([]byte, 200), 0644)

	parts, _ := DetectParts(filepath.Join(dir, "Game.pkg.001"))
	if len(parts) != 3 {
		t.Fatalf("expected 3, got %d", len(parts))
	}
}

func TestSuggestOutputPath_collision(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "Game.pkg")
	os.WriteFile(base, make([]byte, 10), 0644)

	parts := []string{base, filepath.Join(dir, "Game.pkg_0")}
	suggested := SuggestOutputPath(parts, "Game.pkg")
	if filepath.Base(suggested) != "Game_merged.pkg" {
		t.Fatalf("expected Game_merged.pkg, got %s", filepath.Base(suggested))
	}
}
