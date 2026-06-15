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
	"sort"
	"time"
)

// ---------------------------------------------------------------------------
// PKG constants and enums
// ---------------------------------------------------------------------------

const (
	pkgContentIDSize = 0x24 // 36 bytes
	pkgPasscodeSize  = 0x20 // 32 bytes
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
	EntryIDPlayGoChunkDat uint32 = 0x00001001
	EntryIDPlayGoChunkSha uint32 = 0x00001002
	EntryIDPlayGoManifest uint32 = 0x00001003
	EntryIDIcon0Png       uint32 = 0x00001200
	EntryIDPic0Png        uint32 = 0x00001220
	EntryIDPic1Png        uint32 = 0x00001006
	EntryIDSaveDataPng    uint32 = 0x0000100D
)

// Entry name to ID mapping (for sce_sys files)
var entryNameToID = map[string]uint32{
	"icon0.png":                  EntryIDIcon0Png,
	"pic0.png":                   EntryIDPic0Png,
	"pic1.png":                   EntryIDPic1Png,
	"param.sfo":                  EntryIDParamSfo,
	"license.dat":                EntryIDLicenseDat,
	"license.info":               EntryIDLicenseInfo,
	"nptitle.dat":                0x00000402,
	"npbind.dat":                 0x00000403,
	"selfinfo.dat":               0x00000404,
	"imageinfo.dat":              0x00000406,
	"target-deltainfo.dat":       0x00000407,
	"origin-deltainfo.dat":       0x00000408,
	"psreserved.dat":             EntryIDPSReservedDat,
	"playgo-chunk.dat":           EntryIDPlayGoChunkDat,
	"playgo-chunk.sha":           EntryIDPlayGoChunkSha,
	"playgo-manifest.xml":        EntryIDPlayGoManifest,
	"pubtoolinfo.dat":            0x00001007,
	"shareparam.json":            0x0000100B,
	"shareoverlayimage.png":      0x0000100C,
	"save_data.png":              EntryIDSaveDataPng,
	"shareprivacyguardimage.png": 0x0000100E,
	"snd0.at9":                   0x00001240,
}

// ---------------------------------------------------------------------------
// PKG entry types
// ---------------------------------------------------------------------------

type pkgEntry struct {
	id              uint32
	name            string
	data            []byte
	encrypted       bool
	keyIndex        uint32
	flags1          uint32
	flags2          uint32
	metaOffset      uint32
	metaSize        uint32
	nameTableOffset uint32
	dataOffset      uint32
	dataSize        uint32
}

