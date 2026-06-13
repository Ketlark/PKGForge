package fpkg

// This file implements PKG assembly and encryption for PS4 fPKG files.
// Ported from LibOrbisPkg/PKG/PkgBuilder.cs and PkgWriter.cs.
//
// An fPKG consists of:
//  1. PKG header (0x1000 bytes)
//  2. Header signature (RSA-encrypted, 0x100 bytes at 0x1000)
//  3. Body segment (entries: ENTRY_KEYS, IMAGE_KEY, GENERAL_DIGESTS, METAS, DIGESTS, etc.)
//  4. PFS outer image (signed, encrypted, contains pfs_image.dat with the game files)

import (
	"encoding/binary"
	"fmt"
	"io"
)

// ---------------------------------------------------------------------------
// PKG constants and enums
// ---------------------------------------------------------------------------

const (
	pkgContentIDSize = 0x24  // 36 bytes
	pkgPasscodeSize  = 0x20  // 32 bytes
	pkgHashSize      = 32
)

// Content types
const (
	ContentTypeGD = 0x1A // Game Digital Application (pkg_ps4_app)
	ContentTypeAC = 0x1B // Additional Content
	ContentTypeAL = 0x1C // Additional Content (no data)
	ContentTypeDP = 0x1E // Delta Patch
)

// DRM types
const (
	DrmTypeNone = 0x0
	DrmTypePS4  = 0xF
)

// Entry IDs
const (
	EntryIDDigests        uint32 = 0x00000001
	EntryIDEntryKeys      uint32 = 0x00000010
	EntryIDImageKey       uint32 = 0x00000020
	EntryIDGeneralDigests uint32 = 0x00000080
	EntryIDMetas          uint32 = 0x00000100
	EntryIDEntryNames     uint32 = 0x00000200
	EntryIDLicenseDat     uint32 = 0x00000400
	EntryIDLicenseInfo    uint32 = 0x00000401
	EntryIDPSReservedDat  uint32 = 0x00000409
	EntryIDParamSfo       uint32 = 0x00001000
	EntryIDIcon0Png       uint32 = 0x00001200
	EntryIDPic0Png        uint32 = 0x00001220
	EntryIDPic1Png        uint32 = 0x00001006
)

// Entry name to ID mapping (for sce_sys files)
var entryNameToID = map[string]uint32{
	"icon0.png":          EntryIDIcon0Png,
	"pic0.png":           EntryIDPic0Png,
	"pic1.png":           EntryIDPic1Png,
	"param.sfo":          EntryIDParamSfo,
	"license.dat":        EntryIDLicenseDat,
	"license.info":       EntryIDLicenseInfo,
	"nptitle.dat":        0x00000402,
	"npbind.dat":         0x00000403,
	"selfinfo.dat":       0x00000404,
	"imageinfo.dat":      0x00000406,
	"target-deltainfo.dat": 0x00000407,
	"origin-deltainfo.dat": 0x00000408,
	"psreserved.dat":     EntryIDPSReservedDat,
	"pubtoolinfo.dat":    0x00001007,
	"shareparam.json":    0x0000100B,
	"shareoverlayimage.png": 0x0000100C,
	"save_data.png":      0x0000100D,
	"shareprivacyguardimage.png": 0x0000100E,
	"snd0.at9":           0x00001240,
}

// ---------------------------------------------------------------------------
// PKG entry types
// ---------------------------------------------------------------------------

type pkgEntry struct {
	id             uint32
	name           string
	data           []byte
	encrypted      bool
	keyIndex       uint32
	flags1         uint32
	flags2         uint32
	metaOffset     uint32
	metaSize       uint32
	nameTableOffset uint32
	dataOffset     uint32
	dataSize       uint32
}

type metaEntry struct {
	id              uint32
	nameTableOffset uint32
	flags1          uint32
	flags2         uint32
	dataOffset      uint32
	dataSize        uint32
}

