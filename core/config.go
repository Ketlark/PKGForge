package core

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user preferences.
type Config struct {
	DefaultBufferLabel string `json:"defaultBufferLabel"`
	DefaultChunkLabel  string `json:"defaultChunkLabel"`
	DefaultSplitFormat string `json:"defaultSplitFormat"`
	DefaultOutputDir   string `json:"defaultOutputDir"`
	Language           string `json:"language"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		DefaultBufferLabel: "64 MB",
		DefaultChunkLabel:  "4 GB",
		DefaultSplitFormat: "_NNN.pkgpart",
		DefaultOutputDir:   "",
		Language:           "en",
	}
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(dir, "pkg-forge")
	return d, os.MkdirAll(d, 0755)
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig reads preferences from disk, returning defaults on any error.
func LoadConfig() Config {
	path, err := configPath()
	if err != nil {
		return DefaultConfig()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig()
	}

	defaults := DefaultConfig()
	cfg := defaults

	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaults
	}

	if cfg.Language == "" {
		cfg.Language = defaults.Language
	}
	if cfg.DefaultBufferLabel == "" {
		cfg.DefaultBufferLabel = defaults.DefaultBufferLabel
	}
	if cfg.DefaultChunkLabel == "" {
		cfg.DefaultChunkLabel = defaults.DefaultChunkLabel
	}
	if cfg.DefaultSplitFormat == "" {
		cfg.DefaultSplitFormat = defaults.DefaultSplitFormat
	}
	return cfg
}

// SaveConfig writes preferences to disk.
func SaveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
