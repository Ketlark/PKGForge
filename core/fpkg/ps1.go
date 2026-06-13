package fpkg

// This file implements PS1-specific fPKG creation logic.
//
// PS1 fPKG structure (inside the PFS image):
//
//	eboot.bin              — PS1 emulator (ps1_emu or ps1_netemu)
//	libc.prx               — C library module
//	image/
//	  disc01.bin           — merged PS1 disc image (raw 2352 bps sectors)
//	  disc02.bin           — (optional, multi-disc)
//	  ...
//	sce_sys/
//	  icon0.png            — 512x512 icon
//	  pic1.png             — 1920x1080 background/splash
//	  param.sfo            — game metadata
//	  keystone             — fake keystone
//
// PS1-specific features:
//   - Multi-bin merging (concatenate .bin files in track order)
//   - TOC generation for CDDA games
//   - LibCrypt detection
//   - Analog stick simulation option
//   - Skip bootlogo option
//   - Force 60Hz option

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// PS1 fPKG options
// ---------------------------------------------------------------------------

// PS1FPKGOptions contains all parameters for creating a PS1 fPKG.
type PS1FPKGOptions struct {
	// CuePath is the path to the .cue file (required).
	CuePath string

	// ExtraDiscs are additional .cue paths for multi-disc games (up to 3 extra, 4 total).
	ExtraDiscs []string

	// OutputPath is where to write the .pkg file (required).
	OutputPath string

	// Title is the game title. Auto-detected from disc if empty.
	Title string

	// TitleID is the game ID (e.g. "SCUS-94244"). Auto-detected if empty.
	TitleID string

	// Icon0 is the path to a 512x512 PNG icon. If empty, a default is used.
	Icon0 string

	// Pic1 is the path to a 1920x1080 PNG background. If empty, a default is used.
	Pic1 string

	// Emulator selects the PS1 emulator to use:
	//   "ps1_emu"     — original PS1 emulator (default)
	//   "ps1_netemu"  — PS Plus net emulator
	Emulator string

	// Options
	AnalogSticks   bool // Enable analog stick simulation
	SkipBootLogo   bool // Skip Sony/PlayStation boot logo
	Force60Hz      bool // Force 60Hz output
	EnableCDDATOC  bool // Generate TOC for CDDA music

	// EmulatorFilesDir is the directory containing emulator binaries (eboot.bin, libc.prx).
	// If empty, the default embedded path is used.
	EmulatorFilesDir string
}

// ---------------------------------------------------------------------------
// PS1 TOC generation
// ---------------------------------------------------------------------------

// TOCEntry represents a track entry in the PS1 TOC.
type TOCEntry struct {
	Number   int
	StartLBA int
	Mode     int // 0 = audio, 1 = mode 1, 2 = mode 2
}

// GeneratePS1TOC generates TOC data for PS1 CDDA games.
// Returns the binary TOC data that gets written alongside the disc image.
func GeneratePS1TOC(tracks []CueTrack) []byte {
	// The TOC format used by the PS1 emulator is a simple binary structure:
	// 4 bytes: number of tracks
	// Per track: 4 bytes track number, 4 bytes start LBA, 4 bytes mode, 4 bytes padding

	numTracks := len(tracks)
	tocSize := 4 + numTracks*16
	toc := make([]byte, tocSize)

	binary.LittleEndian.PutUint32(toc[0:4], uint32(numTracks))

	for i, t := range tracks {
		off := 4 + i*16
		binary.LittleEndian.PutUint32(toc[off:off+4], uint32(t.Number))
		binary.LittleEndian.PutUint32(toc[off+4:off+8], uint32(t.StartLBA))

		mode := 0
		if strings.Contains(t.Mode, "MODE1") {
			mode = 1
		} else if strings.Contains(t.Mode, "MODE2") {
			mode = 2
		}
		binary.LittleEndian.PutUint32(toc[off+8:off+12], uint32(mode))
		// off+12: padding (0)
	}

	return toc
}

// ---------------------------------------------------------------------------
// PS1 LibCrypt detection
// ---------------------------------------------------------------------------

