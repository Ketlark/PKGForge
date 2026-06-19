package fpkg

// This file implements PS2-specific fPKG creation logic.
//
// PS2 fPKG structure (inside the PFS image):
//
//	eboot.bin                — PS2 emulator (Jak v2 or Rogue v1)
//	libc.prx                 — C library module
//	ps2-emu-compiler.self    — PS2 JIT compiler
//	PS20220WD20050620.crack   — PS2 BIOS
//	formatted.card           — 8MB virtual memory card
//	image/
//	  disc01.iso             — PS2 disc image (ISO 9660, 2048 bps)
//	  disc02.iso             — (optional, multi-disc, up to 5)
//	  ...
//	config-emu-ps4.txt       — emulator configuration
//	feature_data/
//	  <GAMEID>_features.lua  — per-game Lua feature script
//	lua_include/
//	  ee-cpr0-alias.lua       — register alias definitions
//	  ee-gpr-alias.lua
//	  ee-hwaddr.lua
//	  language.lua
//	  pad-and-key.lua
//	  MipsInsn.lua
//	  PadStick.lua
//	  sprite.lua
//	  utils.lua
//	sce_sys/
//	  icon0.png              — 512x512 icon
//	  pic1.png               — 1920x1080 background
//	  param.sfo              — game metadata
//	  keystone               — fake keystone
//
// For CD-based PS2 games (2352 bps), the disc image needs an LIMG sector prepended
// to indicate the sector size. For DVD-based games (2048 bps), the ISO is used as-is.

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// PS2 fPKG options
// ---------------------------------------------------------------------------

// PS2EmulatorType represents the PS2 emulator variant to use.
type PS2EmulatorType string

const (
	// EmuJakV2 is the most compatible emulator (from Jak & Daxter games).
	EmuJakV2 PS2EmulatorType = "jakv2"

	// EmuRogue is the Rogue Galaxy emulator (good for VU accuracy).
	EmuRogue PS2EmulatorType = "rogue"

	// EmuSiren is the Forbidden Siren PS2 Classics emulator.
	EmuSiren PS2EmulatorType = "siren"
)

// PS2FPKGOptions contains all parameters for creating a PS2 fPKG.
type PS2FPKGOptions struct {
	// ISOPaths are the paths to .iso files (required, at least 1).
	// Up to 5 discs supported.
	ISOPaths []string

	// OutputPath is where to write the .pkg file (required).
	OutputPath string

	// Title is the game title. Auto-detected from disc if empty.
	Title string

	// TitleID is the game ID (e.g. "SLUS-20062"). Auto-detected if empty.
	TitleID string

	// Icon0 is the path to a 512x512 PNG icon.
	Icon0 string

	// Pic1 is the path to a 1920x1080 PNG background.
	Pic1 string

	// Emulator selects the PS2 emulator launcher (eboot, modules).
	Emulator PS2EmulatorType

	// EmuCore optionally overlays PS2 core files (compiler, BIOS, discmap) from another emu set.
	EmuCore PS2EmulatorType

	// ConfigTXT is an optional emulator config (config-emu-ps4.txt content).
	ConfigTXT string

	// ConfigLUA is an optional per-game Lua script content.
	ConfigLUA string

	// EmulatorFilesDir is the directory containing emulator binaries.
	EmulatorFilesDir string

	// MemoryCardPath is an optional path to a .ps2/.vm2/.bin memory card file.
	MemoryCardPath string

	// WidescreenPatch is optional widescreen Lua patch content.
	WidescreenPatch string

	// DiscSwap indicates multi-disc setup.
	DiscSwap bool

	// Uprender selects uprender mode: "off", "2x2", "4x"
	Uprender string

	// DisplayMode: "4:3", "16:9", "auto"
	DisplayMode string

	OnProgress func(percent float64, phase string)
}

// ---------------------------------------------------------------------------
// PS2 LIMG sector generation
// ---------------------------------------------------------------------------