type metaEntry struct {
	id              uint32
	nameTableOffset uint32
	flags1          uint32
	flags2          uint32
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
	ContentID  string            // e.g. "UP9000-SCUS94400_00-0000000000000001"
	Passcode   string            // 32 chars, default all zeros
	Files      map[string][]byte // game files (path -> content)
	Icon0      []byte            // icon0.png data
	Pic0       []byte            // pic0.png data (optional)
	Pic1       []byte            // pic1.png data (optional)
	ParamSfo   []byte            // param.sfo data (optional, generated if nil)
	Title      string            // game title
	TitleID    string            // e.g. "SCUS94400"
	OnProgress func(percent float64, phase string)
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

	packageSFO, err := preparePackageSFO(opts)
	if err != nil {
		return nil, err
	}
	packageSFOData := packageSFO.Serialize()

	// Build file map for inner PFS
	files := make(map[string][]byte)
	for k, v := range opts.Files {
		files[k] = v
	}
	runtimeSFOData := opts.Files["sce_sys/param.sfo"]
	if len(runtimeSFOData) == 0 {
		runtimeSFOData = packageSFOData
	}
	files["sce_sys/param.sfo"] = runtimeSFOData
	if opts.Icon0 != nil && files["sce_sys/icon0.png"] == nil {
		files["sce_sys/icon0.png"] = opts.Icon0
	}
	if files["sce_sys/icon0.png"] == nil {
		files["sce_sys/icon0.png"] = defaultIcon0PNG(opts.TitleID)
	}
	if files["sce_sys/save_data.png"] == nil {
		files["sce_sys/save_data.png"] = defaultSaveDataPNG(opts.TitleID)
	}
	if opts.Pic0 != nil && files["sce_sys/pic0.png"] == nil {
		files["sce_sys/pic0.png"] = opts.Pic0
	}
	if opts.Pic1 != nil && files["sce_sys/pic1.png"] == nil {
		files["sce_sys/pic1.png"] = opts.Pic1
	}
	if files["sce_sys/pic1.png"] == nil {
		files["sce_sys/pic1.png"] = defaultPic1PNG(opts.TitleID)
	}
	// Keystone is derived from the package passcode. Keep it centralized here so
	// every frontend-specific project builder stays consistent with PKG keys.
	files["sce_sys/keystone"] = CreateKeystone(opts.Passcode)
	opts.Files = files

	// Build inner PFS (unsigned, unencrypted).
	// All files including sce_sys/ go into the inner PFS — the PS4 runtime
	// needs /app0/sce_sys/param.sfo, save_data.png, etc. at launch.
	if opts.OnProgress != nil {
		opts.OnProgress(15, "Building inner filesystem")
	}
	innerPFS, err := BuildPFS(files, 0x10000, 0x55)
	if err != nil {
		return nil, fmt.Errorf("fpkg: inner PFS: %w", err)
	}

	// PFSC-wrap the inner PFS
	if opts.OnProgress != nil {
		opts.OnProgress(35, "Wrapping filesystem")
	}
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
	if opts.OnProgress != nil {
		opts.OnProgress(55, "Building encrypted filesystem")
	}
	outerPFS, err := BuildPFSImage(outerProps)
	if err != nil {
		return nil, fmt.Errorf("fpkg: outer PFS: %w", err)
	}

	// Build PKG
	if opts.OnProgress != nil {
		opts.OnProgress(80, "Assembling package")
	}
	pkg, err := assemblePKG(opts, packageSFO, packageSFOData, outerPFS, ekpfs, uint64(len(innerPFS)))
	if err != nil {
		return nil, fmt.Errorf("fpkg: PKG assembly: %w", err)
	}
	if opts.OnProgress != nil {
		opts.OnProgress(95, "Package assembled")
	}

	return pkg, nil
}

func preparePackageSFO(opts PKGOptions) (*ParamSfo, error) {
	sfoData := opts.ParamSfo
	if len(sfoData) == 0 {
		sfoData = opts.Files["sce_sys/param.sfo"]
	}

	var sfo *ParamSfo
	if len(sfoData) > 0 {
		parsed, err := ParseParamSfo(sfoData)
		if err != nil {
			return nil, fmt.Errorf("fpkg: param.sfo: %w", err)
		}
		sfo = parsed
	} else {
		sfo = NewPS1ParamSfo(opts.Title, opts.TitleID, opts.ContentID)
	}

	sfo.Set("CONTENT_ID", opts.ContentID, 48)
	if opts.TitleID != "" {
		sfo.Set("TITLE_ID", normalizeTitleID(opts.TitleID), 12)
	}
	sfo.Set("PUBTOOLINFO", "", 0x200)
	sfo.SetInt("PUBTOOLMINVER", 0x02990000)
	sfo.SetInt("PUBTOOLVER", 0x03380000)
	return sfo, nil
}

