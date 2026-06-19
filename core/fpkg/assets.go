package fpkg

// This file handles emulator assets bundled as an encrypted archive in the binary.
//
// At build time, tools/encrypt_assets.go creates core/fpkg/assets.dat —
// a tar.gz encrypted with AES-256-GCM containing the essential emulator files.
// The encryption key is compiled into the binary.
//
// At runtime, EnsureAssets() decrypts and extracts to the user's
// config directory on first use. Subsequent runs reuse the cache.
//
// Cache layout after extraction:
//
//	<cache>/emus/ps1hd/eboot.bin, sce_module/libc.prx, ...
//	<cache>/emus/Jak v2/eboot.bin, sce_module/libc.prx, ps2-emu-compiler.self, ...
//	<cache>/emus/Rogue v1/...
//	<cache>/lua_include/*.lua

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed assets.dat
var encryptedAssets []byte

// Encryption key — must match tools/encrypt_assets.go
var assetsKey = []byte("01234567890123456789012345678901")

var assetsMu sync.Mutex

const assetsManifestName = ".pkg-forge-assets-manifest"

var requiredBundledAssetPaths = []string{
	"emus/ps1hd/eboot.bin",
	"emus/ps1hd/sce_module/libc.prx",
	"emus/ps1hd/sce_module/libSceFios2.prx",
	"emus/ps1hd/sce_module/libSceNpToolkit2.prx",
	"emus/ps1hd/assets/PS1HD/bios/SCPH5500.bin",
	"emus/ps1hd/assets/PS1HD/bios/SCPH5501.bin",
	"emus/ps1hd/assets/PS1HD/bios/SCPH5502.bin",
	"emus/ps1hd/assets/common/consola.ttf",
	"emus/ps1hd/package-ps4.conf",
	"emus/ps1hd/sce_discmap.plt",
	"emus/ps1hd/sce_sys/about/right.sprx",
	"emus/ps1hd/sce_sys/pic1.png",
	"emus/Jak v2/eboot.bin",
	"emus/Jak v2/sce_module/libc.prx",
	"emus/Jak v2/sce_module/libSceFios2.prx",
	"emus/Jak v2/ps2-emu-compiler.self",
	"emus/Jak v2/PS20220WD20050620.crack",
	"emus/Jak v2/sce_discmap.plt",
	"emus/Jak v2/config-emu-ps4.txt",
	"emus/Jak v2/formatted.card",
	"emus/Rogue v1/eboot.bin",
	"emus/Rogue v1/sce_module/libc.prx",
	"emus/Rogue v1/sce_module/libSceFios2.prx",
	"emus/Rogue v1/ps2-emu-compiler.self",
	"emus/Rogue v1/PS20220WD20050620.crack",
	"emus/Rogue v1/sce_discmap.plt",
	"emus/Rogue v1/config-emu-ps4.txt",
	"emus/Rogue v1/formatted.card",
	"emus/Siren v2/eboot.bin",
	"emus/Siren v2/sce_module/libc.prx",
	"emus/Siren v2/sce_module/libSceFios2.prx",
	"emus/Siren v2/ps2-emu-compiler.self",
	"emus/Siren v2/PS20220WD20050620.crack",
	"emus/Siren v2/sce_discmap.plt",
	"emus/Siren v2/config-emu-ps4.txt",
	"emus/Siren v2/formatted.card",
}

// AssetsCacheDir returns the local cache directory for emulator assets.
func AssetsCacheDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	dir := filepath.Join(configDir, "pkg-forge", "assets")
	return dir, os.MkdirAll(dir, 0755)
}

// HasCachedAssets returns true if emulator assets have been extracted.
func HasCachedAssets() bool {
	dir, err := AssetsCacheDir()
	if err != nil {
		return false
	}
	return assetCacheMatchesBundle(dir)
}

// EnsureAssets extracts bundled assets to cache if not already present.
// It is safe to call concurrently — only one extraction runs at a time.
// Progress callback receives 0.0 → 1.0.
func EnsureAssets(progress func(float64)) error {
	assetsMu.Lock()
	defer assetsMu.Unlock()

	if HasCachedAssets() {
		if progress != nil {
			progress(1.0)
		}
		return nil
	}

	dir, err := AssetsCacheDir()
	if err != nil {
		return fmt.Errorf("cache dir: %w", err)
	}

	if err := clearExtractedAssets(dir); err != nil {
		return fmt.Errorf("clear stale assets: %w", err)
	}
	return extractBundledAssets(dir, progress)
}