// GenerateLIMG creates an LIMG sector header for CD-based PS2 images.
// The LIMG sector is prepended to the ISO to tell the emulator about sector sizes.
//
// LIMG structure (16 KB):
//   - Bytes 0-3: "LIMG" magic
//   - Bytes 4-7: version (2)
//   - Bytes 8-11: sector type flags (0xFFFFFFFF = standard)
//   - Bytes 12-15: total sectors in the image
//   - Remaining: zero-padded to 16 KB (8 sectors of 2048)
func GenerateLIMG(totalSectors uint32) []byte {
	// LIMG is padded to 16384 bytes (8 sectors × 2048)
	limg := make([]byte, 16384)
	copy(limg[0:4], "LIMG")
	binary.LittleEndian.PutUint32(limg[4:8], 2)           // version
	binary.LittleEndian.PutUint32(limg[8:12], 0xFFFFFFFF) // flags
	binary.LittleEndian.PutUint32(limg[12:16], totalSectors)
	return limg
}

// ---------------------------------------------------------------------------
// PS2 memory card generation
// ---------------------------------------------------------------------------

// GenerateBlankMemoryCard creates an empty 8MB PS2 memory card image.
// Format: raw MCS format (8,388,608 bytes = 8192 clusters × 1024 bytes).
func GenerateBlankMemoryCard() []byte {
	// PS2 memory card: 8MB = 8,388,608 bytes
	card := make([]byte, 8*1024*1024)

	// Superblock at offset 0
	// Magic: 0x2B (MC) + version info
	card[0x00] = 0x2B
	copy(card[0x01:0x04], "MC")
	binary.LittleEndian.PutUint16(card[0x04:0x06], 1)        // version
	binary.LittleEndian.PutUint16(card[0x06:0x08], 0)        // flags
	binary.LittleEndian.PutUint32(card[0x08:0x0C], 8192)     // total clusters
	binary.LittleEndian.PutUint32(card[0x0C:0x10], 0x400000) // total bytes
	binary.LittleEndian.PutUint32(card[0x10:0x14], 1024)     // cluster size
	binary.LittleEndian.PutUint32(card[0x14:0x18], 8)        // spare size (ECC)

	// FAT starts at cluster 0 (offset 0x2000 typically)
	// Mark cluster 0 as free (already 0)

	return card
}

// ---------------------------------------------------------------------------
// PS2 content ID generation
// ---------------------------------------------------------------------------

// PS2ContentID generates the content ID for a PS2 fPKG.
// Format: UP9000-<TITLEID>_00-<GAMEID>0000<N>
// e.g. UP9000-SLUS20062_00-SLUS200620000001
func PS2ContentID(gameID string) string {
	// Normalize: SLUS-20062/SLUS_200.62 → SLUS20062.
	normalized := normalizeTitleID(gameID)
	// Build the suffix: GAMEID + "0000001"
	suffix := normalized + "0000001"
	return fmt.Sprintf("UP9000-%s_00-%s", normalized, suffix)
}

// ---------------------------------------------------------------------------
// PS2 emulator config generation
// ---------------------------------------------------------------------------

// GeneratePS2EmuConfig generates the config-emu-ps4.txt content for a PS2 fPKG.
// Deprecated: use PS2EmuConfig with an explicit title ID.
func GeneratePS2EmuConfig(opts PS2FPKGOptions) string {
	titleID := opts.TitleID
	if titleID == "" {
		titleID = "SLUS-00000"
	}
	return PS2EmuConfig(opts, titleID, len(opts.ISOPaths))
}

// ---------------------------------------------------------------------------
// PS2 disc swap config
// ---------------------------------------------------------------------------

// GenerateDiscSwapConfig generates the disc-swap-cli.conf content for multi-disc games.
func GenerateDiscSwapConfig(numDiscs int) string {
	var lines []string
	for i := 1; i <= numDiscs; i++ {
		lines = append(lines, fmt.Sprintf("disc%02d.iso", i))
	}
	return strings.Join(lines, "\n") + "\n"
}

// ---------------------------------------------------------------------------
// PS2 project builder
// ---------------------------------------------------------------------------

