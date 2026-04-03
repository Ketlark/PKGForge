package core

import (
	"bytes"
	"fmt"
	"os"
)

var pkgMagicPS4 = []byte{0x7F, 0x43, 0x4E, 0x54} // \x7FCNT

// ValidatePKG checks whether the file at path starts with a valid PS4/PS5 PKG header.
func ValidatePKG(path string) (bool, string) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Sprintf("cannot open: %v", err)
	}
	defer f.Close()

	magic := make([]byte, 4)
	n, err := f.Read(magic)
	if err != nil || n < 4 {
		return false, "file too small to contain a PKG header"
	}
	if bytes.Equal(magic, pkgMagicPS4) {
		return true, "Valid PS4/PS5 PKG header detected"
	}
	return false, fmt.Sprintf("unrecognised header: 0x%X", magic)
}
