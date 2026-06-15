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

	// Emulator selects the PS2 emulator variant.
	Emulator PS2EmulatorType

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
func GeneratePS2EmuConfig(opts PS2FPKGOptions) string {
	var lines []string

	// Basic config
	lines = append(lines, fmt.Sprintf("--max-disc-num=%d", len(opts.ISOPaths)))

	// Uprender
	switch strings.ToLower(opts.Uprender) {
	case "2x2":
		lines = append(lines, "--uprender=2x2")
	case "4x":
		lines = append(lines, "--uprender=4x")
	default:
		// no uprender
	}

	// Display mode
	switch strings.ToLower(opts.DisplayMode) {
	case "4:3":
		lines = append(lines, "--display-mode=4:3")
	case "16:9":
		lines = append(lines, "--display-mode=16:9")
	default:
		// auto
	}

	return strings.Join(lines, "\n") + "\n"
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

// BuildPS2Project creates the file map for a PS2 fPKG.
func BuildPS2Project(opts PS2FPKGOptions) (map[string][]byte, *DiscInfo, error) {
	if len(opts.ISOPaths) == 0 {
		return nil, nil, fmt.Errorf("no ISO files provided")
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
			return nil, nil, cueErr
		}
		discInfo, err = ParsePS2DiscFromCUE(cuePath)
	default:
		return nil, nil, fmt.Errorf("unsupported file extension: %s (expected .iso, .cue, or .bin)", ext)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("parse ps2 disc: %w", err)
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

	// 3. Build file map
	files := make(map[string][]byte)

	// param.sfo
	sfo := NewPS2ParamSfo(title, titleID, contentID)
	files["sce_sys/param.sfo"] = sfo.Serialize()

	// Icon and background
	if opts.Icon0 != "" {
		data, err := os.ReadFile(opts.Icon0)
		if err == nil {
			files["sce_sys/icon0.png"] = data
		}
	}
	if opts.Pic1 != "" {
		data, err := os.ReadFile(opts.Pic1)
		if err == nil {
			files["sce_sys/pic1.png"] = data
		}
	}

	// Memory card
	if opts.MemoryCardPath != "" {
		data, err := os.ReadFile(opts.MemoryCardPath)
		if err == nil {
			files["formatted.card"] = data
		}
	} else {
		files["formatted.card"] = GenerateBlankMemoryCard()
	}

	// Emulator files (eboot.bin, libc.prx, ps2-emu-compiler.self, .crack)
	emuSets := DefaultPS2EmulatorSets()
	emuSet, ok := emuSets[opts.Emulator]
	if !ok {
		emuSet = emuSets[EmuJakV2]
	}

	emuDir, loadErr := ResolveEmulatorsDir(opts.EmulatorFilesDir, nil)
	if loadErr != nil {
		return nil, nil, fmt.Errorf("resolve emulator files: %w", loadErr)
	}

	if opts.OnProgress != nil {
		opts.OnProgress(15, "Loading emulator files")
	}
	emuFiles, loadErr := LoadEmulatorFiles(emuDir, emuSet)
	if loadErr != nil {
		return nil, nil, fmt.Errorf("load ps2 emulator files: %w", loadErr)
	}
	for k, v := range emuFiles {
		files[k] = v
	}

	// Lua include files
	cacheDir, _ := AssetsCacheDir()
	for k, v := range GetLuaIncludeData(cacheDir) {
		files[k] = v
	}

	// Emulator config
	if opts.ConfigTXT != "" {
		files["config-emu-ps4.txt"] = []byte(opts.ConfigTXT)
	} else {
		files["config-emu-ps4.txt"] = []byte(GeneratePS2EmuConfig(opts))
	}

	// Lua config
	if opts.ConfigLUA != "" {
		normalizedID := strings.ReplaceAll(titleID, "-", "_")
		files[fmt.Sprintf("feature_data/%s_features.lua", normalizedID)] = []byte(opts.ConfigLUA)
	}

	// Widescreen patch
	if opts.WidescreenPatch != "" {
		normalizedID := strings.ReplaceAll(titleID, "-", "_")
		files[fmt.Sprintf("patches/%s_cli.conf", normalizedID)] = []byte(opts.WidescreenPatch)
	}

	// Disc images
	tmpDir := filepath.Join(os.TempDir(), "pkg-forge-ps2")
	for i, isoPath := range opts.ISOPaths {
		discNum := i + 1
		if opts.OnProgress != nil {
			opts.OnProgress(30+float64(i)*5, fmt.Sprintf("Reading Disc %d image", discNum))
		}
		discData, needsLIMG, sectorSize, err := loadPS2DiscImage(isoPath, discNum, tmpDir)
		if err != nil {
			return nil, nil, fmt.Errorf("read disc %d (%s): %w", discNum, isoPath, err)
		}

		discName := fmt.Sprintf("image/disc%02d.iso", discNum)

		if needsLIMG {
			// Calculate total sectors and prepend LIMG
			totalSectors := uint32(len(discData) / sectorSize)
			limg := GenerateLIMG(totalSectors)
			combined := append(limg, discData...)
			files[discName] = combined
		} else {
			files[discName] = discData
		}
	}

	// Disc swap config for multi-disc
	if len(opts.ISOPaths) > 1 {
		files["disc-swap-cli.conf"] = []byte(GenerateDiscSwapConfig(len(opts.ISOPaths)))
	}

	if opts.OnProgress != nil {
		opts.OnProgress(60, "Project files ready")
	}
	return files, discInfo, nil
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
	files, discInfo, err := BuildPS2Project(opts)
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
		Files:     files,
		Title:     title,
		TitleID:   normalizeTitleID(titleID),
		ContentID: contentID,
		OnProgress: func(percent float64, phase string) {
			if opts.OnProgress != nil {
				opts.OnProgress(65+percent*0.3, phase)
			}
		},
	}

	pkgData, err := BuildFPKG(pkgOpts)
	if err != nil {
		return fmt.Errorf("build fpkg: %w", err)
	}

	// Write output file
	if opts.OnProgress != nil {
		opts.OnProgress(97, "Writing package")
	}
	if err := os.WriteFile(opts.OutputPath, pkgData, 0644); err != nil {
		return fmt.Errorf("write pkg: %w", err)
	}
	if opts.OnProgress != nil {
		opts.OnProgress(100, "Complete")
	}

	return nil
}
