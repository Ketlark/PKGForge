package fpkg

import (
	"bytes"
	"crypto/sha256"
	"image"
	"image/color"
	"image/png"
)

func defaultIcon0PNG(titleID string) []byte {
	accent := accentColor(titleID)
	img := image.NewRGBA(image.Rect(0, 0, 512, 512))
	for y := 0; y < 512; y++ {
		for x := 0; x < 512; x++ {
			shade := uint8(24 + (x+y)/48)
			img.SetRGBA(x, y, color.RGBA{R: shade, G: shade + 4, B: shade + 10, A: 255})
		}
	}

	for y := 48; y < 464; y++ {
		for x := 48; x < 464; x++ {
			if x < 64 || x >= 448 || y < 64 || y >= 448 {
				img.SetRGBA(x, y, accent)
			}
		}
	}
	for y := 198; y < 314; y++ {
		for x := 126; x < 386; x++ {
			if (x+y)%17 < 9 {
				img.SetRGBA(x, y, color.RGBA{R: 236, G: 238, B: 242, A: 255})
			}
		}
	}
	return encodePNG(img)
}

func defaultPic1PNG(titleID string) []byte {
	accent := accentColor(titleID)
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y++ {
		for x := 0; x < 1920; x++ {
			base := uint8(10 + y/90)
			r := base
			g := uint8(12 + x/220)
			b := uint8(20 + (x+y)/260)
			if x >= 96 && x < 120 {
				r, g, b = accent.R, accent.G, accent.B
			}
			if y >= 860 && y < 900 && x >= 96 && x < 1824 {
				r, g, b = mixByte(r, accent.R), mixByte(g, accent.G), mixByte(b, accent.B)
			}
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return encodePNG(img)
}

func defaultSaveDataPNG(titleID string) []byte {
	accent := accentColor(titleID)
	img := image.NewRGBA(image.Rect(0, 0, 228, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 228; x++ {
			base := uint8(18 + y/8)
			r := base
			g := uint8(20 + x/32)
			b := uint8(28 + (x+y)/44)
			if x >= 12 && x < 20 {
				r, g, b = accent.R, accent.G, accent.B
			}
			if y >= 96 && y < 104 && x >= 20 && x < 216 {
				r, g, b = mixByte(r, accent.R), mixByte(g, accent.G), mixByte(b, accent.B)
			}
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return encodePNG(img)
}

func accentColor(titleID string) color.RGBA {
	sum := sha256.Sum256([]byte(titleID))
	return color.RGBA{
		R: 72 + sum[0]%128,
		G: 88 + sum[1]%112,
		B: 104 + sum[2]%96,
		A: 255,
	}
}

func mixByte(a, b uint8) uint8 {
	return uint8((uint16(a) + uint16(b)) / 2)
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
