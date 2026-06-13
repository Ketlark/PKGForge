package main

// encrypt_assets packs the staged emulator files into an AES-256-GCM encrypted
// archive and writes the result as core/fpkg/assets.dat for embedding.
//
// Usage:
//   go run tools/encrypt_assets.go
//
// Prerequisites: the assets must have been downloaded first (run the app once
// or call EnsureAssets manually so the cache exists).
//
// The archive format is a simple tar.zst encrypted with AES-256-GCM.
// The encryption key is hardcoded in assets.go (compiled into the binary).

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Must match the key in assets.go
var encryptionKey = []byte("01234567890123456789012345678901")

func main() {
	cacheDir := filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "pkg-forge", "assets")
	outPath := filepath.Join("core", "fpkg", "assets.dat")

	src := cacheDir

	// Collect files
	var files []string
	filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		// Only include essential files
		if isEssential(rel) {
			files = append(files, path)
		}
		return nil
	})

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "No assets found in cache. Run the app once to download assets.")
		os.Exit(1)
	}

	fmt.Printf("Packing %d files...\n", len(files))

	// Create tar.gz in memory, then encrypt
	var tarBuf bytes.Buffer
	zw := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(zw)

	for _, f := range files {
		rel, _ := filepath.Rel(src, f)
		info, _ := os.Stat(f)

		tw.WriteHeader(&tar.Header{
			Name: rel,
			Size: info.Size(),
			Mode: 0644,
		})

		fh, _ := os.Open(f)
		io.Copy(tw, fh)
		fh.Close()
	}

	tw.Close()
	zw.Close()

	encrypted, err := encrypt(bytes.NewReader(tarBuf.Bytes()), encryptionKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Encrypt error:", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outPath, encrypted, 0644); err != nil {
		fmt.Fprintln(os.Stderr, "Write error:", err)
		os.Exit(1)
	}

	fmt.Printf("Wrote %s (%.1f MB)\n", outPath, float64(len(encrypted))/(1024*1024))
}

func isEssential(rel string) bool {
	rel = filepath.ToSlash(rel)

	if strings.HasPrefix(rel, "lua_include/") {
		return true
	}

	// Strip leading "emus/" if present (cache layout)
	rel = strings.TrimPrefix(rel, "emus/")

	// Extract emulator name (first path component)
	emu := rel
	if idx := strings.Index(rel, "/"); idx >= 0 {
		emu = rel[:idx]
	}

	switch emu {
	case "ps1hd":
		return strings.Contains(rel, "eboot.bin") ||
			strings.Contains(rel, "sce_module/")
	case "Jak v2", "Rogue v1":
		return strings.Contains(rel, "eboot.bin") ||
			strings.Contains(rel, "sce_module/") ||
			strings.Contains(rel, "ps2-emu-compiler") ||
			strings.Contains(rel, ".crack")
	}

	return false
}

func encrypt(plaintext io.Reader, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Read all plaintext
	data, err := io.ReadAll(plaintext)
	if err != nil {
		return nil, err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	// Encrypt: nonce || ciphertext || tag
	sealed := gcm.Seal(nil, nonce, data, nil)
	result := make([]byte, len(nonce)+len(sealed))
	copy(result, nonce)
	copy(result[len(nonce):], sealed)

	return result, nil
}