// ResolveEmulatorsDir returns the directory containing emulator subdirectories.
// If manualDir is non-empty, it is returned directly (user override).
// Otherwise, ensures assets are extracted and returns <cache>/emus/.
func ResolveEmulatorsDir(manualDir string, progress func(float64)) (string, error) {
	if manualDir != "" {
		return manualDir, nil
	}

	if err := EnsureAssets(progress); err != nil {
		return "", fmt.Errorf("ensure assets: %w", err)
	}

	cacheDir, err := AssetsCacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(cacheDir, "emus"), nil
}

// ---------------------------------------------------------------------------
// Bundled asset extraction (decrypt + untar)
// ---------------------------------------------------------------------------

func extractBundledAssets(destDir string, progress func(float64)) error {
	data, err := decryptBundledAssets()
	if err != nil {
		return err
	}

	// Decompress gzip
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	// Extract tar
	tr := tar.NewReader(gz)
	total := len(data)
	done := 0
	var manifest []string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// Prefix with "emus/" if it's an emulator dir, otherwise keep as-is
		targetRel := assetCacheRel(hdr.Name)

		target := filepath.Join(destDir, targetRel)

		// Path traversal guard
		if strings.Contains(targetRel, "..") {
			continue
		}

		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		f, err := os.Create(target)
		if err != nil {
			return err
		}
		written, err := io.Copy(f, tr)
		f.Close()
		if err != nil {
			return fmt.Errorf("extract %s: %w", targetRel, err)
		}
		manifest = append(manifest, filepath.ToSlash(targetRel))

		done += int(written)
		if progress != nil && total > 0 {
			progress(float64(done) / float64(total))
		}
	}

	if err := validateRequiredBundledAssets(manifest); err != nil {
		return err
	}
	if err := writeAssetsManifest(destDir, manifest); err != nil {
		return err
	}

	if progress != nil {
		progress(1.0)
	}
	return nil
}

func decryptBundledAssets() ([]byte, error) {
	data, err := decrypt(encryptedAssets, assetsKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt assets: %w", err)
	}
	return data, nil
}

func bundledAssetCachePaths() ([]string, error) {
	data, err := decryptBundledAssets()
	if err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	var paths []string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read: %w", err)
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		rel := assetCacheRel(hdr.Name)
		if strings.Contains(rel, "..") {
			continue
		}
		paths = append(paths, filepath.ToSlash(rel))
	}
	return paths, validateRequiredBundledAssets(paths)
}

func assetCacheRel(archiveRel string) string {
	rel := filepath.ToSlash(archiveRel)
	rel = strings.TrimPrefix(rel, "./")
	emu := rel
	if idx := strings.Index(rel, "/"); idx >= 0 {
		emu = rel[:idx]
	}
	switch emu {
	case "ps1hd", "Jak v2", "Rogue v1":
		return filepath.ToSlash(filepath.Join("emus", rel))
	default:
		return rel
	}
}

func validateRequiredBundledAssets(paths []string) error {
	present := make(map[string]bool, len(paths))
	for _, path := range paths {
		present[filepath.ToSlash(path)] = true
	}

	var missing []string
	for _, required := range requiredBundledAssetPaths {
		if !present[required] {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("bundled assets missing required runtime files: %s", strings.Join(missing, ", "))
	}
	return nil
}

func assetCacheMatchesBundle(dir string) bool {
	expected, err := bundledAssetCachePaths()
	if err != nil {
		return false
	}

	actual, err := os.ReadFile(filepath.Join(dir, assetsManifestName))
	if err != nil {
		return false
	}
	if string(actual) != strings.Join(expected, "\n")+"\n" {
		return false
	}

	for _, rel := range expected {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func writeAssetsManifest(destDir string, paths []string) error {
	var b strings.Builder
	for _, path := range paths {
		b.WriteString(filepath.ToSlash(path))
		b.WriteByte('\n')
	}
	return os.WriteFile(filepath.Join(destDir, assetsManifestName), []byte(b.String()), 0644)
}

func clearExtractedAssets(dir string) error {
	for _, rel := range []string{"emus", "lua_include", assetsManifestName} {
		if err := os.RemoveAll(filepath.Join(dir, rel)); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// AES-256-GCM decryption
// ---------------------------------------------------------------------------

func decrypt(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// Suppress unused import warning
var _ embed.FS

// ---------------------------------------------------------------------------
// Emulator name mapping (internal names → archive folder names)
// ---------------------------------------------------------------------------

// EmulatorArchiveName maps our internal emulator identifiers to the folder
// names in the bundled assets archive.
var EmulatorArchiveName = map[string]string{
	"ps1_emu":    "ps1hd",
	"ps1_netemu": "ps1hd",
	"jakv2":      "Jak v2",
	"rogue":      "Rogue v1",
	"siren":      "Siren v2",
}

// ResolveArchiveEmuName returns the archive folder name for an internal emulator id.
func ResolveArchiveEmuName(internalName string) string {
	if name, ok := EmulatorArchiveName[internalName]; ok {
		return name
	}
	return internalName
}