// BuildPS2Project creates the virtual file tree for a PS2 fPKG.
func BuildPS2Project(opts PS2FPKGOptions) (VirtualFS, *DiscInfo, error) {
	if len(opts.ISOPaths) == 0 {
		return VirtualFS{}, nil, fmt.Errorf("no ISO files provided")
	}

	// 1. Parse the main disc
	if opts.OnProgress != nil {
		opts.OnProgress(5, "Parsing Disc 1")
	}
	mainISO := opts.ISOPaths[0]
	ext := strings.ToLower(filepath.Ext(mainISO))

	var discInfo *DiscInfo
	var err error

	switch ext {
	case ".iso":
		discInfo, err = ParsePS2Disc(mainISO)
	case ".cue":
		discInfo, err = ParsePS2DiscFromCUE(mainISO)
	case ".bin":
		cuePath, cueErr := companionCuePath(mainISO)
		if cueErr != nil {
			return VirtualFS{}, nil, cueErr
		}
		discInfo, err = ParsePS2DiscFromCUE(cuePath)
	default:
		return VirtualFS{}, nil, fmt.Errorf("unsupported file extension: %s (expected .iso, .cue, or .bin)", ext)
	}

	if err != nil {
		return VirtualFS{}, nil, fmt.Errorf("parse ps2 disc: %w", err)
	}

	// 2. Resolve title and ID
	titleID := opts.TitleID
	if titleID == "" {
		titleID = discInfo.GameID
	}
	if titleID == "" {
		titleID = "SLUS-00000"
	}

	title := opts.Title
	if title == "" {
		title = discInfo.Title
	}
	if title == "" {
		title = titleID
	}

	contentID := PS2ContentID(titleID)
	opts = applyPS2GameDefaults(opts, ps2PatchID(titleID))

	// 3. Build the virtual file tree
	project := NewVirtualFS()

	// param.sfo
	sfo := NewPS2ParamSfo(title, titleID, contentID)
	project.PutData("sce_sys/param.sfo", sfo.Serialize())

	// Icon and background
	iconPath := opts.Icon0
	if iconPath == "" {
		if coverPath, err := ResolvePS2Cover(mainISO, titleID); err == nil {
			iconPath = coverPath
		}
	}
	if iconPath != "" {
		if data, err := os.ReadFile(iconPath); err == nil {
			project.PutData("sce_sys/icon0.png", data)
		}
	}
	if opts.Pic1 != "" {
		if data, err := os.ReadFile(opts.Pic1); err == nil {
			project.PutData("sce_sys/pic1.png", data)
		}
	} else if backgroundPath, err := ResolvePS2Background(mainISO, titleID); err == nil {
		if data, err := os.ReadFile(backgroundPath); err == nil {
			project.PutData("sce_sys/pic1.png", data)
		}
	}
	if len(project.Mem["sce_sys/pic1.png"]) == 0 && iconPath != "" {
		if data, err := ps1BackgroundFromImagePath(iconPath, titleID); err == nil {
			project.PutData("sce_sys/pic1.png", data)
		}
	}

	// Emulator files (eboot.bin, libc.prx, ps2-emu-compiler.self, .crack, sce_discmap.plt, …)
	emuSets := DefaultPS2EmulatorSets()
	launchEmu := opts.Emulator
	if launchEmu == "" {
		launchEmu = EmuJakV2
	}
	coreEmu := opts.EmuCore
	if coreEmu == "" {
		coreEmu = launchEmu
	}
	emuSet, ok := emuSets[launchEmu]
	if !ok {
		emuSet = emuSets[EmuJakV2]
		launchEmu = EmuJakV2
	}

	emuDir, loadErr := ResolveEmulatorsDir(opts.EmulatorFilesDir, nil)
	if loadErr != nil {
		return VirtualFS{}, nil, fmt.Errorf("resolve emulator files: %w", loadErr)
	}

	if opts.OnProgress != nil {
		opts.OnProgress(15, "Loading emulator files")
	}
	emuFiles, loadErr := LoadEmulatorFiles(emuDir, emuSet)
	if loadErr != nil {
		return VirtualFS{}, nil, fmt.Errorf("load ps2 emulator files: %w", loadErr)
	}
	if coreEmu != launchEmu {
		if err := overlayPS2EmulatorCore(emuFiles, emuDir, emuSets, coreEmu); err != nil {
			return VirtualFS{}, nil, fmt.Errorf("load ps2 emulator core (%s): %w", coreEmu, err)
		}
	}
	for k, v := range emuFiles {
		project.PutData(k, v)
	}
	patchID := ps2PatchID(titleID)
	retailLauncher := ps2ProfileUsesRetailLauncher(titleID)
	if allEmuFiles, err := LoadEmulatorDirectoryFiles(emuDir, emuSet); err == nil {
		for k, v := range allEmuFiles {
			if shouldSkipPS2EmulatorSidecar(k, retailLauncher) {
				continue
			}
			if _, exists := project.Mem[k]; !exists {
				project.PutData(k, v)
			}
		}
	}
	ensurePS2LauncherSidecars(project, emuDir, emuSet, launchEmu)
	if len(project.Mem["package-ps4.conf"]) == 0 {
		project.PutData("package-ps4.conf", []byte(PS2CompatPackageConf()))
	}

	// Memory card: prefer user file, then emulator dump (retail), then blank.
	if len(project.Mem["formatted.card"]) == 0 {
		if opts.MemoryCardPath != "" {
			data, err := os.ReadFile(opts.MemoryCardPath)
			if err == nil {
				project.PutData("formatted.card", data)
			}
		}
	}
	if len(project.Mem["formatted.card"]) == 0 && !retailLauncher {
		project.PutData("formatted.card", GenerateBlankMemoryCard())
	}

	// Lua include files — fill gaps only; per-emulator lua_include wins when present.
	cacheDir, _ := AssetsCacheDir()
	for k, v := range GetLuaIncludeData(cacheDir) {
		if _, exists := project.Mem[k]; !exists {
			project.PutData(k, v)
		}
	}

	// Emulator config
	if opts.ConfigTXT != "" {
		project.PutData("config-emu-ps4.txt", []byte(opts.ConfigTXT))
	} else {
		template := string(project.Mem["config-emu-ps4.txt"])
		project.PutData("config-emu-ps4.txt", []byte(ps2RuntimeEmuConfig(opts, titleID, len(opts.ISOPaths), template)))
	}

	// Lua config (custom or built-in compatibility script).
	if opts.ConfigLUA != "" {
		project.PutData(fmt.Sprintf("feature_data/%s_features.lua", patchID), []byte(opts.ConfigLUA))
	} else if lua, ok := PS2CompatFeatureLUA(patchID); ok {
		project.PutData(fmt.Sprintf("feature_data/%s_features.lua", patchID), []byte(lua))
	}

	// Per-game cli.conf patch (custom or built-in database entry).
	if opts.WidescreenPatch != "" {
		project.PutData(fmt.Sprintf("patches/%s_cli.conf", patchID), []byte(opts.WidescreenPatch))
	} else if cli, ok := PS2CompatCLI(patchID); ok {
		project.PutData(fmt.Sprintf("patches/%s_cli.conf", patchID), []byte(cli))
	}

	// Disc images are staged on disk and streamed into the PFS to avoid
	// loading multi-gigabyte ISOs into memory.
	tmpDir := filepath.Join(os.TempDir(), "pkg-forge-ps2")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return VirtualFS{}, nil, fmt.Errorf("create temp dir: %w", err)
	}
	for i, isoPath := range opts.ISOPaths {
		discNum := i + 1
		if opts.OnProgress != nil {
			opts.OnProgress(30+float64(i)*5, fmt.Sprintf("Staging Disc %d image", discNum))
		}
		stagedPath, err := stagePS2DiscImage(isoPath, discNum, tmpDir)
		if err != nil {
			return VirtualFS{}, nil, fmt.Errorf("stage disc %d (%s): %w", discNum, isoPath, err)
		}
		project.PutFile(fmt.Sprintf("image/disc%02d.iso", discNum), stagedPath)
	}

	// Disc swap config for multi-disc
	if len(opts.ISOPaths) > 1 {
		project.PutData("disc-swap-cli.conf", []byte(GenerateDiscSwapConfig(len(opts.ISOPaths))))
	}

	if opts.OnProgress != nil {
		opts.OnProgress(60, "Project files ready")
	}
	return project, discInfo, nil
}