func (m *metaEntry) writeTo(w io.Writer) {
	writeBE(w, m.id)
	writeBE(w, m.nameTableOffset)
	writeBE(w, m.flags1)
	writeBE(w, m.flags2)
	writeBE(w, m.dataOffset)
	writeBE(w, m.dataSize)
	// 8 bytes padding
	var pad [8]byte
	w.Write(pad[:])
}

// ---------------------------------------------------------------------------
// PKG builder
// ---------------------------------------------------------------------------

// PKGOptions configures the PKG creation.
type PKGOptions struct {
	ContentID string            // e.g. "UP9000-SCUS94400_00-0000000000000001"
	Passcode  string            // 32 chars, default all zeros
	Files     map[string][]byte // game files (path -> content)
	Icon0     []byte            // icon0.png data
	Pic0      []byte            // pic0.png data (optional)
	Pic1      []byte            // pic1.png data (optional)
	ParamSfo  []byte            // param.sfo data (optional, generated if nil)
	Title     string            // game title
	TitleID   string            // e.g. "SCUS94400"
}

// BuildFPKG creates a complete PS4 fPKG from the given options.
// Returns the raw PKG bytes.
func BuildFPKG(opts PKGOptions) ([]byte, error) {
	if opts.Passcode == "" {
		opts.Passcode = string(DefaultPasscode)
	}
	if len(opts.Passcode) != 32 {
		return nil, fmt.Errorf("fpkg: passcode must be 32 bytes, got %d", len(opts.Passcode))
	}
	if len(opts.ContentID) != 36 {
		return nil, fmt.Errorf("fpkg: content ID must be 36 chars, got %d", len(opts.ContentID))
	}

	// Generate param.sfo if not provided
	sfoData := opts.ParamSfo
	if sfoData == nil {
		sfo := NewPS1ParamSfo(opts.Title, opts.TitleID, opts.ContentID)
		sfoData = sfo.Serialize()
	}

	// Build file map for inner PFS
	files := make(map[string][]byte)
	for k, v := range opts.Files {
		files[k] = v
	}
	// Add sce_sys files
	if files["sce_sys/param.sfo"] == nil {
		files["sce_sys/param.sfo"] = sfoData
	}
	if opts.Icon0 != nil && files["sce_sys/icon0.png"] == nil {
		files["sce_sys/icon0.png"] = opts.Icon0
	}
	if opts.Pic0 != nil && files["sce_sys/pic0.png"] == nil {
		files["sce_sys/pic0.png"] = opts.Pic0
	}
	if opts.Pic1 != nil && files["sce_sys/pic1.png"] == nil {
		files["sce_sys/pic1.png"] = opts.Pic1
	}
	// Add keystone
	if files["sce_sys/keystone"] == nil {
		files["sce_sys/keystone"] = CreateKeystone(opts.Passcode)
	}

	// Build inner PFS (unsigned, unencrypted)
	innerPFS, err := BuildPFS(files, 0x10000, 0x55)
	if err != nil {
		return nil, fmt.Errorf("fpkg: inner PFS: %w", err)
	}

	// PFSC-wrap the inner PFS
	pfscWrapped := wrapPFSC(innerPFS)

	// Compute EKPFS
	ekpfs := ComputeKeys(opts.ContentID, opts.Passcode, 1)

	// Build outer PFS (signed, encrypted) wrapping inner as pfs_image.dat
	outerFiles := map[string][]byte{
		"pfs_image.dat": pfscWrapped,
	}
	outerRoot := BuildFSTree(outerFiles)
	// Set compressedSize for PFSC-wrapped pfs_image.dat
	for _, child := range outerRoot.children {
		if child.name == "pfs_image.dat" {
			child.compressedSize = int64(len(innerPFS))
		}
	}

	outerProps := PfsProperties{
		Root:      outerRoot,
		BlockSize: 0x10000,
		Encrypt:   true,
		Sign:      true,
		EKPFS:     ekpfs,
		Seed:      make([]byte, 16),
		MinBlocks: 0,
	}
	outerPFS, err := BuildPFSImage(outerProps)
	if err != nil {
		return nil, fmt.Errorf("fpkg: outer PFS: %w", err)
	}

	// Build PKG
	pkg, err := assemblePKG(opts, sfoData, outerPFS, ekpfs)
	if err != nil {
		return nil, fmt.Errorf("fpkg: PKG assembly: %w", err)
	}

	return pkg, nil
}