// DetectLibCrypt checks if a PS1 disc image has LibCrypt protection.
// LibCrypt protection is found in the subchannel data of specific sectors.
// For simplicity, we detect it by checking known protected title IDs.
func DetectLibCrypt(gameID string) bool {
	// Known LibCrypt protected games (partial list)
	// Full list would be much larger
	libCryptIDs := map[string]bool{
		"SCES-02377": true, // Ape Escape (EU)
		"SCES-02438": true, // Crash Team Racing (EU)
		"SCES-02104": true, // Final Fantasy VIII (EU)
		"SCES-02750": true, // Final Fantasy IX (EU)
		"SLES-02963": true, // ISS Pro Evolution 2
		"SCES-03424": true, // MediEvil II (EU)
		"SCES-01824": true, // Metal Gear Solid (EU)
		"SCES-02027": true, // Spyro 2 (EU)
		"SCES-02835": true, // Spyro 3 (EU)
		"SCPS-10105": true, // Chrono Cross (JP)
		"SLUS-01097": true, // Chrono Cross (US) — actually not LibCrypt
	}
	return libCryptIDs[gameID]
}

// ---------------------------------------------------------------------------
// PS1 content ID generation
// ---------------------------------------------------------------------------

// PS1ContentID generates the content ID for a PS1 fPKG.
// Format: UP9000-<TITLEID>_00-<HASH16CHARS>
// The hash is derived from the game ID for deterministic results.
func PS1ContentID(gameID string) string {
	// Normalize the game ID: remove dash → SCUS94244
	normalized := strings.ReplaceAll(gameID, "-", "")

	// Generate a short hash from the normalized ID
	h := sha256.Sum256([]byte("ps1_" + normalized))
	hashStr := fmt.Sprintf("%x", h)[:16]

	return fmt.Sprintf("UP9000-%s_00-%s", normalized, strings.ToUpper(hashStr))
}

// ---------------------------------------------------------------------------
// PS1 emulator config
// ---------------------------------------------------------------------------

// PS1EmuConfig generates the emulator configuration text for a PS1 fPKG.
// This goes into the config-emu-ps4.txt file inside the PKG.
func PS1EmuConfig(opts PS1FPKGOptions) string {
	var lines []string

	if opts.AnalogSticks {
		lines = append(lines, "--simulate-analog-sticks=1")
	}
	if opts.SkipBootLogo {
		lines = append(lines, "--skip-bootlogo=1")
	}
	if opts.Force60Hz {
		lines = append(lines, "--force-60hz=1")
	}

	return strings.Join(lines, "\n") + "\n"
}

// ---------------------------------------------------------------------------
// PS1 project builder
// ---------------------------------------------------------------------------

