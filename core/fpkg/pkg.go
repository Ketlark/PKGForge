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
	"os"
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
	ContentID  string // e.g. "UP9000-SCUS94400_00-0000000000000001"
	Passcode   string // 32 chars, default all zeros
	Project    VirtualFS
	Files      map[string][]byte // legacy: in-memory files (use Project instead)
	FileSources map[string]string // legacy: streamed files (use Project.Disk instead)
	Icon0      []byte            // icon0.png data
	Pic0       []byte            // pic0.png data (optional)
	Pic1       []byte            // pic1.png data (optional)
	ParamSfo   []byte            // param.sfo data (optional, generated if nil)
	Title      string            // game title
	TitleID    string            // e.g. "SCUS94400"
	OnProgress ProgressReporter
}

func (opts PKGOptions) project() VirtualFS {
	if len(opts.Project.Mem) > 0 || len(opts.Project.Disk) > 0 {
		return opts.Project
	}
	return VirtualFSFromMaps(opts.Files, opts.FileSources)
}

// BuildFPKG creates a complete PS4 fPKG from the given options.
// Returns the raw PKG bytes. For large packages prefer BuildFPKGToFile.
func BuildFPKG(opts PKGOptions) ([]byte, error) {
	pkgData, outerPath, outerCleanup, err := buildFPKGCore(opts, "")
	if outerCleanup != nil {
		defer outerCleanup()
	}
	if pkgData != nil {
		return pkgData, nil
	}
	data, err := os.ReadFile(outerPath)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// BuildFPKGToFile writes a complete PS4 fPKG directly to outputPath.
func BuildFPKGToFile(outputPath string, opts PKGOptions) error {
	_, path, cleanup, err := buildFPKGCore(opts, outputPath)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}
	if path == outputPath {
		return nil
	}
	return os.Rename(path, outputPath)
}