// assemblePKG builds the complete PKG binary from components.
func assemblePKG(opts PKGOptions, sfoData, outerPFS, ekpfs []byte) ([]byte, error) {
	pfsSize := uint64(len(outerPFS))
	bodyOffset := uint64(0x2000)
	pfsImageOffset := uint64(0x80000)
	bodySize := pfsImageOffset - bodyOffset
	packageSize := pfsImageOffset + pfsSize

	// Build entries
	var entries []*pkgEntry

	// ENTRY_KEYS
	entryKeys := buildEntryKeys(opts.ContentID, opts.Passcode)
	entries = append(entries, &pkgEntry{
		id:     EntryIDEntryKeys,
		name:   ".entry_keys",
		data:   entryKeys,
		flags1: 0x60000000,
	})

	// IMAGE_KEY
	imageKey := RSA2048EncryptKey(FakeKeyset.Modulus, ekpfs)
	entries = append(entries, &pkgEntry{
		id:      EntryIDImageKey,
		name:    ".image_key",
		data:    padTo(imageKey, 256),
		flags1:  0xE0000000,
		flags2: 3 << 12,
	})

	// GENERAL_DIGESTS
	gdData := make([]byte, 0x180)
	binary.BigEndian.PutUint16(gdData[0:], 0xD256)
	binary.BigEndian.PutUint16(gdData[2:], 0x100)
	entries = append(entries, &pkgEntry{
		id:     EntryIDGeneralDigests,
		name:   ".general_digests",
		data:   gdData,
		flags1: 0x60000000,
	})

	// METAS (placeholder, filled later)
	metasEntry := &pkgEntry{
		id:   EntryIDMetas,
		name: ".metas",
	}
	entries = append(entries, metasEntry)

	// DIGESTS (placeholder)
	digestsEntry := &pkgEntry{
		id:   EntryIDDigests,
		name: ".digests",
	}
	entries = append(entries, digestsEntry)

	// ENTRY_NAMES
	entryNames := buildEntryNames(entries)
	entries = append(entries, &pkgEntry{
		id:     EntryIDEntryNames,
		name:   ".entry_names",
		data:   entryNames,
		flags1: 0x40000000,
	})

	// LICENSE_DAT
	licenseDat := buildLicenseDat(opts.ContentID)
	entries = append(entries, &pkgEntry{
		id:      EntryIDLicenseDat,
		name:    "license.dat",
		data:    licenseDat,
		flags1:  0x80000000,
		flags2: 3 << 12,
	})

	// LICENSE_INFO
	licenseInfo := make([]byte, 0x200)
	entries = append(entries, &pkgEntry{
		id:      EntryIDLicenseInfo,
		name:    "license.info",
		data:    licenseInfo,
		flags1:  0x80000000,
		flags2: 2 << 12,
	})

	// PARAM_SFO
	entries = append(entries, &pkgEntry{
		id:   EntryIDParamSfo,
		name: "param.sfo",
		data: sfoData,
	})

	// PSRESERVED_DAT
	entries = append(entries, &pkgEntry{
		id:   EntryIDPSReservedDat,
		name: "psreserved.dat",
		data: make([]byte, 0x2000),
	})

	// sce_sys file entries
	// These are passed via opts.Files with "sce_sys/" prefix
	for path, data := range opts.Files {
		if len(path) > 8 && path[:8] == "sce_sys/" {
			fileName := path[8:]
			if fileName == "param.sfo" || fileName == "keystone" {
				continue
			}
			if id, ok := entryNameToID[fileName]; ok {
				entries = append(entries, &pkgEntry{
					id:   id,
					name: fileName,
					data: data,
				})
			}
		}
	}

	// Build name table (first pass)
	nameTable := make([]byte, 1) // starts with null byte
	nameOffsets := make(map[string]uint32)
	nameOffsets[""] = 0
	for _, e := range entries {
		if e.name == "" {
			e.name = "\x00"
		}
		if _, ok := nameOffsets[e.name]; !ok {
			nameOffsets[e.name] = uint32(len(nameTable))
			nameTable = append(nameTable, []byte(e.name)...)
			nameTable = append(nameTable, 0)
		}
	}

	// Update entry_names data
	for _, e := range entries {
		if e.id == EntryIDEntryNames {
			e.data = nameTable
		}
	}

	// Build metas and calculate data offsets
	var metas []metaEntry
	dataOffset := uint32(bodyOffset)
	for _, e := range entries {
		size := uint32(len(e.data))
		if e.id == EntryIDMetas {
			size = uint32(len(entries)) * 32
		}
		if e.id == EntryIDDigests {
			size = uint32(len(entries)) * 32
		}
		aligned := align64(uint64(size), 16)

		me := metaEntry{
			id:              e.id,
			nameTableOffset: nameOffsets[e.name],
			flags1:          e.flags1,
			flags2:         e.flags2,
			dataOffset:      dataOffset,
			dataSize:        size,
		}
		metas = append(metas, me)
		e.metaOffset = dataOffset
		e.metaSize = size
		e.nameTableOffset = nameOffsets[e.name]
		e.dataOffset = dataOffset
		e.dataSize = size
		e.encrypted = (e.flags1 & 0x80000000) != 0
		dataOffset += uint32(aligned)
	}

	// Sort metas by ID
	sortMetas(metas)

	// Update METAS entry data
	metasData := make([]byte, len(entries)*32)
	for i, m := range metas {
		mw := newBytesWriteSeeker(metasData[i*32 : (i+1)*32])
		m.writeTo(mw)
	}
	for _, e := range entries {
		if e.id == EntryIDMetas {
			e.data = metasData
		}
	}

	// Update DIGESTS entry (zero-filled for now)
	for _, e := range entries {
		if e.id == EntryIDDigests {
			e.data = make([]byte, len(entries)*32)
		}
	}

	// Recalculate body size
	actualBodySize := align64(uint64(dataOffset) - bodyOffset, 0x80000)
	bodySize = actualBodySize
	pfsImageOffset = bodyOffset + bodySize
	packageSize = pfsImageOffset + pfsSize

	// Allocate PKG buffer
	totalSize := pfsImageOffset + pfsSize
	pkg := make([]byte, totalSize)
	w := newBytesWriteSeeker(pkg)

	// Write PKG header
	writePKGHeader(w, opts, len(entries), bodyOffset, bodySize, pfsImageOffset, pfsSize, packageSize)

	// Write body entries
	for _, e := range entries {
		seekTo(w, int64(e.metaOffset))
		if e.encrypted {
			writeEncryptedEntry(w, e, opts.ContentID, opts.Passcode)
		} else {
			w.Write(e.data)
		}
	}

	// Write outer PFS
	seekTo(w, int64(pfsImageOffset))
	w.Write(outerPFS)

	// Calculate digests
	calculateDigests(pkg, entries, bodyOffset, bodySize, pfsImageOffset, pfsSize)

	// Re-write header with digests
	writePKGHeader(w, opts, len(entries), bodyOffset, bodySize, pfsImageOffset, pfsSize, packageSize)

	// Final: header digest and signature
	headerDigest := Sha256(pkg[0:0xFE0])
	copy(pkg[0xFE0:], headerDigest)
	headerSHA256 := Sha256(pkg[0:0x1000])
	copy(pkg[0x1000:], RSA2048EncryptKey(PkgPublicKeys[3], headerSHA256))

	return pkg, nil
}

