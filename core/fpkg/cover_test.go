package fpkg

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePS1CoverUsesLocalCoverAndCachesPNG(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	cuePath := filepath.Join(dir, "Cool Game.cue")
	coverPath := filepath.Join(dir, "Cool Game_cover.png")

	if err := os.WriteFile(cuePath, []byte(`FILE "Cool Game.bin" BINARY`), 0644); err != nil {
		t.Fatalf("write cue: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(40 * x), G: uint8(80 * y), B: 120, A: 255})
		}
	}
	f, err := os.Create(coverPath)
	if err != nil {
		t.Fatalf("create cover: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode cover: %v", err)
	}
	f.Close()

	got, err := ResolvePS1Cover(cuePath, "SLUS-00875")
	if err != nil {
		t.Fatalf("resolve cover: %v", err)
	}
	if filepath.Ext(got) != ".png" {
		t.Fatalf("cached cover extension = %q, want .png", filepath.Ext(got))
	}

	cached, err := os.Open(got)
	if err != nil {
		t.Fatalf("open cached cover: %v", err)
	}
	defer cached.Close()

	decoded, format, err := image.Decode(cached)
	if err != nil {
		t.Fatalf("decode cached cover: %v", err)
	}
	if format != "png" {
		t.Fatalf("cached cover format = %q, want png", format)
	}
	if decoded.Bounds().Dx() != 512 || decoded.Bounds().Dy() != 512 {
		t.Fatalf("cached cover size = %dx%d, want 512x512", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}