func ensurePS2LauncherSidecars(project VirtualFS, emuDir string, emuSet *EmulatorSet, launchEmu PS2EmulatorType) {
	if len(project.Mem["sce_sys/shareparam.json"]) > 0 {
		return
	}
	candidates := []string{emuSet.Name}
	if launchEmu == EmuSiren {
		candidates = append(candidates, DefaultPS2EmulatorSets()[EmuJakV2].Name)
	}
	for _, name := range candidates {
		path := filepath.Join(emuDir, name, "sce_sys/shareparam.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		project.PutData("sce_sys/shareparam.json", data)
		return
	}
}

func companionCuePath(binPath string) (string, error) {
	ext := filepath.Ext(binPath)
	if strings.ToLower(ext) != ".bin" {
		return binPath, nil
	}

	base := strings.TrimSuffix(binPath, ext)
	for _, candidate := range []string{base + ".cue", base + ".CUE"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf(".bin file without matching .cue; please provide the .cue file instead")
}

func stagePS2DiscImage(path string, discNum int, tmpDir string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".iso":
		needsLIMG, sectorSize, err := isCDBasedImageFile(path)
		if err != nil {
			return "", err
		}
		if !needsLIMG {
			return path, nil
		}
		return writeLIMGPrependedImage(path, discNum, tmpDir, sectorSize)
	case ".cue":
		tracks, err := ParseCUE(path)
		if err != nil {
			return "", err
		}
		return stageCUEImage(tracks, discNum, tmpDir)
	case ".bin":
		if cuePath, err := companionCuePath(path); err == nil {
			tracks, err := ParseCUE(cuePath)
			if err != nil {
				return "", err
			}
			return stageCUEImage(tracks, discNum, tmpDir)
		}
		return writeLIMGPrependedImage(path, discNum, tmpDir, 2352)
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

func stageCUEImage(tracks []CueTrack, discNum int, tmpDir string) (string, error) {
	binFiles := getUniqueBinFiles(tracks)
	if len(binFiles) == 0 {
		return "", fmt.Errorf("cue has no bin files")
	}

	var imagePath string
	if len(binFiles) == 1 {
		imagePath = binFiles[0]
	} else {
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return "", err
		}
		imagePath = filepath.Join(tmpDir, fmt.Sprintf("disc%02d.bin", discNum))
		if _, err := MergeBins(tracks, imagePath); err != nil {
			return "", err
		}
	}

	return writeLIMGPrependedImage(imagePath, discNum, tmpDir, 2352)
}

func writeLIMGPrependedImage(srcPath string, discNum int, tmpDir string, sectorSize int) (string, error) {
	info, err := os.Stat(srcPath)
	if err != nil {
		return "", err
	}
	if sectorSize <= 0 {
		sectorSize = 2048
	}
	if info.Size()%int64(sectorSize) != 0 {
		return "", fmt.Errorf("disc image size %d is not aligned to sector size %d", info.Size(), sectorSize)
	}

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}
	outPath := filepath.Join(tmpDir, fmt.Sprintf("disc%02d-limg.iso", discNum))
	out, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	totalSectors := uint32(info.Size() / int64(sectorSize))
	if _, err := out.Write(GenerateLIMG(totalSectors)); err != nil {
		return "", err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer src.Close()
	if _, err := io.Copy(out, src); err != nil {
		return "", err
	}
	return outPath, nil
}

func isCDBasedImageFile(path string) (bool, int, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".bin" || ext == ".cue" {
		return true, 2352, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, 2048, err
	}
	if info.Size() >= 700*1024*1024 {
		return false, 2048, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return false, 2048, err
	}
	defer f.Close()

	head := make([]byte, 2048)
	if _, err := f.ReadAt(head, 0); err != nil {
		return false, 2048, err
	}
	return isCDBasedImage(path, head), 2048, nil
}

func loadPS2DiscImage(path string, discNum int, tmpDir string) ([]byte, bool, int, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".iso":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, false, 2048, err
		}
		return data, isCDBasedImage(path, data), 2048, nil
	case ".cue":
		tracks, err := ParseCUE(path)
		if err != nil {
			return nil, false, 2352, err
		}
		data, err := readCUEImage(tracks, discNum, tmpDir)
		return data, true, 2352, err
	case ".bin":
		if cuePath, err := companionCuePath(path); err == nil {
			tracks, err := ParseCUE(cuePath)
			if err != nil {
				return nil, false, 2352, err
			}
			data, err := readCUEImage(tracks, discNum, tmpDir)
			return data, true, 2352, err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, false, 2352, err
		}
		return data, true, 2352, nil
	default:
		return nil, false, 2048, fmt.Errorf("unsupported file extension: %s", ext)
	}
}

