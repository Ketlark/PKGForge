//go:build !darwin || !sparkle

package core

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var updateHTTPClient = &http.Client{Timeout: 30 * time.Second}

// UpdateBackend reports the built-in GitHub updater on Windows and Linux.
func UpdateBackend() string { return "builtin" }

type githubRelease struct {
	TagName string        `json:"tag_name"`
	HTMLURL string        `json:"html_url"`
	Body    string        `json:"body"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// ConfigureUpdateOnStartup is a no-op on Windows and Linux; the frontend triggers checks.
func ConfigureUpdateOnStartup(_ bool) {}

func setGitHubHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// CheckForUpdate queries GitHub for the latest release and compares semver versions.
func CheckForUpdate(ctx context.Context, currentVersion string) (*UpdateInfo, error) {
	if currentVersion == "" || strings.HasPrefix(currentVersion, "dev") {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", githubBaseURL+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	setGitHubHeaders(req)

	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	current := normalizeVersion(currentVersion)
	latest := normalizeVersion(release.TagName)
	if !isNewerVersion(current, latest) {
		return nil, nil
	}

	asset := findPlatformAsset(release.Assets)
	if asset == nil {
		return nil, nil
	}

	return &UpdateInfo{
		Version:      latest,
		ReleaseURL:   release.HTMLURL,
		ReleaseNotes: release.Body,
		AssetName:    asset.Name,
		AssetURL:     asset.BrowserDownloadURL,
		AssetSize:    asset.Size,
	}, nil
}

// DownloadAndApplyUpdate downloads, verifies SHA256, and replaces the executable.
func DownloadAndApplyUpdate(ctx context.Context, info *UpdateInfo, progress func(float64)) error {
	tmpDir, err := os.MkdirTemp("", "pkg-forge-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	assetPath := filepath.Join(tmpDir, info.AssetName)
	if err := downloadFile(ctx, info.AssetURL, assetPath, info.AssetSize, progress); err != nil {
		return fmt.Errorf("download asset: %w", err)
	}

	if hash, ok := fetchSHA256(ctx, info.AssetName); ok {
		if err := verifySHA256(assetPath, hash); err != nil {
			return fmt.Errorf("sha256 verification failed: %w", err)
		}
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)

	if strings.HasSuffix(info.AssetName, ".zip") {
		return applyZipUpdate(assetPath, exePath)
	}
	if strings.HasSuffix(info.AssetName, ".tar.gz") || strings.HasSuffix(info.AssetName, ".tgz") {
		return applyTarGzUpdate(assetPath, exePath)
	}
	return applyBinaryUpdate(assetPath, exePath)
}

func platformAssetPattern() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func findPlatformAsset(assets []githubAsset) *githubAsset {
	pattern := platformAssetPattern()
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.Contains(name, pattern) && !strings.HasSuffix(name, ".sha256") && !strings.Contains(name, "sha256sums") {
			return &assets[i]
		}
	}
	return nil
}

func downloadFile(ctx context.Context, url, dest string, expectedSize int64, progress func(float64)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	writer := io.Writer(out)
	if progress != nil && expectedSize > 0 {
		writer = &progressWriter{w: out, total: expectedSize, fn: progress}
	}
	_, err = io.Copy(writer, resp.Body)
	return err
}

type progressWriter struct {
	w     io.Writer
	total int64
	read  int64
	fn    func(float64)
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	pw.read += int64(n)
	if pw.total > 0 {
		pw.fn(float64(pw.read) / float64(pw.total))
	}
	return n, err
}

func fetchSHA256(ctx context.Context, assetName string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubBaseURL+"/releases/latest", nil)
	if err != nil {
		return "", false
	}
	setGitHubHeaders(req)
	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", false
	}
	for _, a := range release.Assets {
		if strings.EqualFold(a.Name, "SHA256SUMS.txt") || strings.EqualFold(a.Name, "sha256sums.txt") {
			req2, err := http.NewRequestWithContext(ctx, "GET", a.BrowserDownloadURL, nil)
			if err != nil {
				return "", false
			}
			resp2, err := updateHTTPClient.Do(req2)
			if err != nil {
				return "", false
			}
			defer resp2.Body.Close()
			body, err := io.ReadAll(resp2.Body)
			if err != nil {
				return "", false
			}
			return findHashInChecksums(string(body), assetName), true
		}
	}
	return "", false
}

func verifySHA256(path, expectedHash string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != strings.ToLower(expectedHash) {
		return fmt.Errorf("expected %s, got %s", expectedHash, got)
	}
	return nil
}

func applyBinaryUpdate(newBinary, exePath string) error {
	oldPath := exePath + ".old"
	_ = os.Remove(oldPath)

	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("rename current binary: %w", err)
	}
	if err := copyFile(newBinary, exePath, 0755); err != nil {
		_ = os.Rename(oldPath, exePath)
		return fmt.Errorf("write new binary: %w", err)
	}
	_ = os.Remove(oldPath)
	return nil
}

func applyZipUpdate(zipPath, exePath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	var binFile *zip.File
	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name == filepath.Base(exePath) || (runtime.GOOS == "windows" && strings.EqualFold(name, filepath.Base(exePath))) {
			binFile = f
			break
		}
	}
	if binFile == nil {
		target := "pkg-forge"
		if runtime.GOOS == "windows" {
			target = "pkg-forge.exe"
		}
		for _, f := range r.File {
			if filepath.Base(f.Name) == target {
				binFile = f
				break
			}
		}
	}
	if binFile == nil {
		return fmt.Errorf("executable not found in zip")
	}

	rc, err := binFile.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	tmpPath := exePath + ".new"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	out.Close()

	oldPath := exePath + ".old"
	_ = os.Remove(oldPath)
	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("rename current binary: %w", err)
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		_ = os.Rename(oldPath, exePath)
		return fmt.Errorf("install new binary: %w", err)
	}
	_ = os.Chmod(exePath, 0755)
	_ = os.Remove(oldPath)
	return nil
}

func applyTarGzUpdate(tarPath, exePath string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tmpPath := exePath + ".new"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, gz); err != nil {
		out.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	out.Close()

	oldPath := exePath + ".old"
	_ = os.Remove(oldPath)
	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("rename current binary: %w", err)
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		_ = os.Rename(oldPath, exePath)
		return fmt.Errorf("install new binary: %w", err)
	}
	_ = os.Chmod(exePath, 0755)
	_ = os.Remove(oldPath)
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
