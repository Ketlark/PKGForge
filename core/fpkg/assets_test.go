package fpkg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledAssetsIncludeDefaultRuntimeFiles(t *testing.T) {
	paths, err := bundledAssetCachePaths()
	if err != nil {
		t.Fatalf("bundled assets are not readable: %v", err)
	}

	present := make(map[string]bool, len(paths))
	for _, path := range paths {
		present[path] = true
	}
	for _, required := range requiredBundledAssetPaths {
		if !present[required] {
			t.Fatalf("bundled assets missing required runtime file %s", required)
		}
	}
}

func TestEnsureAssetsRefreshesStaleCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	cacheDir, err := AssetsCacheDir()
	if err != nil {
		t.Fatalf("AssetsCacheDir: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(cacheDir, "emus"), 0755); err != nil {
		t.Fatalf("create stale cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, assetsManifestName), []byte("stale\n"), 0644); err != nil {
		t.Fatalf("write stale manifest: %v", err)
	}
	if HasCachedAssets() {
		t.Fatal("stale manifest should not be accepted as a valid asset cache")
	}

	if err := EnsureAssets(nil); err != nil {
		t.Fatalf("EnsureAssets: %v", err)
	}
	if !HasCachedAssets() {
		t.Fatal("freshly extracted assets should match the embedded bundle")
	}
	for _, required := range requiredBundledAssetPaths {
		info, err := os.Stat(filepath.Join(cacheDir, filepath.FromSlash(required)))
		if err != nil {
			t.Fatalf("required runtime file was not extracted: %s: %v", required, err)
		}
		if info.IsDir() {
			t.Fatalf("required runtime path is a directory, not a file: %s", required)
		}
	}
}