func readCUEImage(tracks []CueTrack, discNum int, tmpDir string) ([]byte, error) {
	binFiles := getUniqueBinFiles(tracks)
	if len(binFiles) == 0 {
		return nil, fmt.Errorf("cue has no bin files")
	}
	if len(binFiles) == 1 {
		return os.ReadFile(binFiles[0])
	}

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, err
	}
	mergedPath := filepath.Join(tmpDir, fmt.Sprintf("disc%02d.bin", discNum))
	if _, err := MergeBins(tracks, mergedPath); err != nil {
		return nil, err
	}
	return os.ReadFile(mergedPath)
}

// isCDBasedImage detects if a PS2 image is CD-based (2352 bps) vs DVD (2048 bps).
// CD-based images have a sync pattern at offset 0 (0x00FFFFFF...).
// DVD-based images have an ISO 9660 volume descriptor at sector 16.
func isCDBasedImage(path string, data []byte) bool {
	// Check file extension hint
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".bin" || ext == ".cue" {
		return true // BIN/CUE is typically CD-based
	}

	// Check for CD sync pattern: first 12 bytes are 00 FF FF FF FF FF FF FF FF FF FF 00
	if len(data) >= 12 {
		syncPattern := []byte{0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00}
		if string(data[:12]) == string(syncPattern) {
			return true
		}
	}

	// Check file size: CD-based games are typically under 700MB
	if len(data) < 700*1024*1024 {
		// Could be CD — check more carefully
		// A Mode 2 sector has 0x8000 at offset 0x10 (sub-header)
		if len(data) >= 0x12 {
			if data[0x10] == 0x00 && data[0x11] == 0x00 && data[0x12] == 0x08 && data[0x13] == 0x00 {
				return true
			}
		}
	}

	return false
}