// ---------------------------------------------------------------------------
// PKG header writer
// ---------------------------------------------------------------------------

func writePKGHeader(w io.WriteSeeker, opts PKGOptions, entryCount int, bodyOffset, bodySize, pfsImageOffset, pfsSize, packageSize uint64) {
	seekTo(w, 0x00)
	w.Write([]byte("\x7fCNT"))
	seekTo(w, 0x04)
	writeBE(w, uint32(0x01)) // flags
	writeBE(w, uint32(0))    // unk_0x08
	writeBE(w, uint32(0xF))  // unk_0x0C
	writeBE(w, uint32(entryCount))
	seekTo(w, 0x14)
	writeBE(w, uint16(6))            // sc_entry_count (uint16)
	writeBE(w, uint16(entryCount))   // entry_count_2 (uint16)
	seekTo(w, 0x18)
	writeBE(w, uint32(0x2A80)) // entry_table_offset
	writeBE(w, uint32(0xD00))  // main_ent_data_size
	writeBE(w, bodyOffset)     // body_offset
	seekTo(w, 0x28)
	writeBE(w, bodySize)       // body_size
	seekTo(w, 0x40)
	w.Write([]byte(opts.ContentID))
	seekTo(w, 0x70)
	writeBE(w, uint32(DrmTypePS4))      // drm_type
	writeBE(w, uint32(ContentTypeGD))    // content_type
	writeBE(w, uint32(0x0A000000))       // content_flags
	writeBE(w, uint32(bodyOffset+bodySize)) // promote_size
	seekTo(w, 0x80)
	writeBE(w, uint32(0x20161020))       // version_date
	writeBE(w, uint32(0x1738551))        // version_hash
	writeBE(w, uint32(0))                // unk_0x88
	writeBE(w, uint32(0))                // unk_0x8C
	writeBE(w, uint32(0))                // unk_0x90
	writeBE(w, uint32(0))                // unk_0x94
	writeBE(w, uint32(0))                // iro_tag
	writeBE(w, uint32(1))                // ekc_version

	// PFS image info
	seekTo(w, 0x400)
	writeBE(w, uint32(1))                // unk_0x400
	writeBE(w, uint32(1))                // pfs_image_count
	writeBE(w, uint64(0x80000000000003CC)) // pfs_flags
	seekTo(w, 0x410)
	writeBE(w, pfsImageOffset)           // pfs_image_offset
	seekTo(w, 0x418)
	writeBE(w, pfsSize)                  // pfs_image_size
	seekTo(w, 0x420)
	writeBE(w, uint64(0))                // mount_image_offset
	seekTo(w, 0x428)
	writeBE(w, uint64(0))                // mount_image_size
	seekTo(w, 0x430)
	writeBE(w, packageSize)              // package_size
	seekTo(w, 0x438)
	writeBE(w, uint32(0x10000))          // pfs_signed_size
	seekTo(w, 0x43C)
	writeBE(w, uint32(0xD0000))          // pfs_cache_size
}