// assemblePKG builds the complete PKG binary from components.
func assemblePKG(opts PKGOptions, sfo *ParamSfo, sfoData, outerPFS, ekpfs []byte, innerPFSSize uint64) ([]byte, error) {
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
		name:   "",
		data:   entryKeys,
		flags1: 0x60000000,
	})

	// IMAGE_KEY
	imageKey := RSA2048EncryptKey(FakeKeyset.Modulus, ekpfs)
	entries = append(entries, &pkgEntry{
		id:     EntryIDImageKey,
		name:   "",
		data:   padTo(imageKey, 256),
		flags1: 0xE0000000,
		flags2: 3 << 12,
	})

	// GENERAL_DIGESTS
	gdData := make([]byte, 0x180)
	binary.BigEndian.PutUint16(gdData[0:], 0xD256)
	binary.BigEndian.PutUint16(gdData[2:], 0x100)
	entries = append(entries, &pkgEntry{
		id:     EntryIDGeneralDigests,
		name:   "",
		data:   gdData,
		flags1: 0x60000000,
	})

	// METAS (placeholder, filled later)
	metasEntry := &pkgEntry{
		id:     EntryIDMetas,
		name:   "",
		flags1: 0x60000000,
	}
	entries = append(entries, metasEntry)

	// DIGESTS (placeholder)
	digestsEntry := &pkgEntry{
		id:     EntryIDDigests,
		name:   "",
		flags1: 0x40000000,
	}
	entries = append(entries, digestsEntry)

	// ENTRY_NAMES
	entryNames := buildEntryNames(entries)
	entries = append(entries, &pkgEntry{
		id:     EntryIDEntryNames,
		name:   "",
		data:   entryNames,
		flags1: 0x40000000,
	})

	entries = append(entries,
		&pkgEntry{
			id:   EntryIDPlayGoChunkDat,
			name: "playgo-chunk.dat",
			data: buildPlayGoChunkDat(opts.ContentID, 0, innerPFSSize),
		},
		&pkgEntry{
			id:   EntryIDPlayGoChunkSha,
			name: "playgo-chunk.sha",
		},
		&pkgEntry{
			id:   EntryIDPlayGoManifest,
			name: "playgo-manifest.xml",
			data: defaultPlayGoManifest(),
		},
	)

	// LICENSE_DAT
	licenseDat, err := buildLicenseDat(opts.ContentID)
	if err != nil {
		return nil, err
	}
	entries = append(entries, &pkgEntry{
		id:     EntryIDLicenseDat,
		name:   "",
		data:   licenseDat,
		flags1: 0x80000000,
		flags2: 3 << 12,
	})

	entries = append(entries, &pkgEntry{
		id:     EntryIDLicenseInfo,
		name:   "",
		data:   buildLicenseInfo(opts.ContentID),
		flags1: 0x80000000,
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
		name: "",
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
		if e.name != "" {
			if _, ok := nameOffsets[e.name]; !ok {
				nameOffsets[e.name] = uint32(len(nameTable))
				nameTable = append(nameTable, []byte(e.name)...)
				nameTable = append(nameTable, 0)
			}
		}
	}

	// Update entry_names data
	for _, e := range entries {
		if e.id == EntryIDEntryNames {
			e.data = nameTable
		}
	}

	updatePlayGoChunkShaSize(entries, bodyOffset, pfsSize)

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
			flags2:          e.flags2,
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
	actualBodySize := align64(uint64(dataOffset), 0x80000) - bodyOffset
	bodySize = actualBodySize
	pfsImageOffset = bodyOffset + bodySize
	packageSize = pfsImageOffset + pfsSize
	mainEntDataSize := calcMainEntryDataSize(entries)
	if chunkDat := findEntry(entries, EntryIDPlayGoChunkDat); chunkDat != nil {
		chunkDat.data = buildPlayGoChunkDat(opts.ContentID, packageSize, innerPFSSize)
	}
	sfo.Set("PUBTOOLINFO", buildPubToolInfo(packageSize), 0x200)
	finalSFOData := sfo.Serialize()
	if uint32(len(finalSFOData)) != findEntry(entries, EntryIDParamSfo).dataSize {
		return nil, fmt.Errorf("fpkg: final param.sfo size changed from %d to %d", findEntry(entries, EntryIDParamSfo).dataSize, len(finalSFOData))
	}
	findEntry(entries, EntryIDParamSfo).data = finalSFOData

	// Allocate PKG buffer
	totalSize := packageSize
	pkg := make([]byte, totalSize)
	w := newBytesWriteSeeker(pkg)

	// Write PKG header
	writePKGHeader(w, opts, len(entries), mainEntDataSize, bodyOffset, bodySize, pfsImageOffset, pfsSize, packageSize)

	// Write body entries
	for _, e := range entries {
		seekTo(w, int64(e.metaOffset))
		if e.encrypted {
			if err := writeEncryptedEntry(w, e, opts.ContentID, opts.Passcode); err != nil {
				return nil, err
			}
		} else {
			w.Write(e.data)
		}
	}

	// Write outer PFS
	seekTo(w, int64(pfsImageOffset))
	w.Write(outerPFS)

	if err := writePlayGoChunkSha(pkg, entries, pfsImageOffset, packageSize); err != nil {
		return nil, err
	}

	// Calculate digests
	calculateDigests(pkg, entries, opts.ContentID, sfo, bodyOffset, bodySize, pfsImageOffset, pfsSize)

	// Re-write header with digests
	writePKGHeader(w, opts, len(entries), mainEntDataSize, bodyOffset, bodySize, pfsImageOffset, pfsSize, packageSize)

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

func writePKGHeader(w io.WriteSeeker, opts PKGOptions, entryCount int, mainEntDataSize uint32, bodyOffset, bodySize, pfsImageOffset, pfsSize, packageSize uint64) {
	seekTo(w, 0x00)
	w.Write([]byte("\x7fCNT"))
	seekTo(w, 0x04)
	writeBE(w, uint32(0x40000001)) // flags: Unknown | INTERNAL (relaxes SELF auth for fPKGs)
	writeBE(w, uint32(0))    // unk_0x08
	writeBE(w, uint32(0xF))  // unk_0x0C
	writeBE(w, uint32(entryCount))
	seekTo(w, 0x14)
	writeBE(w, uint16(6))          // sc_entry_count (uint16)
	writeBE(w, uint16(entryCount)) // entry_count_2 (uint16)
	seekTo(w, 0x18)
	writeBE(w, uint32(0x2A80)) // entry_table_offset
	writeBE(w, mainEntDataSize)
	writeBE(w, bodyOffset) // body_offset
	seekTo(w, 0x28)
	writeBE(w, bodySize) // body_size
	seekTo(w, 0x40)
	w.Write([]byte(opts.ContentID))
	seekTo(w, 0x70)
	writeBE(w, uint32(DrmTypePS4))          // drm_type
	writeBE(w, uint32(ContentTypeGD))       // content_type
	writeBE(w, uint32(0x0A000000))          // content_flags
	writeBE(w, uint32(bodyOffset+bodySize)) // promote_size
	seekTo(w, 0x80)
	writeBE(w, uint32(0x20171106)) // version_date (SDK 5.05, matches orbis-pub-cmd)
	writeBE(w, uint32(0x01889410)) // version_hash
	writeBE(w, uint32(0))          // unk_0x88
	writeBE(w, uint32(0))          // unk_0x8C
	writeBE(w, uint32(0))          // unk_0x90
	writeBE(w, uint32(0))          // unk_0x94
	writeBE(w, uint32(0))          // iro_tag
	writeBE(w, uint32(1))          // ekc_version

	// PFS image info
	seekTo(w, 0x400)
	writeBE(w, uint32(1))                  // unk_0x400
	writeBE(w, uint32(1))                  // pfs_image_count
	writeBE(w, uint64(0x80000000000003CC)) // pfs_flags (LibOrbisPkg standard; PS4 uses raw EKPFS regardless)
	seekTo(w, 0x410)
	writeBE(w, pfsImageOffset) // pfs_image_offset
	seekTo(w, 0x418)
	writeBE(w, pfsSize) // pfs_image_size
	seekTo(w, 0x420)
	writeBE(w, uint64(0)) // mount_image_offset
	seekTo(w, 0x428)
	writeBE(w, packageSize) // mount_image_size
	seekTo(w, 0x430)
	writeBE(w, packageSize) // package_size
	seekTo(w, 0x438)
	writeBE(w, uint32(0x10000)) // pfs_signed_size
	seekTo(w, 0x43C)
	writeBE(w, uint32(0xE0000)) // pfs_cache_size (matches orbis-pub-cmd output)
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

func buildLicenseDat(contentID string) ([]byte, error) {
	buf := make([]byte, 0x400)
	binary.BigEndian.PutUint32(buf[0x00:], 0x52494600) // "RIF\0"
	binary.BigEndian.PutUint16(buf[0x04:], 1)
	binary.BigEndian.PutUint16(buf[0x06:], 0xFFFF)
	binary.BigEndian.PutUint64(buf[0x08:], 0)
	binary.BigEndian.PutUint64(buf[0x10:], 1364222275)
	binary.BigEndian.PutUint64(buf[0x18:], 0x7FFFFFFFFFFFFFFF)
	copy(buf[0x20:], []byte(contentID))
	binary.BigEndian.PutUint16(buf[0x50:], 0x0200) // Debug_0
	binary.BigEndian.PutUint16(buf[0x52:], DrmTypePS4)
	binary.BigEndian.PutUint16(buf[0x54:], ContentTypeGD)
	binary.BigEndian.PutUint16(buf[0x56:], 3) // Required by ShellCore for GD.
	binary.BigEndian.PutUint32(buf[0x64:], 1)

	paddedID := make([]byte, 48)
	copy(paddedID, []byte(contentID))
	secretSeed := Sha256(paddedID)
	secretIV := secretSeed[:16]
	secret := make([]byte, 144)
	copy(secret[:16], secretSeed[16:])
	secret = AES128CBCEncrypt(secret, rifDebugKey, secretIV)
	copy(buf[0x260:], secretIV)
	copy(buf[0x270:], secret)

	signature, err := RSA2048SignSha256(Sha256(buf[:0x300]), &debugRifKeyset)
	if err != nil {
		return nil, fmt.Errorf("fpkg: sign debug RIF: %w", err)
	}
	copy(buf[0x300:], signature)
	return buf, nil
}

func buildLicenseInfo(contentID string) []byte {
	buf := make([]byte, 0x200)
	copy(buf[0x00:], []byte(contentID))
	binary.BigEndian.PutUint32(buf[0x40:], 0)
	binary.BigEndian.PutUint32(buf[0x44:], ContentTypeGD)
	binary.BigEndian.PutUint32(buf[0x48:], 0)
	binary.BigEndian.PutUint32(buf[0x4C:], 1)
	return buf
}

func buildPubToolInfo(packageSize uint64) string {
	img0SizeMiB := (packageSize + 0xFFFFF) / (1024 * 1024)
	return fmt.Sprintf(
		"c_date=%s,sdk_ver=05050000,st_type=digital50,img0_l0_size=%d,img0_l1_size=0,img0_sc_ksize=512,img0_pc_ksize=576",
		time.Now().UTC().Format("20060102"),
		img0SizeMiB,
	)
}

func defaultPlayGoManifest() []byte {
	manifest := "<?xml version=\"1.0\" encoding=\"utf-8\" standalone=\"yes\"?>\r\n" +
		"<psproject fmt=\"playgo-manifest\" version=\"0990\">\r\n" +
		"  <volume>\r\n" +
		"    <chunk_info chunk_count=\"1\" scenario_count=\"1\">\r\n" +
		"      <scenarios default_id=\"0\">\r\n" +
		"        <scenario id=\"0\" type=\"sp\" initial_chunk_count=\"1\" label=\"Scenario #0\">0</scenario>\r\n" +
		"      </scenarios>\r\n" +
		"    </chunk_info>\r\n" +
		"  </volume>\r\n" +
		"</psproject>\r\n"
	return append([]byte{0xEF, 0xBB, 0xBF}, []byte(manifest)...)
}

func buildPlayGoChunkDat(contentID string, packageSize, innerPFSSize uint64) []byte {
	buf := make([]byte, 416)
	binary.LittleEndian.PutUint32(buf[0x00:], 0x6f676c70) // "plgo"
	binary.LittleEndian.PutUint16(buf[0x08:], 1)          // image_count
	binary.LittleEndian.PutUint16(buf[0x0A:], 1)          // chunk_count
	binary.LittleEndian.PutUint16(buf[0x0C:], 1)          // mchunk_count
	binary.LittleEndian.PutUint16(buf[0x0E:], 1)          // scenario_count
	binary.LittleEndian.PutUint32(buf[0x10:], uint32(len(buf)))
	binary.LittleEndian.PutUint16(buf[0x16:], 1) // attrib
	for i := 0x20; i < 0x40; i++ {
		buf[i] = 0xFF
	}
	copy(buf[0x40:], []byte(contentID))

	table := []struct {
		offset uint32
		size   uint32
	}{
		{0x100, 0x20},
		{0x120, 0x02},
		{0x130, 0x09},
		{0x140, 0x10},
		{0x160, 0x20},
		{0x180, 0x02},
		{0x190, 0x0C},
		{0x150, 0x10},
	}
	for i, item := range table {
		base := 0xC0 + i*8
		binary.LittleEndian.PutUint32(buf[base:], item.offset)
		binary.LittleEndian.PutUint32(buf[base+4:], item.size)
	}

	buf[0x100] = 0x80
	buf[0x102] = 0x03
	binary.LittleEndian.PutUint16(buf[0x10E:], 1)
	binary.LittleEndian.PutUint64(buf[0x110:], 0xFFFFFFFFFFFFFFFF)
	copy(buf[0x130:], []byte("Chunk #0"))

	binary.LittleEndian.PutUint64(buf[0x140:], 0)
	binary.LittleEndian.PutUint64(buf[0x148:], packageSize)
	binary.LittleEndian.PutUint64(buf[0x150:], 0)
	binary.LittleEndian.PutUint64(buf[0x158:], innerPFSSize)

	buf[0x160] = 1
	binary.LittleEndian.PutUint16(buf[0x174:], 1)
	binary.LittleEndian.PutUint16(buf[0x176:], 1)
	copy(buf[0x190:], []byte("Scenario #0"))
	return buf
}

func updatePlayGoChunkShaSize(entries []*pkgEntry, bodyOffset, pfsSize uint64) {
	chunkSha := findEntry(entries, EntryIDPlayGoChunkSha)
	if chunkSha == nil {
		return
	}

	size := uint32(0)
	for {
		chunkSha.data = make([]byte, size)
		packageSize := estimatePackageSizeFromEntries(entries, bodyOffset, pfsSize)
		nextSize := uint32(packageSize/0x10000) * 4
		if nextSize == size {
			return
		}
		size = nextSize
	}
}

func estimatePackageSizeFromEntries(entries []*pkgEntry, bodyOffset, pfsSize uint64) uint64 {
	dataOffset := bodyOffset
	for _, e := range entries {
		size := entryDataSizeForLayout(e, len(entries))
		dataOffset += align64(uint64(size), 16)
	}
	bodySize := align64(dataOffset, 0x80000) - bodyOffset
	return bodyOffset + bodySize + pfsSize
}

func entryDataSizeForLayout(e *pkgEntry, entryCount int) uint32 {
	switch e.id {
	case EntryIDMetas, EntryIDDigests:
		return uint32(entryCount) * 32
	default:
		return uint32(len(e.data))
	}
}

func writePlayGoChunkSha(pkg []byte, entries []*pkgEntry, pfsImageOffset, packageSize uint64) error {
	chunkSha := findEntry(entries, EntryIDPlayGoChunkSha)
	if chunkSha == nil {
		return nil
	}

	data := make([]byte, chunkSha.dataSize)
	startChunk := pfsImageOffset / 0x10000
	totalChunks := packageSize / 0x10000
	for chunk := startChunk; chunk < totalChunks; chunk++ {
		offset := chunk * 0x10000
		end := offset + 0x10000
		if end > uint64(len(pkg)) {
			return fmt.Errorf("fpkg: PlayGo chunk %d exceeds package size", chunk)
		}
		outOffset := chunk * 4
		if outOffset+4 > uint64(len(data)) {
			return fmt.Errorf("fpkg: PlayGo SHA table too small for chunk %d", chunk)
		}
		hash := Sha256(pkg[offset:end])
		copy(data[outOffset:outOffset+4], hash[:4])
	}

	chunkSha.data = data
	copy(pkg[chunkSha.dataOffset:chunkSha.dataOffset+chunkSha.dataSize], data)
	return nil
}

// ---------------------------------------------------------------------------
// Entry encryption
// ---------------------------------------------------------------------------

func writeEncryptedEntry(w io.Writer, e *pkgEntry, contentID, passcode string) error {
	if len(e.data)%16 != 0 {
		return fmt.Errorf("fpkg: encrypted entry %q size %d is not 16-byte aligned", e.name, len(e.data))
	}

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

	encrypted := AES128CBCEncrypt(e.data, key, iv)
	w.Write(encrypted)
	return nil
}

// ---------------------------------------------------------------------------
// Digest calculation
// ---------------------------------------------------------------------------

func calculateDigests(pkg []byte, entries []*pkgEntry, contentID string, sfo *ParamSfo, bodyOffset, bodySize, pfsImageOffset, pfsSize uint64) {
	// PFS image digests
	pfsSignedDigest := Sha256(pkg[pfsImageOffset : pfsImageOffset+0x10000])
	copy(pkg[0x460:], pfsSignedDigest)
	pfsImageDigest := Sha256(pkg[pfsImageOffset : pfsImageOffset+pfsSize])
	copy(pkg[0x440:], pfsImageDigest)

	sortedEntries := sortedEntriesByID(entries)

	if generalEntry := findEntry(entries, EntryIDGeneralDigests); generalEntry != nil {
		paramEntry := findEntry(entries, EntryIDParamSfo)
		if paramEntry != nil {
			paramData := pkg[paramEntry.dataOffset : paramEntry.dataOffset+paramEntry.dataSize]
			generalData := buildGeneralDigests(pkg, contentID, sfo, paramData, pfsImageDigest)
			copy(pkg[generalEntry.dataOffset:generalEntry.dataOffset+generalEntry.dataSize], generalData)
			generalEntry.data = generalData
		}
	}

	// Entry digest table. Entry 0 is the digest table itself and remains zero;
	// all other slots follow the sorted METAS order.
	digestEntry := findEntry(entries, EntryIDDigests)
	if digestEntry != nil {
		digestData := make([]byte, len(sortedEntries)*pkgHashSize)
		for i := 1; i < len(sortedEntries); i++ {
			e := sortedEntries[i]
			hash := Sha256(pkg[e.dataOffset : e.dataOffset+e.dataSize])
			copy(digestData[i*pkgHashSize:(i+1)*pkgHashSize], hash)
		}
		copy(pkg[digestEntry.dataOffset:digestEntry.dataOffset+digestEntry.dataSize], digestData)
		digestEntry.data = digestData
		copy(pkg[0x140:], Sha256(digestData))
	}

	// Body digest
	bodyDigest := Sha256(pkg[bodyOffset : bodyOffset+bodySize])
	copy(pkg[0x160:], bodyDigest)

	if scEntries1 := concatEntryData(pkg, entries, []uint32{
		EntryIDEntryKeys,
		EntryIDImageKey,
		EntryIDGeneralDigests,
		EntryIDMetas,
		EntryIDDigests,
	}, false); len(scEntries1) > 0 {
		copy(pkg[0x100:], Sha256(scEntries1))
	}

	if scEntries2 := concatEntryData(pkg, entries, []uint32{
		EntryIDEntryKeys,
		EntryIDImageKey,
		EntryIDGeneralDigests,
		EntryIDMetas,
	}, true); len(scEntries2) > 0 {
		copy(pkg[0x120:], Sha256(scEntries2))
	}
}

const (
	generalDigestContent    uint32 = 1 << 1
	generalDigestGame       uint32 = 1 << 2
	generalDigestHeader     uint32 = 1 << 3
	generalDigestMajorParam uint32 = 1 << 5
	generalDigestParam      uint32 = 1 << 6
)

func buildGeneralDigests(pkg []byte, contentID string, sfo *ParamSfo, paramData, pfsImageDigest []byte) []byte {
	data := make([]byte, 0x180)
	binary.BigEndian.PutUint16(data[0x00:], 0xD256)
	binary.BigEndian.PutUint16(data[0x02:], 0x0100)
	binary.BigEndian.PutUint32(data[0x1C:], generalDigestContent|generalDigestGame|generalDigestHeader|generalDigestMajorParam|generalDigestParam)

	majorParamDigest := computeMajorParamDigest(sfo)
	copy(data[0x20:], computeContentDigest(contentID, pfsImageDigest, majorParamDigest))
	copy(data[0x40:], pfsImageDigest)
	copy(data[0x60:], computeHeaderDigest(pkg))
	copy(data[0xA0:], majorParamDigest)
	copy(data[0xC0:], Sha256(paramData))
	return data
}

func computeContentDigest(contentID string, pfsImageDigest, majorParamDigest []byte) []byte {
	buf := make([]byte, 0, 36+12+4+4+32+32)
	buf = append(buf, []byte(contentID)...)
	buf = append(buf, make([]byte, 12)...)
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], DrmTypePS4)
	buf = append(buf, tmp[:]...)
	binary.BigEndian.PutUint32(tmp[:], ContentTypeGD)
	buf = append(buf, tmp[:]...)
	buf = append(buf, pfsImageDigest...)
	buf = append(buf, majorParamDigest...)
	return Sha256(buf)
}

