package fpkg

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const pkgToolPath = "/tmp/PkgTool.Core-osx/PkgTool.Core"

// testOutputDir is where test PKGs get written
var testOutputDir = filepath.Join(os.TempDir(), "fpkg-test")

func TestMain(m *testing.M) {
	os.MkdirAll(testOutputDir, 0755)
	code := m.Run()
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// Test 1: SFO serialization
// ---------------------------------------------------------------------------

func TestSFOSerialization(t *testing.T) {
	sfo := NewPS1ParamSfo("Test Game", "SLUS-00100", "UP9000-SLUS00100_00-TESTGAME00000001")
	data := sfo.Serialize()

	// Check magic
	magic := binary.BigEndian.Uint32(data[0:4])
	if magic != 0x00505346 {
		t.Fatalf("SFO magic wrong: got %08X, want 00505346", magic)
	}

	// Check version
	ver := binary.LittleEndian.Uint32(data[4:8])
	if ver != 0x101 {
		t.Fatalf("SFO version wrong: got %X, want 101", ver)
	}

	t.Logf("SFO size: %d bytes", len(data))
	t.Logf("SFO hex (first 64): %s", hex.EncodeToString(data[:min(64, len(data))]))

	// Write to file for inspection
	outPath := filepath.Join(testOutputDir, "test_param.sfo")
	os.WriteFile(outPath, data, 0644)
	t.Logf("Written to %s", outPath)
}

// ---------------------------------------------------------------------------
// Test 2: Keystone generation
// ---------------------------------------------------------------------------

func TestKeystone(t *testing.T) {
	ks := CreateKeystone(string(DefaultPasscode))
	// keystone header (32) + HMAC fingerprint (32) + HMAC final (32) = 96
	if len(ks) != 96 {
		t.Fatalf("Keystone size wrong: got %d, want 96", len(ks))
	}
	t.Logf("Keystone size: %d", len(ks))
	t.Logf("Keystone hex: %s", hex.EncodeToString(ks))
}

// ---------------------------------------------------------------------------
// Test 3: Key derivation
// ---------------------------------------------------------------------------

func TestComputeKeys(t *testing.T) {
	contentID := "UP9000-SLUS00100_00-TESTGAME00000001"
	passcode := string(DefaultPasscode)

	key0 := ComputeKeys(contentID, passcode, 0)
	key1 := ComputeKeys(contentID, passcode, 1) // EKPFS
	key3 := ComputeKeys(contentID, passcode, 3)

	t.Logf("Key0 (passcode): %s", hex.EncodeToString(key0))
	t.Logf("Key1 (EKPFS):    %s", hex.EncodeToString(key1))
	t.Logf("Key3 (dk3):      %s", hex.EncodeToString(key3))

	if len(key0) != 32 || len(key1) != 32 || len(key3) != 32 {
		t.Fatal("Keys must be 32 bytes each")
	}
}

// ---------------------------------------------------------------------------
// Test 4: RSA operations
// ---------------------------------------------------------------------------

func TestRSA2048EncryptDecrypt(t *testing.T) {
	// Test that we can encrypt with the public modulus and decrypt with the private key
	testData := Sha256([]byte("test data for RSA"))

	// Encrypt with FakeKeyset modulus
	encrypted := RSA2048EncryptKey(FakeKeyset.Modulus, testData)
	if len(encrypted) != 256 {
		t.Fatalf("RSA encrypted size wrong: got %d, want 256", len(encrypted))
	}
	t.Logf("RSA encrypted (first 32 bytes): %s", hex.EncodeToString(encrypted[:32]))

	// Raw decrypt with FakeKeyset private key
	decrypted := rsaRawDecrypt(encrypted, &FakeKeyset)
	t.Logf("RSA decrypted (first 32 bytes): %s", hex.EncodeToString(decrypted[:32]))

	// The decrypted data should start with 0x00 0x02 (PKCS#1 v1.5 padding)
	if decrypted[0] != 0x00 || decrypted[1] != 0x02 {
		t.Fatalf("RSA padding wrong: got %02X %02X, want 00 02", decrypted[0], decrypted[1])
	}

	// The last 32 bytes should be our hash
	if hex.EncodeToString(decrypted[224:256]) != hex.EncodeToString(testData) {
		t.Fatalf("RSA payload mismatch:\n  got  %s\n  want %s",
			hex.EncodeToString(decrypted[224:256]),
			hex.EncodeToString(testData))
	}
}

// ---------------------------------------------------------------------------
// Test 5: AES-128-CBC
// ---------------------------------------------------------------------------

func TestAES128CBC(t *testing.T) {
	key := make([]byte, 16)
	iv := make([]byte, 16)
	copy(key, "0123456789abcdef")
	copy(iv, "abcdef0123456789")

	plaintext := make([]byte, 64)
	copy(plaintext, "Hello PKG Forge AES-CBC test data!")

	encrypted := AES128CBCEncrypt(plaintext, key, iv)
	decrypted := AES128CBCDecrypt(encrypted, key, iv)

	expected := "Hello PKG Forge AES-CBC test data!"
	if string(decrypted[:len(expected)]) != expected {
		t.Fatalf("AES-CBC roundtrip failed")
	}
	t.Log("AES-128-CBC roundtrip OK")
}

func TestWriteEncryptedEntryRequiresBlockAlignedData(t *testing.T) {
	contentID := "UP9000-SLUS00100_00-TESTGAME00000001"
	entry := &pkgEntry{
		id:         EntryIDLicenseDat,
		name:       "license.dat",
		data:       make([]byte, 15),
		flags1:     0x80000000,
		flags2:     3 << 12,
		dataOffset: 0x2000,
		dataSize:   15,
	}

	var buf bytes.Buffer
	err := writeEncryptedEntry(&buf, entry, contentID, string(DefaultPasscode))
	if err == nil {
		t.Fatal("writeEncryptedEntry accepted unaligned data")
	}
	if buf.Len() != 0 {
		t.Fatalf("writeEncryptedEntry wrote %d bytes before failing", buf.Len())
	}
}

func TestWriteEncryptedEntryDoesNotPadAlignedData(t *testing.T) {
	contentID := "UP9000-SLUS00100_00-TESTGAME00000001"
	entry := &pkgEntry{
		id:         EntryIDLicenseDat,
		name:       "license.dat",
		data:       make([]byte, 16),
		flags1:     0x80000000,
		flags2:     3 << 12,
		dataOffset: 0x2000,
		dataSize:   16,
	}

	var buf bytes.Buffer
	if err := writeEncryptedEntry(&buf, entry, contentID, string(DefaultPasscode)); err != nil {
		t.Fatalf("writeEncryptedEntry failed: %v", err)
	}
	if buf.Len() != len(entry.data) {
		t.Fatalf("encrypted entry size = %d, want %d", buf.Len(), len(entry.data))
	}
}

// ---------------------------------------------------------------------------
// Test 6: AES-128-XTS
// ---------------------------------------------------------------------------

func TestAES128XTS(t *testing.T) {
	dataKey := make([]byte, 16)
	tweakKey := make([]byte, 16)
	copy(dataKey, "data_key_1234567")
	copy(tweakKey, "tweak_key_123456")

	plaintext := make([]byte, 0x1000) // one sector
	copy(plaintext, "Hello XTS encryption test!")

	encrypted := AES128XTSEncrypt(plaintext, dataKey, tweakKey, 0x1000, 0)
	if len(encrypted) != len(plaintext) {
		t.Fatalf("XTS encrypted size wrong: got %d, want %d", len(encrypted), len(plaintext))
	}

	// Verify it's actually different from plaintext
	same := true
	for i := range plaintext {
		if encrypted[i] != plaintext[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("XTS encryption produced identical output")
	}
	t.Log("AES-128-XTS encryption OK")
}

// ---------------------------------------------------------------------------
// Test 7: PFS key generation
// ---------------------------------------------------------------------------

func TestPFSKeyGeneration(t *testing.T) {
	ekpfs := make([]byte, 32)
	copy(ekpfs, "ekpfs_test_key_1234567890123")
	seed := make([]byte, 16)
	copy(seed, "seed_12345678901")

	tweakKey, dataKey := PfsGenEncKey(ekpfs, seed)
	signKey := PfsGenSignKey(ekpfs, seed)

	t.Logf("Tweak key: %s", hex.EncodeToString(tweakKey))
	t.Logf("Data key:  %s", hex.EncodeToString(dataKey))
	t.Logf("Sign key:  %s", hex.EncodeToString(signKey))

	if len(tweakKey) != 16 || len(dataKey) != 16 || len(signKey) != 32 {
		t.Fatal("PFS key sizes wrong")
	}
}

// ---------------------------------------------------------------------------
// Test 8: Build minimal inner PFS
// ---------------------------------------------------------------------------

func TestBuildInnerPFS(t *testing.T) {
	files := map[string][]byte{
		"sce_sys/param.sfo": make([]byte, 100),
		"sce_sys/keystone":  make([]byte, 80),
		"test_file.bin":     []byte("Hello PFS!"),
	}

	innerPFS, err := BuildPFS(files, nil, 0x10000, 0x55)
	if err != nil {
		t.Fatalf("BuildPFS failed: %v", err)
	}

	t.Logf("Inner PFS size: %d bytes (%.2f KB)", len(innerPFS), float64(len(innerPFS))/1024)

	// Check header magic at offset 8 (after version field)
	magic := binary.LittleEndian.Uint64(innerPFS[8:16])
	if magic != 20130315 {
		t.Fatalf("PFS header magic wrong: got %d, want 20130315", magic)
	}

	outPath := filepath.Join(testOutputDir, "test_inner.pfs")
	os.WriteFile(outPath, innerPFS, 0644)
	t.Logf("Written to %s", outPath)
}

func TestFlatPathTableValuesIncludeInodeNumbers(t *testing.T) {
	root := &fsNode{isDir: true}

	fileIno := &dinodeD32{}
	fileIno.setNumber(7)
	file := &fsNode{name: "pfs_image.dat", parent: root, data: []byte("pfs"), ino: fileIno}

	dirIno := &dinodeD32{}
	dirIno.setNumber(8)
	dir := &fsNode{name: "sce_sys", parent: root, isDir: true, ino: dirIno}

	fpt := buildFlatPathTable([]*fsNode{file, dir})
	if len(fpt) != 16 {
		t.Fatalf("flat path table size = %d, want 16", len(fpt))
	}

	values := map[uint32]uint32{}
	for off := 0; off < len(fpt); off += 8 {
		hash := binary.LittleEndian.Uint32(fpt[off:])
		value := binary.LittleEndian.Uint32(fpt[off+4:])
		values[hash] = value
	}

	if got, want := values[pfsHashFunction("/pfs_image.dat")], uint32(7); got != want {
		t.Fatalf("pfs_image.dat flat path table value = 0x%08X, want 0x%08X", got, want)
	}
	if got, want := values[pfsHashFunction("/sce_sys")], uint32(0x20000008); got != want {
		t.Fatalf("sce_sys flat path table value = 0x%08X, want 0x%08X", got, want)
	}
	if _, exists := values[pfsHashFunction("pfs_image.dat")]; exists {
		t.Fatal("flat path table should hash absolute PFS paths, not relative paths")
	}
}

// ---------------------------------------------------------------------------
// Test 9: Full minimal fPKG build
// ---------------------------------------------------------------------------

func TestBuildMinimalFPKG(t *testing.T) {
	contentID := "UP9000-SLUS00100_00-TESTGAME00000001"

	files := map[string][]byte{
		"test_file.bin": []byte("Hello from PKG Forge! This is a test fPKG."),
	}

	pkgOpts := PKGOptions{
		ContentID: contentID,
		Passcode:  string(DefaultPasscode),
		Files:     files,
		Title:     "Test Game",
		TitleID:   "SLUS-00100",
	}

	pkgData, err := BuildFPKG(pkgOpts)
	if err != nil {
		t.Fatalf("BuildFPKG failed: %v", err)
	}

	t.Logf("PKG size: %d bytes (%.2f MB)", len(pkgData), float64(len(pkgData))/(1024*1024))

	// Verify minimum PKG size (header + body + at least 1 PFS block)
	if len(pkgData) < 0x100000 {
		t.Fatalf("PKG size = 0x%X, want at least 0x100000", len(pkgData))
	}
	// Verify pfs_image_size is non-zero and within the PKG
	pfsImageSize := binary.BigEndian.Uint64(pkgData[0x418:0x420])
	if pfsImageSize == 0 {
		t.Fatalf("pfs_image_size = 0, want non-zero")
	}
	if got, want := binary.BigEndian.Uint64(pkgData[0x430:0x438]), uint64(len(pkgData)); got != want {
		t.Fatalf("header package_size = 0x%X, want 0x%X", got, want)
	}

	// Check PKG magic
	magic := string(pkgData[0:4])
	if magic != "\x7fCNT" {
		t.Fatalf("PKG magic wrong: got %q, want \\x7fCNT", magic)
	}

	// Write to file
	outPath := filepath.Join(testOutputDir, "test_minimal.pkg")
	if err := os.WriteFile(outPath, pkgData, 0644); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	t.Logf("Written to %s", outPath)

	// Try to validate with PkgTool.Core
	validateWithPkgTool(t, outPath)
}

// ---------------------------------------------------------------------------
// Test 10: PS1 fPKG with Spider-Man (France) — only if file exists
// ---------------------------------------------------------------------------

func TestPS1FPKGSpiderMan(t *testing.T) {
	cuePath := "/Users/dehoux/Downloads/Spider-Man (France)/Spider-Man (France)/Spider-Man (France).cue"
	if _, err := os.Stat(cuePath); os.IsNotExist(err) {
		t.Skip("Spider-Man (France) disc image not found")
	}

	outPath := filepath.Join(testOutputDir, "spider-man-fr.pkg")

	err := CreatePS1FPKG(PS1FPKGOptions{
		CuePath:    cuePath,
		OutputPath: outPath,
		Title:      "Spider-Man",
		TitleID:    "SCES-02752",
		Emulator:   "ps1_emu",
	})
	if err != nil {
		// If the error is about missing emulator files, skip instead of fail
		if strings.Contains(err.Error(), "missing emulator file") {
			t.Skip("Emulator assets not available (run once with internet to download)")
		}
		t.Fatalf("CreatePS1FPKG failed: %v", err)
	}

	// Check file was created
	stat, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("Output file not found: %v", err)
	}
	t.Logf("PS1 PKG size: %d bytes (%.2f MB)", stat.Size(), float64(stat.Size())/(1024*1024))

	validateWithPkgTool(t, outPath)
}

// ---------------------------------------------------------------------------
// Helper: validate with PkgTool.Core
// ---------------------------------------------------------------------------

func validateWithPkgTool(t *testing.T, pkgPath string) {
	t.Helper()

	if _, err := os.Stat(pkgToolPath); os.IsNotExist(err) {
		t.Skip("PkgTool.Core not found at " + pkgToolPath)
	}

	// pkg_listentries
	t.Log("=== pkg_listentries ===")
	cmd := exec.Command(pkgToolPath, "pkg_listentries", pkgPath)
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Logf("pkg_listentries error: %v", err)
	}

	t.Log("=== pkg_validate --verbose ===")
	cmd = exec.Command(pkgToolPath, "pkg_validate", "--verbose", pkgPath)
	out, err = cmd.CombinedOutput()
	validateOutput := string(out)
	t.Logf("%s", out)
	if err != nil {
		t.Errorf("pkg_validate FAILED: %v", err)
	}
	for _, marker := range []string{"[FAIL]", "[ERROR]"} {
		if strings.Contains(validateOutput, marker) {
			t.Errorf("pkg_validate reported %s", marker)
		}
	}
	for _, expected := range []string{
		"Content Digest",
		"Major Param Digest",
		"Param Digest",
		"Debug RIF Signature",
	} {
		if !strings.Contains(validateOutput, expected) {
			t.Errorf("pkg_validate did not check %q", expected)
		}
	}

	// pkg_extract to temp dir
	extractDir := filepath.Join(testOutputDir, "extract-"+filepath.Base(pkgPath))
	os.RemoveAll(extractDir)
	os.MkdirAll(extractDir, 0755)

	t.Log("=== pkg_extract ===")
	cmd = exec.Command(pkgToolPath, "pkg_extract", pkgPath, extractDir)
	out, err = cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Errorf("pkg_extract FAILED: %v", err)
	} else {
		t.Logf("pkg_extract SUCCESS — files extracted to %s", extractDir)

		// List extracted files
		filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				rel, _ := filepath.Rel(extractDir, path)
				t.Logf("  extracted: %s (%d bytes)", rel, info.Size())
			}
			return nil
		})
		assertRuntimeExtraction(t, extractDir, "pkg_extract")
	}

	// pkg_extractinnerpfs
	t.Log("=== pkg_extractinnerpfs ===")
	innerPath := filepath.Join(testOutputDir, "inner-"+filepath.Base(pkgPath)+".pfs")
	cmd = exec.Command(pkgToolPath, "pkg_extractinnerpfs", pkgPath, innerPath)
	out, err = cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Logf("pkg_extractinnerpfs error: %v", err)
	} else {
		t.Logf("Inner PFS extracted to %s", innerPath)

		// Extract inner PFS
		innerExtractDir := filepath.Join(testOutputDir, "inner-extract-"+filepath.Base(pkgPath))
		os.RemoveAll(innerExtractDir)
		os.MkdirAll(innerExtractDir, 0755)
		cmd = exec.Command(pkgToolPath, "pfs_extract", innerPath, innerExtractDir)
		out, err = cmd.CombinedOutput()
		t.Logf("%s", out)
		if err != nil {
			t.Logf("pfs_extract (inner) error: %v", err)
		} else {
			t.Logf("Inner PFS extracted to %s", innerExtractDir)
			filepath.Walk(innerExtractDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					rel, _ := filepath.Rel(innerExtractDir, path)
					t.Logf("  inner file: %s (%d bytes)", rel, info.Size())
				}
				return nil
			})

			if innerPFSPathExists(innerExtractDir, "data/disc1.bin") &&
				!innerPFSPathExists(innerExtractDir, "sce_module/libc.prx") {
				t.Log("Skipping pkg_extractinnerpfs content assertions: PkgTool.Core leaves the final 1 MiB of each 8 MiB copy chunk unread for large inner PFS images")
			} else {
				assertRuntimeExtraction(t, innerExtractDir, "inner PFS")
			}
		}
	}

	makeGP4Dir := filepath.Join(testOutputDir, "makegp4-"+filepath.Base(pkgPath))
	os.RemoveAll(makeGP4Dir)
	os.MkdirAll(makeGP4Dir, 0755)
	t.Log("=== pkg_makegp4 ===")
	cmd = exec.Command(pkgToolPath, "pkg_makegp4", pkgPath, makeGP4Dir)
	out, err = cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Errorf("pkg_makegp4 FAILED: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test 11: Mersenne Twister determinism
// ---------------------------------------------------------------------------

func TestMersenneTwister(t *testing.T) {
	seeds := []uint32{0x12345678, 0x9ABCDEF0, 0x13579BDF, 0x2468ACE0, 0xCAFEBABE, 0xDEADBEEF, 0xBAADF00D, 0x8BADF00D}
	mt := newMersenneTwister(seeds)

	// Generate some values and check they're deterministic
	var vals [10]uint32
	for i := 0; i < 10; i++ {
		vals[i] = mt.Uint32()
	}

	// Re-create with same seeds — must produce same sequence
	mt2 := newMersenneTwister(seeds)
	for i := 0; i < 10; i++ {
		v := mt2.Uint32()
		if v != vals[i] {
			t.Fatalf("MT not deterministic at index %d: got %08X, want %08X", i, v, vals[i])
		}
	}
	t.Logf("First 5 MT values: %08X %08X %08X %08X %08X", vals[0], vals[1], vals[2], vals[3], vals[4])
}

// ---------------------------------------------------------------------------
// Test 12: GP4 generation
// ---------------------------------------------------------------------------

func TestGP4Generation(t *testing.T) {
	files := map[string][]byte{
		"sce_sys/param.sfo": make([]byte, 100),
		"test.bin":          []byte("GP4 test"),
	}

	root := BuildFSTree(files, nil)
	t.Logf("FS tree root children: %d", len(root.children))
	for _, c := range root.children {
		t.Logf("  %s (dir=%v, size=%d)", c.name, c.isDir, c.size())
		if c.isDir {
			for _, cc := range c.children {
				t.Logf("    %s (dir=%v, size=%d)", cc.name, cc.isDir, cc.size())
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func innerPFSPathExists(root, rel string) bool {
	for _, candidate := range innerPFSCandidates(root, rel) {
		if _, err := os.Stat(candidate); err == nil {
			return true
		}
	}
	return false
}

func readInnerPFSFile(root, rel string) ([]byte, error) {
	for _, candidate := range innerPFSCandidates(root, rel) {
		if data, err := os.ReadFile(candidate); err == nil {
			return data, nil
		}
	}
	return nil, os.ErrNotExist
}

func assertRuntimeExtraction(t *testing.T, root, label string) {
	t.Helper()

	if keystone, err := readInnerPFSFile(root, "sce_sys/keystone"); err == nil {
		expected := CreateKeystone(string(DefaultPasscode))
		if !bytes.Equal(keystone, expected) {
			t.Errorf("%s: sce_sys/keystone does not match package passcode", label)
		}
	}
	if !innerPFSPathExists(root, "sce_sys/keystone") {
		t.Errorf("%s: sce_sys/keystone should be present for /app0 runtime access", label)
	}
	if !innerPFSPathExists(root, "data/disc1.bin") {
		return
	}
	for _, required := range []string{
		"eboot.bin",
		"config-title.txt",
		"sce_sys/keystone",
		"sce_module/libc.prx",
		"sce_module/libSceFios2.prx",
		"sce_module/libSceNpToolkit2.prx",
		"data/disc1.bin",
		"data/disc1.cue",
		"data/disc1.toc",
	} {
		if !innerPFSPathExists(root, required) {
			t.Errorf("%s: missing required PS1 runtime file %s", label, required)
		}
	}
	for _, bios := range []string{"SCPH5500.bin", "SCPH5501.bin", "SCPH5502.bin"} {
		if !innerPFSPathExists(root, "assets/PS1HD/bios/"+bios) && !innerPFSPathExists(root, "bios/"+bios) {
			t.Errorf("%s: missing required PS1 BIOS file %s", label, bios)
		}
	}
	cfg, err := readInnerPFSFile(root, "config-title.txt")
	if err != nil {
		t.Errorf("%s: read config-title.txt: %v", label, err)
	} else if !strings.Contains(string(cfg), `--image="data/disc1.bin"`) {
		t.Errorf("%s: config-title.txt missing PS1HD image path:\n%s", label, cfg)
	}
	for _, obsoleteRootPath := range []string{"libc.prx", "libSceFios2.prx", "libSceNpToolkit2.prx"} {
		if innerPFSPathExists(root, obsoleteRootPath) {
			t.Errorf("%s: should not place PS1 runtime module at app0 root: %s", label, obsoleteRootPath)
		}
	}
}

func innerPFSCandidates(root, rel string) []string {
	return []string{
		filepath.Join(root, rel),
		filepath.Join(root, "uroot", rel),
	}
}

// Verify PkgTool.Core is available
func TestPkgToolAvailable(t *testing.T) {
	if _, err := os.Stat(pkgToolPath); os.IsNotExist(err) {
		t.Skip("PkgTool.Core not available")
	}
	cmd := exec.Command(pkgToolPath)
	out, err := cmd.CombinedOutput()
	t.Logf("PkgTool.Core output:\n%s", out)
	if err != nil {
		t.Logf("Exit code: %v (expected for no args)", err)
	}
}

// Print test output dir
func TestPrintOutputDir(t *testing.T) {
	t.Logf("Test output directory: %s", testOutputDir)
	fmt.Printf("Test output directory: %s\n", testOutputDir)
}
