package fpkg

import (
	"bytes"
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

func TestResolvePS1BackgroundUsesLocalBackgroundAndCachesPNG(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	cuePath := filepath.Join(dir, "Cool Game.cue")
	backgroundPath := filepath.Join(dir, "Cool Game_background.png")

	if err := os.WriteFile(cuePath, []byte(`FILE "Cool Game.bin" BINARY`), 0644); err != nil {
		t.Fatalf("write cue: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 8, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(25 * x), G: uint8(70 * y), B: 160, A: 255})
		}
	}
	f, err := os.Create(backgroundPath)
	if err != nil {
		t.Fatalf("create background: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode background: %v", err)
	}
	f.Close()

	got, err := ResolvePS1Background(cuePath, "SLUS-00875")
	if err != nil {
		t.Fatalf("resolve background: %v", err)
	}
	cached, err := os.Open(got)
	if err != nil {
		t.Fatalf("open cached background: %v", err)
	}
	defer cached.Close()

	cfg, format, err := image.DecodeConfig(cached)
	if err != nil {
		t.Fatalf("decode cached background: %v", err)
	}
	if format != "png" {
		t.Fatalf("cached background format = %q, want png", format)
	}
	if cfg.Width != 1920 || cfg.Height != 1080 {
		t.Fatalf("cached background size = %dx%d, want 1920x1080", cfg.Width, cfg.Height)
	}
}

func TestResolvePS2CoverUsesLocalCoverAndCachesPNG(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	isoPath := filepath.Join(dir, "Gran Turismo 4.iso")
	coverPath := filepath.Join(dir, "Gran Turismo 4_cover.png")

	if err := os.WriteFile(isoPath, []byte("iso"), 0644); err != nil {
		t.Fatalf("write iso: %v", err)
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

	got, err := ResolvePS2Cover(isoPath, "SCES-51719")
	if err != nil {
		t.Fatalf("resolve cover: %v", err)
	}
	if filepath.Ext(got) != ".png" {
		t.Fatalf("cached cover extension = %q, want .png", filepath.Ext(got))
	}
}

func TestResolvePS2BackgroundUsesLocalBackgroundAndCachesPNG(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	isoPath := filepath.Join(dir, "Gran Turismo 4.iso")
	backgroundPath := filepath.Join(dir, "Gran Turismo 4_background.png")

	if err := os.WriteFile(isoPath, []byte("iso"), 0644); err != nil {
		t.Fatalf("write iso: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 8, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(25 * x), G: uint8(70 * y), B: 160, A: 255})
		}
	}
	f, err := os.Create(backgroundPath)
	if err != nil {
		t.Fatalf("create background: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode background: %v", err)
	}
	f.Close()

	got, err := ResolvePS2Background(isoPath, "SCES-51719")
	if err != nil {
		t.Fatalf("resolve background: %v", err)
	}
	cached, err := os.Open(got)
	if err != nil {
		t.Fatalf("open cached background: %v", err)
	}
	defer cached.Close()

	cfg, format, err := image.DecodeConfig(cached)
	if err != nil {
		t.Fatalf("decode cached background: %v", err)
	}
	if format != "png" {
		t.Fatalf("cached background format = %q, want png", format)
	}
	if cfg.Width != 1920 || cfg.Height != 1080 {
		t.Fatalf("cached background size = %dx%d, want 1920x1080", cfg.Width, cfg.Height)
	}
}

func TestPS1BackgroundFromImagePathUsesLaunchDimensions(t *testing.T) {
	dir := t.TempDir()
	coverPath := filepath.Join(dir, "cover.png")

	img := image.NewRGBA(image.Rect(0, 0, 4, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 120, G: uint8(30 * y), B: uint8(50 * x), A: 255})
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

	data, err := ps1BackgroundFromImagePath(coverPath, "SLUS-00875")
	if err != nil {
		t.Fatalf("compose background: %v", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode composed background: %v", err)
	}
	if format != "png" {
		t.Fatalf("composed background format = %q, want png", format)
	}
	if cfg.Width != 1920 || cfg.Height != 1080 {
		t.Fatalf("composed background size = %dx%d, want 1920x1080", cfg.Width, cfg.Height)
	}
}