// ---------------------------------------------------------------------------
// Entry builders
// ---------------------------------------------------------------------------

func buildEntryKeys(contentID, passcode string) []byte {
	// 32 bytes seed digest + 7*32 byte digests + 7*256 byte keys = 2048
	buf := make([]byte, 2048)
	w := newBytesWriteSeeker(buf)

	// Seed digest
	paddedID := make([]byte, 48)
	copy(paddedID, contentID)
	seedDigest := Sha256(paddedID)
	w.Write(seedDigest)

	// Compute 7 keys
	var digests [7][]byte
	var keys [7][]byte
	for i := uint32(0); i < 7; i++ {
		passcodeKey := ComputeKeys(contentID, passcode, i)
		xored := Sha256(passcodeKey)
		for j := 0; j < 32; j++ {
			xored[j] ^= passcodeKey[j]
		}
		digests[i] = xored

		if PkgPublicKeys[i] != nil {
			keys[i] = RSA2048EncryptKey(PkgPublicKeys[i], passcodeKey)
		} else {
			keys[i] = make([]byte, 256)
		}
	}

	// Override key[0] with passcode encrypted by PkgPublicKeys[0]
	keys[0] = RSA2048EncryptKey(PkgPublicKeys[0], []byte(passcode))

	// Write digests
	for _, d := range digests {
		w.Write(d)
	}
	// Write keys
	for _, k := range keys {
		w.Write(padTo(k, 256))
	}

	return buf
}

