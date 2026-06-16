package core

import (
	"encoding/hex"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	githubOwner   = "Ketlark"
	githubRepo    = "PKGForge"
	githubBaseURL = "https://api.github.com/repos/" + githubOwner + "/" + githubRepo

	// SparkleAppcastURL is a stable feed URL; each GitHub release ships appcast.xml
	// as a release asset with this exact name (resolved via /releases/latest/download/).
	SparkleAppcastURL = "https://github.com/Ketlark/PKGForge/releases/latest/download/appcast.xml"
)

// UpdateInfo describes an available update (Windows / Linux built-in updater).
type UpdateInfo struct {
	Version      string `json:"version"`
	ReleaseURL   string `json:"releaseUrl"`
	ReleaseNotes string `json:"releaseNotes"`
	AssetName    string `json:"assetName"`
	AssetURL     string `json:"assetUrl"`
	AssetSize    int64  `json:"assetSize"`
}

func normalizeVersion(tag string) string {
	v := strings.TrimSpace(tag)
	v = strings.TrimPrefix(v, "v")
	return v
}

func isNewerVersion(current, candidate string) bool {
	c := semver.Canonical("v" + normalizeVersion(current))
	n := semver.Canonical("v" + normalizeVersion(candidate))
	if c == "" || n == "" {
		return false
	}
	return semver.Compare(n, c) > 0
}

func findHashInChecksums(content, filename string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.Fields(line)
		if len(parts) >= 2 && (parts[1] == filename || strings.HasSuffix(parts[1], filename)) {
			h := strings.TrimSpace(parts[0])
			if _, err := hex.DecodeString(h); err == nil {
				return h
			}
		}
	}
	return ""
}