// ---------------------------------------------------------------------------
// PS2 disc image conversion (CD → ISO with LIMG)
// ---------------------------------------------------------------------------

// ConvertBinToISO converts a raw BIN (2352 bps) to ISO (2048 bps) format.
// For Mode 2 tracks: extracts data portion (24 bytes header + 2048 bytes data).
func ConvertBinToISO(binData []byte) []byte {
	sectorSize := 2352
	dataSize := 2048
	numSectors := len(binData) / sectorSize

	iso := make([]byte, numSectors*dataSize)
	for i := 0; i < numSectors; i++ {
		src := i * sectorSize
		dst := i * dataSize
		copy(iso[dst:dst+dataSize], binData[src+24:src+24+dataSize])
	}

	return iso
}

// ---------------------------------------------------------------------------
// PS2 content hash for content ID
// ---------------------------------------------------------------------------

// PS2ContentHash generates a deterministic hash for content ID generation.
func PS2ContentHash(gameID string) string {
	h := sha256.Sum256([]byte("ps2_" + gameID))
	return fmt.Sprintf("%x", h)[:16]
}

// ---------------------------------------------------------------------------
// PS2 main entry point
// ---------------------------------------------------------------------------

// CreatePS2FPKG is the main entry point for PS2 fPKG creation.
// It orchestrates the full pipeline: disc parsing → project setup → PFS → PKG.
func CreatePS2FPKG(opts PS2FPKGOptions) error {
	if opts.OnProgress != nil {
		opts.OnProgress(0, "Preparing PS2 package")
	}
	project, discInfo, err := BuildPS2Project(opts)
	if err != nil {
		return err
	}
	if opts.OnProgress != nil {
		opts.OnProgress(65, "Building PS4 package")
	}

	// Resolve title and ID
	titleID := opts.TitleID
	if titleID == "" {
		titleID = discInfo.GameID
	}
	if titleID == "" {
		titleID = "SLUS-00000"
	}

	title := opts.Title
	if title == "" {
		title = discInfo.Title
	}
	if title == "" {
		title = titleID
	}

	contentID := PS2ContentID(titleID)

	// Build the fPKG using the generic PKG builder
	pkgOpts := PKGOptions{
		Project: project,
		Title:   title,
		TitleID:     normalizeTitleID(titleID),
		ContentID:   contentID,
		OnProgress: func(percent float64, phase string) {
			if opts.OnProgress != nil {
				opts.OnProgress(65+percent*0.3, phase)
			}
		},
	}

	if err := BuildFPKGToFile(opts.OutputPath, pkgOpts); err != nil {
		return fmt.Errorf("build fpkg: %w", err)
	}
	if opts.OnProgress != nil {
		opts.OnProgress(100, "Complete")
	}

	return nil
}
