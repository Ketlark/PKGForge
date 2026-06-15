package fpkg

import (
	"bytes"
	"image"
	"testing"
)

func TestDefaultMediaPNGsHaveExpectedDimensions(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		w    int
		h    int
	}{
		{name: "icon0", data: defaultIcon0PNG("SCES02752"), w: 512, h: 512},
		{name: "pic1", data: defaultPic1PNG("SCES02752"), w: 1920, h: 1080},
		{name: "save_data", data: defaultSaveDataPNG("SCES02752"), w: 228, h: 128},
	}

	for _, tc := range tests {
		cfg, format, err := image.DecodeConfig(bytes.NewReader(tc.data))
		if err != nil {
			t.Fatalf("%s decode config: %v", tc.name, err)
		}
		if format != "png" {
			t.Fatalf("%s format = %q, want png", tc.name, format)
		}
		if cfg.Width != tc.w || cfg.Height != tc.h {
			t.Fatalf("%s dimensions = %dx%d, want %dx%d", tc.name, cfg.Width, cfg.Height, tc.w, tc.h)
		}
	}
}
