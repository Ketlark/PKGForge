package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Language != "en" {
		t.Errorf("language = %q, want en", cfg.Language)
	}
	if cfg.DefaultBufferLabel != "64 MB" {
		t.Errorf("buffer = %q", cfg.DefaultBufferLabel)
	}
}

func TestConfig_saveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := Config{
		DefaultBufferLabel: "128 MB",
		DefaultChunkLabel:  "8 GB",
		DefaultSplitFormat: ".pkg.NNN",
		Language:           "fr",
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, data, 0644)

	loaded := Config{}
	raw, _ := os.ReadFile(path)
	json.Unmarshal(raw, &loaded)

	if loaded.Language != "fr" {
		t.Errorf("language = %q", loaded.Language)
	}
	if loaded.DefaultBufferLabel != "128 MB" {
		t.Errorf("buffer = %q", loaded.DefaultBufferLabel)
	}
	if loaded.DefaultChunkLabel != "8 GB" {
		t.Errorf("chunk = %q", loaded.DefaultChunkLabel)
	}
}