func buildFPKGCore(opts PKGOptions, outputPath string) (pkgData []byte, writtenPath string, cleanup func(), err error) {
	if opts.Passcode == "" {
		opts.Passcode = string(DefaultPasscode)
	}
	if len(opts.Passcode) != 32 {
		return nil, "", nil, fmt.Errorf("fpkg: passcode must be 32 bytes, got %d", len(opts.Passcode))
	}
	if len(opts.ContentID) != 36 {
		return nil, "", nil, fmt.Errorf("fpkg: content ID must be 36 chars, got %d", len(opts.ContentID))
	}

	packageSFO, err := preparePackageSFO(opts)
	if err != nil {
		return nil, "", nil, err
	}
	packageSFOData := packageSFO.Serialize()

	// Build file map for inner PFS (runtime defaults merged into the project tree).
	project := opts.project()
	runtimeSFOData := project.Mem["sce_sys/param.sfo"]
	if len(runtimeSFOData) == 0 {
		runtimeSFOData = packageSFOData
	}
	runtimeFiles := map[string][]byte{
		"sce_sys/param.sfo": runtimeSFOData,
	}
	if opts.Icon0 != nil && project.Mem["sce_sys/icon0.png"] == nil {
		runtimeFiles["sce_sys/icon0.png"] = opts.Icon0
	}
	if project.Mem["sce_sys/icon0.png"] == nil {
		runtimeFiles["sce_sys/icon0.png"] = defaultIcon0PNG(opts.TitleID)
	}
	if project.Mem["sce_sys/save_data.png"] == nil {
		runtimeFiles["sce_sys/save_data.png"] = defaultSaveDataPNG(opts.TitleID)
	}
	if opts.Pic0 != nil && project.Mem["sce_sys/pic0.png"] == nil {
		runtimeFiles["sce_sys/pic0.png"] = opts.Pic0
	}
	if opts.Pic1 != nil && project.Mem["sce_sys/pic1.png"] == nil {
		runtimeFiles["sce_sys/pic1.png"] = opts.Pic1
	}
	if project.Mem["sce_sys/pic1.png"] == nil {
		runtimeFiles["sce_sys/pic1.png"] = defaultPic1PNG(opts.TitleID)
	}
	runtimeFiles["sce_sys/keystone"] = CreateKeystone(opts.Passcode)
	opts.Files = project.MergeMem(runtimeFiles)

	images, err := BuildPackagedImages(project, runtimeFiles, ImagePipelineOptions{
		ContentID:  opts.ContentID,
		Passcode:   opts.Passcode,
		OnProgress: opts.OnProgress,
	})
	if err != nil {
		return nil, "", nil, err
	}
	if images.OuterPFSCleanup != nil {
		defer images.OuterPFSCleanup()
	}

	outer := outerPFSSource{Data: images.OuterPFS, Path: images.OuterPFSPath}
	if outer.Size == 0 {
		if size, sizeErr := outerPFSSize(outer); sizeErr == nil {
			outer.Size = size
		}
	}

	if opts.OnProgress != nil {
		opts.OnProgress(80, "Assembling package")
	}

	stream, streamErr := shouldStreamPKGAssembly(outer)
	if streamErr != nil {
		return nil, "", nil, streamErr
	}

	if outputPath != "" || stream {
		target := outputPath
		if target == "" {
			tmp, tmpErr := os.CreateTemp("", "pkg-forge-out-*.pkg")
			if tmpErr != nil {
				return nil, "", nil, tmpErr
			}
			target = tmp.Name()
			tmp.Close()
			cleanup = func() { os.Remove(target) }
		}
		if err := assemblePKGToFile(target, opts, packageSFO, packageSFOData, outer, images.EKPFS, uint64(images.InnerLogicalSize)); err != nil {
			return nil, "", nil, fmt.Errorf("fpkg: PKG assembly: %w", err)
		}
		if opts.OnProgress != nil {
			opts.OnProgress(95, "Package assembled")
		}
		return nil, target, cleanup, nil
	}

	pkg, err := assemblePKG(opts, packageSFO, packageSFOData, images.OuterPFS, images.EKPFS, uint64(images.InnerLogicalSize))
	if err != nil {
		return nil, "", nil, fmt.Errorf("fpkg: PKG assembly: %w", err)
	}
	if opts.OnProgress != nil {
		opts.OnProgress(95, "Package assembled")
	}
	return pkg, "", nil, nil
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

// assemblePKG builds the complete PKG binary from components (in-memory, small packages).
func assemblePKG(opts PKGOptions, sfo *ParamSfo, sfoData, outerPFS, ekpfs []byte, innerPFSSize uint64) ([]byte, error) {
	layout, err := buildPKGEntries(opts, sfo, sfoData, ekpfs, uint64(len(outerPFS)), innerPFSSize)
	if err != nil {
		return nil, err
	}

	pkg := make([]byte, layout.packageSize)
	w := newBytesWriteSeeker(pkg)

	writePKGHeader(w, opts, len(layout.entries), layout.mainEntDataSize, layout.bodyOffset, layout.bodySize, layout.pfsImageOffset, layout.pfsSize, layout.packageSize)
	if err := writePKGBodyEntries(w, layout.entries, opts); err != nil {
		return nil, err
	}

	seekTo(w, int64(layout.pfsImageOffset))
	w.Write(outerPFS)

	pfsDigests := digestPFSSlice(outerPFS)
	if err := writePlayGoChunkSha(pkg, layout.entries, layout.pfsImageOffset, layout.packageSize); err != nil {
		return nil, err
	}

	calculateDigests(pkg, layout.entries, opts.ContentID, sfo, layout.bodyOffset, layout.bodySize, layout.pfsImageOffset, layout.pfsSize, pfsDigests)

	writePKGHeader(w, opts, len(layout.entries), layout.mainEntDataSize, layout.bodyOffset, layout.bodySize, layout.pfsImageOffset, layout.pfsSize, layout.packageSize)

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

func calculateDigests(pkg []byte, entries []*pkgEntry, contentID string, sfo *ParamSfo, bodyOffset, bodySize, pfsImageOffset, pfsSize uint64, precomputed pfsDigestResult) {
	var pfsSignedDigest, pfsImageDigest []byte
	if len(precomputed.FullDigest) > 0 {
		pfsSignedDigest = precomputed.SignedDigest
		pfsImageDigest = precomputed.FullDigest
	} else {
		signedLen := int64(0x10000)
		if int64(pfsSize) < signedLen {
			signedLen = int64(pfsSize)
		}
		pfsSignedDigest = Sha256(pkg[pfsImageOffset : pfsImageOffset+uint64(signedLen)])
		pfsImageDigest = Sha256(pkg[pfsImageOffset : pfsImageOffset+pfsSize])
	}
	copy(pkg[0x460:], pfsSignedDigest)
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