func computeHeaderDigest(pkg []byte) []byte {
	buf := make([]byte, 0, 64+128)
	buf = append(buf, pkg[0:64]...)
	buf = append(buf, pkg[0x400:0x480]...)
	return Sha256(buf)
}

func computeMajorParamDigest(sfo *ParamSfo) []byte {
	majorParam := "ATTRIBUTE" + sfo.digestString("ATTRIBUTE")
	if sfo.Get("ATTRIBUTE2") != nil {
		majorParam += "ATTRIBUTE2" + sfo.digestString("ATTRIBUTE2")
	}
	majorParam += "CATEGORY" + sfo.digestString("CATEGORY")
	majorParam += "FORMAT" + sfo.digestString("FORMAT")
	majorParam += "PUBTOOLVER" + sfo.digestString("PUBTOOLVER")
	return Sha256([]byte(majorParam))
}

func padTo(data []byte, size int) []byte {
	if len(data) >= size {
		return data[:size]
	}
	result := make([]byte, size)
	copy(result, data)
	return result
}

func align64(v, align uint64) uint64 {
	rem := v % align
	if rem != 0 {
		v += align - rem
	}
	return v
}

// isInnerPFSSysExclusion returns true for sce_sys files that are emitted as
// PKG body entries and therefore excluded from the inner PFS.
// The PS4 installer copies PKG body entries to /app0/sce_sys/ at install time.
// Files like keystone and about/right.sprx stay because they have no PKG body entry.
func isInnerPFSSysExclusion(path string) bool {
	if len(path) < 8 || path[:8] != "sce_sys/" {
		return false
	}
	name := path[8:]
	switch name {
	case "param.sfo", "icon0.png", "pic1.png", "save_data.png":
		return true
	}
	return false
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

func sortedEntriesByID(entries []*pkgEntry) []*pkgEntry {
	sortedEntries := append([]*pkgEntry(nil), entries...)
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].id < sortedEntries[j].id
	})
	return sortedEntries
}

func findEntry(entries []*pkgEntry, id uint32) *pkgEntry {
	for _, e := range entries {
		if e.id == id {
			return e
		}
	}
	return nil
}

func calcMainEntryDataSize(entries []*pkgEntry) uint32 {
	var size uint32
	for _, id := range []uint32{
		EntryIDEntryKeys,
		EntryIDImageKey,
		EntryIDGeneralDigests,
		EntryIDMetas,
		EntryIDDigests,
	} {
		if e := findEntry(entries, id); e != nil {
			size += e.dataSize
		}
	}
	return size
}

func concatEntryData(pkg []byte, entries []*pkgEntry, ids []uint32, sc2MetasSize bool) []byte {
	var out []byte
	for _, id := range ids {
		e := findEntry(entries, id)
		if e == nil {
			return nil
		}
		size := e.dataSize
		if sc2MetasSize && id == EntryIDMetas {
			size = 6 * 0x20
		}
		out = append(out, pkg[e.dataOffset:e.dataOffset+size]...)
	}
	return out
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