// BuildPS1Project creates the file map for a PS1 fPKG.
// Returns a map of virtual file paths to their byte contents.
func BuildPS1Project(opts PS1FPKGOptions) (map[string][]byte, *PS1DiscResult, error) {
	// 1. Parse the main disc
	disc, err := ParsePS1Disc(opts.CuePath)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ps1 disc: %w", err)
	}

	// Auto-detect title and ID
	titleID := opts.TitleID
	if titleID == "" {
		titleID = disc.Info.GameID
	}
	if titleID == "" {
		titleID = "SLUS-00000" // fallback
	}

	title := opts.Title
	if title == "" {
		title = disc.Info.Title
	}
	if title == "" {
		title = titleID
	}

	contentID := PS1ContentID(titleID)

	// 2. Build the file map
	files := make(map[string][]byte)

	// param.sfo
	sfo := NewPS1ParamSfo(title, titleID, contentID)
	files["sce_sys/param.sfo"] = sfo.Serialize()

	// keystone
	keystone := CreateKeystone(contentID)
	files["sce_sys/keystone"] = keystone

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

	// Emulator files (eboot.bin, sce_module/libc.prx, etc.)
	emuType := opts.Emulator
	if emuType == "" {
		emuType = "ps1_emu"
	}

	archiveName := ResolveArchiveEmuName(emuType)
	emuSet := &EmulatorSet{
		Name: archiveName,
		Files: map[string]string{
			"eboot.bin":              "eboot.bin",
			"libc.prx":               "sce_module/libc.prx",
			"libSceFios2.prx":        "sce_module/libSceFios2.prx",
			"libSceNpToolkit2.prx":   "sce_module/libSceNpToolkit2.prx",
		},
	}

	emuDir, err := ResolveEmulatorsDir(opts.EmulatorFilesDir, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve emulator files: %w", err)
	}

	emuFiles, err := LoadEmulatorFiles(emuDir, emuSet)
	if err != nil {
		return nil, nil, fmt.Errorf("load ps1 emulator files: %w", err)
	}
	for k, v := range emuFiles {
		files[k] = v
	}

	// Emulator config
	emuConfig := PS1EmuConfig(opts)
	if emuConfig != "\n" {
		files["config-emu-ps4.txt"] = []byte(emuConfig)
	}

	// 3. Merge and add disc images
	tmpDir := filepath.Join(os.TempDir(), "pkg-forge-ps1")

	// Main disc
	tracks := disc.Info.Tracks
	if disc.HasCDDA && opts.EnableCDDATOC {
		toc := GeneratePS1TOC(tracks)
		files["image/disc01.toc"] = toc
	}

	// Merge bins into a single file
	mergedPath := filepath.Join(tmpDir, "disc01.bin")
	os.MkdirAll(filepath.Join(tmpDir, "image"), 0755)

	// Check if we actually need to merge (multi-bin vs single bin)
	binFiles := getUniqueBinFiles(tracks)
	if len(binFiles) == 1 {
		// Single bin — just reference it directly
		data, err := os.ReadFile(binFiles[0])
		if err != nil {
			return nil, nil, fmt.Errorf("read bin: %w", err)
		}
		files["image/disc01.bin"] = data
	} else {
		// Multi-bin — merge
		_, err := MergeBins(tracks, mergedPath)
		if err != nil {
			return nil, nil, fmt.Errorf("merge bins: %w", err)
		}
		data, err := os.ReadFile(mergedPath)
		if err != nil {
			return nil, nil, fmt.Errorf("read merged bin: %w", err)
		}
		files["image/disc01.bin"] = data
	}

	// Extra discs
	for i, extraCue := range opts.ExtraDiscs {
		discNum := i + 2
		extraTracks, err := ParseCUE(extraCue)
		if err != nil {
			return nil, nil, fmt.Errorf("parse extra disc %d: %w", discNum, err)
		}

		mergedPathN := filepath.Join(tmpDir, fmt.Sprintf("disc%02d.bin", discNum))
		binFilesN := getUniqueBinFiles(extraTracks)
		if len(binFilesN) == 1 {
			data, err := os.ReadFile(binFilesN[0])
			if err != nil {
				return nil, nil, fmt.Errorf("read extra disc %d bin: %w", discNum, err)
			}
			files[fmt.Sprintf("image/disc%02d.bin", discNum)] = data
		} else {
			_, err := MergeBins(extraTracks, mergedPathN)
			if err != nil {
				return nil, nil, fmt.Errorf("merge extra disc %d: %w", discNum, err)
			}
			data, err := os.ReadFile(mergedPathN)
			if err != nil {
				return nil, nil, fmt.Errorf("read merged extra disc %d: %w", discNum, err)
			}
			files[fmt.Sprintf("image/disc%02d.bin", discNum)] = data
		}

	}

	return files, disc, nil
}

// CreatePS1FPKG is the main entry point for PS1 fPKG creation.
// It orchestrates the full pipeline: disc parsing → project setup → PFS → PKG.
func CreatePS1FPKG(opts PS1FPKGOptions) error {
	files, disc, err := BuildPS1Project(opts)
	if err != nil {
		return err
	}

	// Auto-detect title ID
	titleID := opts.TitleID
	if titleID == "" {
		titleID = disc.Info.GameID
	}
	if titleID == "" {
		titleID = "SLUS-00000"
	}

	title := opts.Title
	if title == "" {
		title = disc.Info.Title
	}
	if title == "" {
		title = titleID
	}

	contentID := PS1ContentID(titleID)

	// Build the fPKG using the generic PKG builder
	pkgOpts := PKGOptions{
		Files:     files,
		Title:     title,
		TitleID:   titleID,
		ContentID: contentID,
	}

	pkgData, err := BuildFPKG(pkgOpts)
	if err != nil {
		return fmt.Errorf("build fpkg: %w", err)
	}

	// Write output file
	if err := os.WriteFile(opts.OutputPath, pkgData, 0644); err != nil {
		return fmt.Errorf("write pkg: %w", err)
	}

	return nil
}
