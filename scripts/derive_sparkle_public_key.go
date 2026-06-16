// Derive Sparkle SUPublicEDKey (base64) from an exported EdDSA private key file.
// Usage: go run scripts/derive_sparkle_public_key.go /path/to/private.key
package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ed25519"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: derive_sparkle_public_key <private-key-file>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		fmt.Fprintln(os.Stderr, "decode private key:", err)
		os.Exit(1)
	}
	if len(seed) != ed25519.SeedSize {
		fmt.Fprintf(os.Stderr, "expected %d-byte Ed25519 seed, got %d bytes\n", ed25519.SeedSize, len(seed))
		os.Exit(1)
	}
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	fmt.Println(base64.StdEncoding.EncodeToString(pub))
}