func buildEntryNames(entries []*pkgEntry) []byte {
	seen := map[string]bool{"": true}
	buf := []byte{0} // null byte at offset 0
	for _, e := range entries {
		name := e.name
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		buf = append(buf, []byte(name)...)
		buf = append(buf, 0)
	}
	return buf
}

func buildLicenseDat(contentID string) []byte {
	// Minimal license.dat for fPKG
	buf := make([]byte, 0x200)
	copy(buf[0:4], []byte{0x00, 0x00, 0x00, 0x01})
	copy(buf[4:8], []byte{0x00, 0x00, 0x00, 0x00})
	copy(buf[8:], contentID)
	return buf
}

// ---------------------------------------------------------------------------
// Entry encryption
// ---------------------------------------------------------------------------

func writeEncryptedEntry(w io.Writer, e *pkgEntry, contentID, passcode string) {
	// Build meta bytes (32 bytes, big-endian) for IV derivation
	metaBytes := make([]byte, 32)
	mw := newBytesWriteSeeker(metaBytes)
	writeBE(mw, e.id)
	writeBE(mw, e.nameTableOffset)
	writeBE(mw, e.flags1)
	writeBE(mw, e.flags2)
	writeBE(mw, e.dataOffset)
	writeBE(mw, e.dataSize)
	// 8 bytes padding (zeros)

	// Derive key seed from contentID + passcode at key index
	keyIndex := (e.flags2 & 0xF000) >> 12
	keySeed := ComputeKeys(contentID, passcode, keyIndex)

	// iv_key = SHA256(metaBytes || keySeed)
	ivKeySource := append(metaBytes, keySeed...)
	ivKey := Sha256(ivKeySource)
	iv := ivKey[0:16]
	key := ivKey[16:32]

	encrypted := AES128CBCEncryptPad(e.data, key, iv)
	w.Write(encrypted)
}

// ---------------------------------------------------------------------------
// Digest calculation
// ---------------------------------------------------------------------------

func calculateDigests(pkg []byte, entries []*pkgEntry, bodyOffset, bodySize, pfsImageOffset, pfsSize uint64) {
	// PFS image digests
	pfsSignedDigest := Sha256(pkg[pfsImageOffset : pfsImageOffset+0x10000])
	copy(pkg[0x460:], pfsSignedDigest)
	pfsImageDigest := Sha256(pkg[pfsImageOffset : pfsImageOffset+pfsSize])
	copy(pkg[0x440:], pfsImageDigest)

	// Body digest
	bodyDigest := Sha256(pkg[bodyOffset : bodyOffset+bodySize])
	copy(pkg[0x160:], bodyDigest)

	// Digest table hash (hash of the digests entry data)
	for _, e := range entries {
		if e.id == EntryIDDigests {
			digestTableHash := Sha256(e.data)
			copy(pkg[0x140:], digestTableHash)
		}
	}
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

func align64(v, align uint64) uint64 {
	rem := v % align
	if rem != 0 {
		v += align - rem
	}
	return v
}

func padTo(data []byte, size int) []byte {
	if len(data) >= size {
		return data[:size]
	}
	result := make([]byte, size)
	copy(result, data)
	return result
}

func sortMetas(metas []metaEntry) {
	for i := 0; i < len(metas)-1; i++ {
		for j := i + 1; j < len(metas); j++ {
			if metas[j].id < metas[i].id {
				metas[i], metas[j] = metas[j], metas[i]
			}
		}
	}
}

// Big-endian write helpers for PKG (which uses BE for header).
func writeBE(w io.Writer, v interface{}) {
	switch v := v.(type) {
	case uint16:
		var buf [2]byte
		binary.BigEndian.PutUint16(buf[:], v)
		w.Write(buf[:])
	case uint32:
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], v)
		w.Write(buf[:])
	case uint64:
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], v)
		w.Write(buf[:])
	}
}
