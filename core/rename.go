package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// SuggestRename generates a clean filename from PKG metadata.
// Returns the new filename (not full path) and the PKGInfo used.
func SuggestRename(path string) (string, PKGInfo) {
	info := InspectPKG(path)
	if !info.Valid || info.TitleID == "" {
		return filepath.Base(path), info
	}

	ext := filepath.Ext(path)
	if ext == "" {
		ext = ".pkg"
	}

	typeSuffix := sanitizeForFilename(info.ContentType)
	name := fmt.Sprintf("%s-%s%s", info.TitleID, typeSuffix, ext)

	return name, info
}

// RenamePKG renames the file on disk to the suggested name.
// Returns the new full path.
func RenamePKG(path string) (string, error) {
	newName, _ := SuggestRename(path)
	if newName == filepath.Base(path) {
		return path, nil
	}

	dir := filepath.Dir(path)
	newPath := filepath.Join(dir, newName)

	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("target already exists: %s", newName)
	}

	if err := os.Rename(path, newPath); err != nil {
		return "", fmt.Errorf("rename failed: %w", err)
	}
	return newPath, nil
}

func sanitizeForFilename(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = unsafeChars.ReplaceAllString(s, "")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}
