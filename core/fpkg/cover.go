package fpkg

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var psxCoverHTTPClient = &http.Client{Timeout: 8 * time.Second}

// ResolvePS1Cover finds or downloads a PS1 cover and returns a cached 512x512 PNG.
// It is best-effort: callers should keep building with default artwork when it
// returns an error.
func ResolvePS1Cover(cuePath, titleID string) (string, error) {
	serial := formatPSXSerial(titleID)
	if serial == "" {
		return "", fmt.Errorf("ps1 cover: missing title id")
	}

	cachePath, err := ps1CoverCachePath(serial)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	for _, local := range localPS1CoverCandidates(cuePath) {
		if _, err := os.Stat(local); err == nil {
			if err := convertCoverToPNGFile(local, cachePath); err != nil {
				return "", err
			}
			return cachePath, nil
		}
	}

	var lastErr error
	for _, url := range ps1CoverURLCandidates(serial, titleFromDiscPath(cuePath)) {
		data, err := downloadCover(url)
		if err != nil {
			lastErr = err
			continue
		}
		if err := convertCoverBytesToPNGFile(data, cachePath); err != nil {
			lastErr = err
			continue
		}
		return cachePath, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("ps1 cover: no cover found for %s", serial)
}

func localPS1CoverCandidates(cuePath string) []string {
	if cuePath == "" {
		return nil
	}
	base := strings.TrimSuffix(cuePath, filepath.Ext(cuePath))
	return []string{
		base + "_cover.png",
		base + "_cover.jpg",
		base + "_cover.jpeg",
		base + "-cover.png",
		base + "-cover.jpg",
		base + "-cover.jpeg",
	}
}

func ps1CoverCachePath(serial string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("ps1 cover cache: %w", err)
	}
	dir = filepath.Join(dir, "pkg-forge", "covers")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("ps1 cover cache mkdir: %w", err)
	}
	return filepath.Join(dir, serial+".png"), nil
}

func ps1CoverURLCandidates(serial, title string) []string {
	letters := coverSearchLetters(title)
	urls := []string{
		"https://raw.githubusercontent.com/xlenore/psx-covers/main/covers/default/" + serial + ".jpg",
		"https://raw.githubusercontent.com/xlenore/psx-covers/main/covers/3d/" + serial + ".png",
	}

	for _, region := range psxDataCenterRegions(serial) {
		for _, letter := range letters {
			for _, ext := range []string{".jpg", ".png"} {
				urls = append(urls, "https://psxdatacenter.com/images/covers/"+region+"/"+letter+"/"+serial+ext)
			}
		}
	}
	return urls
}

func coverSearchLetters(title string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(letter string) {
		if letter == "" || seen[letter] {
			return
		}
		seen[letter] = true
		out = append(out, letter)
	}

	cleanTitle := strings.ToUpper(strings.TrimSpace(title))
	for _, prefix := range []string{"THE ", "A ", "AN ", "LE ", "LA ", "LES ", "L'"} {
		cleanTitle = strings.TrimPrefix(cleanTitle, prefix)
	}
	for _, r := range cleanTitle {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			add(string(r))
			break
		}
	}
	for ch := 'A'; ch <= 'Z'; ch++ {
		add(string(ch))
	}
	for ch := '0'; ch <= '9'; ch++ {
		add(string(ch))
	}
	return out
}

func psxDataCenterRegions(serial string) []string {
	switch regionFromID(serial) {
	case "america":
		return []string{"U"}
	case "europe":
		return []string{"P", "E"}
	case "japan":
		return []string{"J"}
	default:
		return []string{"U", "P", "E", "J"}
	}
}

func formatPSXSerial(titleID string) string {
	normalized := normalizeGameID(titleID)
	if normalized == "" {
		return ""
	}
	return normalized
}

func downloadCover(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "PKG-Forge/1.0")

	resp, err := psxCoverHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover GET %s: %s", url, resp.Status)
	}
	if ct := strings.ToLower(resp.Header.Get("Content-Type")); ct != "" && !strings.Contains(ct, "image/") && !strings.Contains(ct, "octet-stream") {
		return nil, fmt.Errorf("cover GET %s: unexpected content type %s", url, ct)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func convertCoverToPNGFile(srcPath, destPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return convertCoverBytesToPNGFile(data, destPath)
}

func convertCoverBytesToPNGFile(data []byte, destPath string) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode cover: %w", err)
	}
	icon := squareResizeNearest(img, 512)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, icon)
}

func squareResizeNearest(src image.Image, size int) *image.RGBA {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	crop := bounds
	if w > h {
		x0 := bounds.Min.X + (w-h)/2
		crop = image.Rect(x0, bounds.Min.Y, x0+h, bounds.Min.Y+h)
	} else if h > w {
		y0 := bounds.Min.Y + (h-w)/2
		crop = image.Rect(bounds.Min.X, y0, bounds.Min.X+w, y0+w)
	}

	cropped := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	draw.Draw(cropped, cropped.Bounds(), src, crop.Min, draw.Src)

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		sy := y * cropped.Bounds().Dy() / size
		for x := 0; x < size; x++ {
			sx := x * cropped.Bounds().Dx() / size
			dst.Set(x, y, cropped.At(sx, sy))
		}
	}
	return dst
}

var (
	_ = jpeg.DefaultQuality
	_ = gif.GIF{}
)
