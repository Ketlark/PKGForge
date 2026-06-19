package fpkg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSignedEncryptedPFSToFileMatchesInMemory(t *testing.T) {
	inner := bytes.Repeat([]byte{0xAB}, pfscBlockSize*2+321)
	ekpfs := ComputeKeys("UP9000-TEST00000_00-TESTGAME00000001", string(DefaultPasscode), 1)

	outerRoot := BuildFSTree(map[string][]byte{"pfs_image.dat": inner}, nil)
	for _, child := range outerRoot.children {
		if child.name == "pfs_image.dat" {
			child.compressedSize = int64(len(inner))
		}
	}

	props := PfsProperties{
		Root:      outerRoot,
		BlockSize: 0x10000,
		Encrypt:   true,
		Sign:      true,
		EKPFS:     ekpfs,
		Seed:      make([]byte, 16),
		MinBlocks: 0,
	}

	inMemory, err := BuildPFSImage(props)
	if err != nil {
		t.Fatalf("BuildPFSImage: %v", err)
	}

	dir := t.TempDir()
	onDiskPath := filepath.Join(dir, "outer.pfs")
	written, err := BuildSignedEncryptedPFSToFile(props, onDiskPath)
	if err != nil {
		t.Fatalf("BuildSignedEncryptedPFSToFile: %v", err)
	}
	fileData, err := os.ReadFile(onDiskPath)
	if err != nil {
		t.Fatalf("read outer: %v", err)
	}
	if int64(len(fileData)) != written {
		t.Fatalf("reported size %d != file size %d", written, len(fileData))
	}
	if !bytes.Equal(inMemory, fileData) {
		t.Fatalf("file-backed signed outer PFS differs from in-memory build")
	}
}

func TestAES128XTSInPlaceMatchesBuffer(t *testing.T) {
	dataKey := make([]byte, 16)
	tweakKey := make([]byte, 16)
	copy(dataKey, "data_key_1234567")
	copy(tweakKey, "tweak_key_123456")

	plaintext := make([]byte, 0x10000)
	copy(plaintext[0x1000:], "Hello in-place XTS test payload")

	expected := AES128XTSEncryptSkipBlocks(
		append([]byte(nil), plaintext...),
		dataKey, tweakKey, 0x1000, 16, 16, map[int64]bool{2: true},
	)

	dir := t.TempDir()
	path := filepath.Join(dir, "xts.bin")
	if err := os.WriteFile(path, plaintext, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	if err := AES128XTSEncryptSkipBlocksInPlace(f, int64(len(plaintext)), dataKey, tweakKey, 0x1000, 16, 16, map[int64]bool{2: true}); err != nil {
		t.Fatalf("in-place XTS: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(expected, got) {
		t.Fatalf("in-place XTS output differs from buffer encryption")
	}
}
