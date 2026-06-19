package fpkg

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
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

	for _, local := range localDiscCoverCandidates(cuePath) {
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

// ResolvePS1Background finds a CUE-adjacent launch background and returns a
// cached 1920x1080 PNG. Remote background art is intentionally not required for
// package creation; callers should keep building with bundled/default artwork.
func ResolvePS1Background(cuePath, titleID string) (string, error) {
	serial := formatPSXSerial(titleID)
	if serial == "" {
		return "", fmt.Errorf("ps1 background: missing title id")
	}

	cachePath, err := ps1BackgroundCachePath(serial)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	var lastErr error
	for _, local := range localDiscBackgroundCandidates(cuePath) {
		if _, err := os.Stat(local); err != nil {
			continue
		}
		if err := convertBackgroundToPNGFile(local, cachePath); err != nil {
			lastErr = err
			continue
		}
		return cachePath, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("ps1 background: no local background found for %s", serial)
}

// ResolvePS2Cover finds or downloads a PS2 cover and returns a cached 512x512 PNG.
// It is best-effort: callers should keep building with default artwork when it
// returns an error.
func ResolvePS2Cover(discPath, titleID string) (string, error) {
	serial := formatGameSerial(titleID)
	if serial == "" {
		return "", fmt.Errorf("ps2 cover: missing title id")
	}

	cachePath, err := ps1CoverCachePath(serial)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	for _, local := range localDiscCoverCandidates(discPath) {
		if _, err := os.Stat(local); err == nil {
			if err := convertCoverToPNGFile(local, cachePath); err != nil {
				return "", err
			}
			return cachePath, nil
		}
	}

	var lastErr error
	for _, url := range ps2CoverURLCandidates(serial) {
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
	return "", fmt.Errorf("ps2 cover: no cover found for %s", serial)
}

// ResolvePS2Background finds a disc-adjacent launch background and returns a
// cached 1920x1080 PNG. Remote background art is not required for package
// creation; callers should keep building with bundled/default artwork.
func ResolvePS2Background(discPath, titleID string) (string, error) {
	serial := formatGameSerial(titleID)
	if serial == "" {
		return "", fmt.Errorf("ps2 background: missing title id")
	}

	cachePath, err := ps1BackgroundCachePath(serial)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	var lastErr error
	for _, local := range localDiscBackgroundCandidates(discPath) {
		if _, err := os.Stat(local); err != nil {
			continue
		}
		if err := convertBackgroundToPNGFile(local, cachePath); err != nil {
			lastErr = err
			continue
		}
		return cachePath, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("ps2 background: no local background found for %s", serial)
}

func localDiscCoverCandidates(discPath string) []string {
	if discPath == "" {
		return nil
	}
	base := strings.TrimSuffix(discPath, filepath.Ext(discPath))
	return []string{
		base + "_cover.png",
		base + "_cover.jpg",
		base + "_cover.jpeg",
		base + "-cover.png",
		base + "-cover.jpg",
		base + "-cover.jpeg",
	}
}

func localDiscBackgroundCandidates(discPath string) []string {
	if discPath == "" {
		return nil
	}
	dir := filepath.Dir(discPath)
	base := strings.TrimSuffix(discPath, filepath.Ext(discPath))
	return []string{
		base + "_background.png",
		base + "_background.jpg",
		base + "_background.jpeg",
		base + "-background.png",
		base + "-background.jpg",
		base + "-background.jpeg",
		base + "_pic1.png",
		base + "_pic1.jpg",
		base + "_pic1.jpeg",
		base + "-pic1.png",
		base + "-pic1.jpg",
		base + "-pic1.jpeg",
		filepath.Join(dir, "pic1.png"),
		filepath.Join(dir, "background.png"),
		filepath.Join(dir, "background.jpg"),
		filepath.Join(dir, "background.jpeg"),
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

func ps1BackgroundCachePath(serial string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("ps1 background cache: %w", err)
	}
	dir = filepath.Join(dir, "pkg-forge", "backgrounds")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("ps1 background cache mkdir: %w", err)
	}
	return filepath.Join(dir, serial+".png"), nil
}

func ps2CoverURLCandidates(serial string) []string {
	return []string{
		"https://raw.githubusercontent.com/xlenore/ps2-covers/main/covers/default/" + serial + ".jpg",
		"https://raw.githubusercontent.com/xlenore/ps2-covers/main/covers/3d/" + serial + ".png",
	}
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
	return formatGameSerial(titleID)
}

func formatGameSerial(titleID string) string {
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

func convertBackgroundToPNGFile(srcPath, destPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return convertBackgroundBytesToPNGFile(data, destPath)
}

func convertBackgroundBytesToPNGFile(data []byte, destPath string) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode background: %w", err)
	}
	background := resizeFillNearest(img, 1920, 1080)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, background)
}

func ps1BackgroundFromImagePath(imagePath, titleID string) ([]byte, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, err
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode background source: %w", err)
	}

	canvas := resizeFillNearest(src, 1920, 1080)
	for y := 0; y < 1080; y++ {
		for x := 0; x < 1920; x++ {
			r, g, b, a := canvas.At(x, y).RGBA()
			canvas.SetRGBA(x, y, color.RGBA{
				R: uint8((r >> 8) / 3),
				G: uint8((g >> 8) / 3),
				B: uint8((b >> 8) / 3),
				A: uint8(a >> 8),
			})
		}
	}

	accent := accentColor(titleID)
	for y := 880; y < 920; y++ {
		for x := 120; x < 1800; x++ {
			canvas.SetRGBA(x, y, color.RGBA{
				R: uint8((uint16(accent.R) + 24) / 2),
				G: uint8((uint16(accent.G) + 24) / 2),
				B: uint8((uint16(accent.B) + 32) / 2),
				A: 255,
			})
		}
	}

	cover := resizeFitNearest(src, 640, 820)
	x := 160
	y := (1080 - cover.Bounds().Dy()) / 2
	draw.Draw(canvas, image.Rect(x-8, y-8, x+cover.Bounds().Dx()+8, y+cover.Bounds().Dy()+8), image.NewUniform(color.RGBA{R: 238, G: 238, B: 242, A: 255}), image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(x, y, x+cover.Bounds().Dx(), y+cover.Bounds().Dy()), cover, image.Point{}, draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

func resizeFillNearest(src image.Image, width, height int) *image.RGBA {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, width, height))
	}
	crop := bounds
	if w*height > h*width {
		cropW := h * width / height
		x0 := bounds.Min.X + (w-cropW)/2
		crop = image.Rect(x0, bounds.Min.Y, x0+cropW, bounds.Min.Y+h)
	} else if w*height < h*width {
		cropH := w * height / width
		y0 := bounds.Min.Y + (h-cropH)/2
		crop = image.Rect(bounds.Min.X, y0, bounds.Min.X+w, y0+cropH)
	}
	return resizeCropNearest(src, crop, width, height)
}

func resizeFitNearest(src image.Image, maxWidth, maxHeight int) *image.RGBA {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, maxWidth, maxHeight))
	}
	dstW, dstH := maxWidth, h*maxWidth/w
	if dstH > maxHeight {
		dstH = maxHeight
		dstW = w * maxHeight / h
	}
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}
	return resizeCropNearest(src, bounds, dstW, dstH)
}

func resizeCropNearest(src image.Image, crop image.Rectangle, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		sy := crop.Min.Y + y*crop.Dy()/height
		for x := 0; x < width; x++ {
			sx := crop.Min.X + x*crop.Dx()/width
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

var (
	_ = jpeg.DefaultQuality
	_ = gif.GIF{}
)
