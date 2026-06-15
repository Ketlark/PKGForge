package fpkg

// This file implements PS1-specific fPKG creation logic.
//
// PS1 fPKG structure (inside the PFS image):
//
//	eboot.bin              — PS1 emulator (ps1_emu or ps1_netemu)
//	config-title.txt       — PS1HD runtime config, including the disc image path
//	bios/SCPH550x.bin      — OpenBIOS copies named for PS1HD region autodetect
//	sce_module/libc.prx    — C library module
//	sce_module/libSceFios2.prx
//	sce_module/libSceNpToolkit2.prx
//	data/
//	  disc1.bin            — merged PS1 disc image (raw 2352 bps sectors)
//	  disc1.cue            — package-local cue sheet pointing at disc1.bin
//	  disc1.toc            — PS1HD TOC sidecar generated from the CUE
//	  disc2.bin            — (optional, multi-disc)
//	  ...
//	sce_sys/
//	  icon0.png            — 512x512 icon
//	  save_data.png        — 228x128 save-data artwork consumed by sceSaveData
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
	AnalogSticks  bool // Enable analog stick simulation
	SkipBootLogo  bool // Skip Sony/PlayStation boot logo
	Force60Hz     bool // Force 60Hz output
	EnableCDDATOC bool // Generate TOC for CDDA music

	// EmulatorFilesDir is the directory containing emulator binaries (eboot.bin, libc.prx).
	// If empty, the default embedded path is used.
	EmulatorFilesDir string

	// RuntimeProfile selects the PS1HD project layout.
	// Empty defaults to auto-detecting from the emulator files.
	RuntimeProfile PS1RuntimeProfile

	OnProgress func(percent float64, phase string)
}

// PS1RuntimeProfile selects the PS1HD runtime layout to generate.
type PS1RuntimeProfile string

const (
	// PS1RuntimeProfileModern matches PSX-FPKG/PS Classics fPKG Builder.
	PS1RuntimeProfileModern PS1RuntimeProfile = "modern"

	// PS1RuntimeProfileSIEALua matches older Markus95 PSX2PS4 packages that
	// register discs through SIEA/app_boot.lua and --image-dir=data.
	PS1RuntimeProfileSIEALua PS1RuntimeProfile = "siea-lua"
)

// ---------------------------------------------------------------------------
// PS1 TOC generation
// ---------------------------------------------------------------------------

// TOCEntry represents a track entry in the PS1 TOC.
type TOCEntry struct {
	Number   int
	StartLBA int
	Mode     int // 0 = audio, 1 = mode 1, 2 = mode 2
}

// GeneratePS1TOC generates the binary TOC sidecar consumed by PS1HD.
// The layout matches Goatman13/Cue2toc: a 30-byte A0/A1/A2 header followed by
// 10-byte BCD track descriptors with the PS1HD-compatible +2s offset.
func GeneratePS1TOC(tracks []CueTrack, imageSize int64) []byte {
	if len(tracks) == 0 {
		return nil
	}

	toc := []byte{
		0x41, 0x00, 0xa0, 0x00, 0x00, 0x00, 0x00, 0x01, 0x20, 0x00,
		0x01, 0x00, 0xa1, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0xa2, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	toc[17] = bcdByte(len(tracks))
	endM, endS, endF := msfFromSectorsAlt(int(imageSize/2352) + 150)
	toc[27] = bcdByte(endM)
	toc[28] = bcdByte(endS)
	toc[29] = bcdByte(endF)

	for i, track := range tracks {
		buf := make([]byte, 10)
		if i == 0 {
			buf[0] = 0x41
		} else {
			buf[0] = 0x01
			if track.PregapLBA >= 0 {
				m, s, f := msfFromSectorsAlt(track.PregapLBA + 150)
				buf[3] = bcdByte(m)
				buf[4] = bcdByte(s)
				buf[5] = bcdByte(f)
			}
		}
		buf[2] = bcdByte(i + 1)

		sector := track.StartLBA + 150
		if i == 0 {
			sector = 150
			m, s, f := msfFromSectors(sector)
			buf[7] = bcdByte(m)
			buf[8] = bcdByte(s)
			buf[9] = bcdByte(f)
			toc = append(toc, buf...)
			continue
		}
		m, s, f := msfFromSectorsAlt(sector)
		buf[7] = bcdByte(m)
		buf[8] = bcdByte(s)
		buf[9] = bcdByte(f)

		toc = append(toc, buf...)
	}

	return toc
}

func bcdByte(i int) byte {
	return byte(i%10 + 16*((i/10)%10))
}

func msfFromSectors(sectors int) (int, int, int) {
	if sectors < 0 {
		sectors = 0
	}
	totalSeconds := sectors / 75
	frame := sectors % 75
	minute := totalSeconds / 60
	second := totalSeconds % 60
	return minute, second, frame
}

func msfFromSectorsAlt(sectors int) (int, int, int) {
	totalSeconds := sectors / 75
	frame := sectors % 75
	minute := totalSeconds / 60
	second := totalSeconds % 60
	if frame == 0 {
		frame = 75
		if second != 0 {
			second--
		} else {
			second = 59
			minute--
		}
	}
	if minute < 0 {
		return 0, 0, 0
	}
	return minute, second, frame
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
	// Normalize the game ID: SCUS-94244/SCUS_942.44 → SCUS94244.
	normalized := normalizeTitleID(gameID)

	// Generate a short hash from the normalized ID
	h := sha256.Sum256([]byte("ps1_" + normalized))
	hashStr := fmt.Sprintf("%x", h)[:16]

	return fmt.Sprintf("UP9000-%s_00-%s", normalized, strings.ToUpper(hashStr))
}

// ---------------------------------------------------------------------------
// PS1 emulator config
// ---------------------------------------------------------------------------

type ps1EmuConfigOptions struct {
	RuntimeProfile    PS1RuntimeProfile
	Template          []byte
	BiosDir           string
	HideSceOSD        bool
	HasGlobalGameData bool
}

// PS1EmuConfig generates the default PSX-FPKG/PS Classics-compatible PS1HD
// configuration text for config-title.txt.
func PS1EmuConfig(opts PS1FPKGOptions, titleID string, discCount int) string {
	return ps1ModernEmuConfig(opts, titleID, discCount, ps1EmuConfigOptions{
		HideSceOSD: opts.SkipBootLogo,
	})
}

func ps1RuntimeEmuConfig(opts PS1FPKGOptions, titleID string, discCount int, cfg ps1EmuConfigOptions) string {
	if cfg.RuntimeProfile == PS1RuntimeProfileSIEALua {
		return ps1SIEAEmuConfig(opts, discCount, cfg)
	}
	return ps1ModernEmuConfig(opts, titleID, discCount, cfg)
}

func ps1ModernEmuConfig(opts PS1FPKGOptions, titleID string, discCount int, cfg ps1EmuConfigOptions) string {
	if discCount < 1 {
		discCount = 1
	}

	prefix := ps1ConfigTemplatePrefix(cfg.Template)
	if len(prefix) == 0 {
		prefix = []string{"# PKG Forge PS1HD defaults", "", "--scale=6"}
	} else if !ps1ConfigHasKey(prefix, "--scale") {
		prefix = append(prefix, "--scale=6")
	}
	if cfg.HideSceOSD && !ps1ConfigHasKey(prefix, "--bios-hide-sce-osd") {
		prefix = append(prefix, "--bios-hide-sce-osd=1")
	}
	if !ps1ConfigHasKey(prefix, "--has-shown-start-select-help") {
		prefix = append(prefix, "--has-shown-start-select-help=0")
	}
	if opts.AnalogSticks && !ps1ConfigHasKey(prefix, "--sim-analog-pad") {
		prefix = append(prefix, "--sim-analog-pad=0x2020")
	}

	lines := append([]string{}, prefix...)
	lines = append(lines, "", "# following settings are machine-generated")
	lines = ps1AppendGeneratedConfigLine(lines, prefix, fmt.Sprintf("--ps1-title-id=%s", normalizeTitleID(titleID)))
	lines = ps1AppendGeneratedConfigLine(lines, prefix, fmt.Sprintf("--title-id=%s", normalizeTitleID(titleID)))
	lines = ps1AppendGeneratedConfigLine(lines, prefix, fmt.Sprintf("--region=\"%s\"", ps1HDRegion(titleID)))

	for i := 0; i < discCount; i++ {
		lines = append(lines, fmt.Sprintf("--image=\"%s\"", ps1DataBinPath(i+1)))
	}
	lines = append(lines,
		"--ps4-trophies=0",
		"--ps5-uds=0",
		"--trophies=0",
	)
	if cfg.BiosDir != "" {
		lines = append(lines, fmt.Sprintf("--bios-dir=\"%s\"", cfg.BiosDir))
	}

	if discCount > 1 {
		for i := 0; i < discCount; i++ {
			lines = append(lines,
				fmt.Sprintf("--image%d=\"%s\"", i, ps1DataBinPath(i+1)),
				fmt.Sprintf("--imageName%d=\"Disc %d\"", i, i+1),
			)
		}
	}
	if opts.Force60Hz {
		lines = append(lines, "--gpu-scanout-fps-override=60")
	}

	return strings.Join(lines, "\n") + "\n"
}

func ps1AppendGeneratedConfigLine(lines, prefix []string, line string) []string {
	key := ps1ConfigKey(line)
	if ps1ConfigHasKey(prefix, key) || ps1ConfigHasKey(lines, key) {
		return lines
	}
	return append(lines, line)
}

func ps1SIEAEmuConfig(opts PS1FPKGOptions, discCount int, cfg ps1EmuConfigOptions) string {
	if discCount < 1 {
		discCount = 1
	}

	prefix := ps1ConfigTemplatePrefix(cfg.Template)
	if len(prefix) == 0 {
		prefix = []string{
			"# PKG Forge SIEA PS1HD defaults",
			"",
			"--scale=6",
			"--gamma=5",
			"--brightness=9",
			"--contrast=8",
			"--sim-analog-pad=0x2020",
			"",
			"# Use the shared image directory",
			"--image-dir=data",
			"",
			"--force-pad-connect=0b1",
			"--use-lopnor-spu=1",
		}
	} else {
		for _, required := range []string{
			"--scale=6",
			"--image-dir=data",
			"--force-pad-connect=0b1",
			"--use-lopnor-spu=1",
		} {
			if !ps1ConfigHasKey(prefix, ps1ConfigKey(required)) {
				prefix = append(prefix, required)
			}
		}
	}
	if !ps1ConfigHasKey(prefix, "--has-shown-start-select-help") {
		prefix = append(prefix, "--has-shown-start-select-help=0")
	}

	lines := append([]string{}, prefix...)
	lines = append(lines,
		"",
		"# following settings are machine-generated",
		"--region-dir=SIEA",
		"--ps4-trophies=1",
		"--ps5-uds=1",
		"--trophies=1",
	)
	if cfg.HasGlobalGameData {
		lines = append(lines, "--globalgamedata-dir=global")
	}
	if opts.Force60Hz {
		lines = append(lines, "--gpu-scanout-fps-override=ntsc")
	}
	return strings.Join(lines, "\n") + "\n"
}

func ps1ConfigTemplatePrefix(template []byte) []string {
	if len(template) == 0 {
		return nil
	}
	raw := strings.ReplaceAll(string(template), "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	var prefix []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "# following settings are machine-generated" {
			break
		}
		if strings.TrimSpace(line) == "" && len(prefix) == 0 {
			continue
		}
		prefix = append(prefix, strings.TrimRight(line, " \t"))
	}
	for len(prefix) > 0 && strings.TrimSpace(prefix[len(prefix)-1]) == "" {
		prefix = prefix[:len(prefix)-1]
	}
	return prefix
}

func ps1ConfigHasKey(lines []string, key string) bool {
	key = ps1ConfigKey(key)
	for _, line := range lines {
		if ps1ConfigKey(line) == key {
			return true
		}
	}
	return false
}

func ps1ConfigKey(line string) string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "--") {
		return ""
	}
	for _, sep := range []string{"=", " "} {
		if idx := strings.Index(line, sep); idx >= 0 {
			return line[:idx]
		}
	}
	return line
}

func ps1DataBinPath(discNum int) string {
	return fmt.Sprintf("data/disc%d.bin", discNum)
}

func ps1DataCuePath(discNum int) string {
	return fmt.Sprintf("data/disc%d.cue", discNum)
}

func ps1DataTOCPath(discNum int) string {
	return fmt.Sprintf("data/disc%d.toc", discNum)
}

// RewritePS1CueForPackage creates a package-local CUE sheet for the merged disc.
// The packaged image is always data/discN.bin, regardless of the user's source
// filenames or whether the source CUE referenced multiple BIN files.
func RewritePS1CueForPackage(tracks []CueTrack, discNum int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "FILE \"disc%d.bin\" BINARY\n", discNum)
	for _, track := range tracks {
		fmt.Fprintf(&b, "  TRACK %02d %s\n", track.Number, track.Mode)
		if track.PregapLBA >= 0 {
			fmt.Fprintf(&b, "    INDEX 00 %s\n", lbaToCueTime(track.PregapLBA))
		}
		fmt.Fprintf(&b, "    INDEX 01 %s\n", lbaToCueTime(track.StartLBA))
	}
	return b.String()
}

func lbaToCueTime(lba int) string {
	if lba < 0 {
		lba = 0
	}
	minute := lba / (60 * 75)
	second := (lba / 75) % 60
	frame := lba % 75
	return fmt.Sprintf("%02d:%02d:%02d", minute, second, frame)
}

func ps1HDRegion(titleID string) string {
	switch regionFromID(titleID) {
	case "europe":
		return "SCEE"
	case "america":
		return "SCEA"
	case "japan":
		return "SCEI"
	default:
		return "SCEA"
	}
}

func resolvePS1RuntimeProfile(opts PS1FPKGOptions, files map[string][]byte) PS1RuntimeProfile {
	switch opts.RuntimeProfile {
	case PS1RuntimeProfileModern, PS1RuntimeProfileSIEALua:
		return opts.RuntimeProfile
	}
	if _, ok := files["SIEA/app_boot.lua"]; ok {
		return PS1RuntimeProfileSIEALua
	}
	if _, ok := files["SIEA/config-region.txt"]; ok {
		return PS1RuntimeProfileSIEALua
	}
	return PS1RuntimeProfileModern
}

func ps1BiosDirOverride(files map[string][]byte) string {
	// The PS1HD emulator autodetects BIOS files at assets/PS1HD/bios/.
	// If they are at that path, no --bios-dir flag is needed (default).
	// If they are at bios/ (legacy layout), set --bios-dir="bios".
	if ps1HasAnyFile(files,
		"assets/PS1HD/bios/SCPH5500.bin",
		"assets/PS1HD/bios/SCPH5501.bin",
		"assets/PS1HD/bios/SCPH5502.bin",
	) {
		return ""
	}
	if ps1HasAnyFile(files,
		"bios/SCPH5500.bin",
		"bios/SCPH5501.bin",
		"bios/SCPH5502.bin",
	) {
		return "bios"
	}
	return ""
}

func ps1HasAnyFile(files map[string][]byte, paths ...string) bool {
	for _, path := range paths {
		if len(files[path]) > 0 {
			return true
		}
	}
	return false
}

func setPS1LaunchBackground(files map[string][]byte, data []byte) {
	if len(data) == 0 {
		return
	}
	files["sce_sys/pic1.png"] = data
	files["sce_sys/pic0.png"] = data
}

func ensurePS1LaunchBackground(files map[string][]byte, titleID string) {
	if pic1 := files["sce_sys/pic1.png"]; len(pic1) > 0 {
		if len(files["sce_sys/pic0.png"]) == 0 {
			files["sce_sys/pic0.png"] = pic1
		}
		return
	}
	if pic0 := files["sce_sys/pic0.png"]; len(pic0) > 0 {
		files["sce_sys/pic1.png"] = pic0
		return
	}
	setPS1LaunchBackground(files, defaultPic1PNG(titleID))
}

func ps1HasGlobalGameData(files map[string][]byte) bool {
	for path := range files {
		if strings.HasPrefix(path, "global/") {
			return true
		}
	}
	return false
}

func ps1HasRetailBIOS(files map[string][]byte) bool {
	retailBIOS := map[string]string{
		"assets/PS1HD/bios/SCPH5500.bin": "9c0421858e217805f4abe18698afea8d5aa36ff0727eb8484944e00eb5e7eadb",
		"assets/PS1HD/bios/SCPH5501.bin": "11052b6499e466bbf0a709b1f9cb6834a9418e66680387912451e971cf8a1fef",
		"assets/PS1HD/bios/SCPH5502.bin": "1faaa18fa820a0225e488d9f086296b8e6c46df739666093987ff7d8fd352c09",
	}
	for path, want := range retailBIOS {
		if data := files[path]; len(data) > 0 {
			sum := sha256.Sum256(data)
			if fmt.Sprintf("%x", sum[:]) == want {
				return true
			}
		}
	}
	return false
}

func applyPS1SIEALuaProfile(files map[string][]byte, opts PS1FPKGOptions, discCount int) {
	files["SIEA/app_boot.lua"] = []byte(generatePS1SIEAAppBootLua(discCount))
	files["SIEA/config-region.txt"] = []byte(mergePS1ConfigLines(string(files["SIEA/config-region.txt"]), ps1SIEARegionConfigLines(opts)))
}

func generatePS1SIEAAppBootLua(discCount int) string {
	if discCount < 1 {
		discCount = 1
	}
	var b strings.Builder
	b.WriteString("local discs = {\n")
	for i := 1; i <= discCount; i++ {
		fmt.Fprintf(&b, "    \"disc%d.bin\",\n", i)
	}
	b.WriteString("}\n\n")
	b.WriteString("for i, disc in ipairs(discs) do\n")
	b.WriteString("    EM_SetCDRom(disc, i)\n")
	b.WriteString("end\n")
	return b.String()
}

func ps1SIEARegionConfigLines(opts PS1FPKGOptions) []string {
	lines := []string{
		"--force-frame-blend=false",
		"--userui-region-selector=true",
	}
	if opts.SkipBootLogo {
		lines = append(lines, "--bios-hide-sce-osd=1")
	}
	if opts.Force60Hz {
		lines = append(lines, "--gpu-scanout-fps-override=ntsc")
	}
	return lines
}

func mergePS1ConfigLines(existing string, additions []string) string {
	existing = strings.ReplaceAll(existing, "\r\n", "\n")
	existing = strings.ReplaceAll(existing, "\r", "\n")
	var lines []string
	if strings.TrimSpace(existing) != "" {
		for _, line := range strings.Split(existing, "\n") {
			if strings.TrimSpace(line) != "" {
				lines = append(lines, strings.TrimRight(line, " \t"))
			}
		}
	}
	for _, line := range additions {
		if !ps1ConfigHasKey(lines, ps1ConfigKey(line)) {
			lines = append(lines, line)
		}
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
	if opts.OnProgress != nil {
		opts.OnProgress(5, "Parsing Disc 1")
	}
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

	// Icon and background
	iconPath := opts.Icon0
	if iconPath == "" {
		if coverPath, err := ResolvePS1Cover(opts.CuePath, titleID); err == nil {
			iconPath = coverPath
		}
	}
	if iconPath != "" {
		data, err := os.ReadFile(iconPath)
		if err == nil {
			files["sce_sys/icon0.png"] = data
		}
	}
	if opts.Pic1 != "" {
		if data, err := os.ReadFile(opts.Pic1); err == nil {
			setPS1LaunchBackground(files, data)
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
			"eboot.bin":                       "eboot.bin",
			"sce_module/libc.prx":             "sce_module/libc.prx",
			"sce_module/libSceFios2.prx":      "sce_module/libSceFios2.prx",
			"sce_module/libSceNpToolkit2.prx": "sce_module/libSceNpToolkit2.prx",
		},
	}

	emuDir, err := ResolveEmulatorsDir(opts.EmulatorFilesDir, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve emulator files: %w", err)
	}

	if opts.OnProgress != nil {
		opts.OnProgress(15, "Loading emulator files")
	}
	emuFiles, err := LoadEmulatorFiles(emuDir, emuSet)
	if err != nil {
		return nil, nil, fmt.Errorf("load ps1 emulator files: %w", err)
	}
	for k, v := range emuFiles {
		files[k] = v
	}
	if allEmuFiles, err := LoadEmulatorDirectoryFiles(emuDir, emuSet); err == nil {
		for k, v := range allEmuFiles {
			if _, exists := files[k]; !exists {
				files[k] = v
			}
		}
	}
	if opts.Pic1 == "" {
		if backgroundPath, err := ResolvePS1Background(opts.CuePath, titleID); err == nil {
			if data, err := os.ReadFile(backgroundPath); err == nil {
				setPS1LaunchBackground(files, data)
			}
		}
	}
	if len(files["sce_sys/pic1.png"]) == 0 && iconPath != "" {
		if data, err := ps1BackgroundFromImagePath(iconPath, titleID); err == nil {
			setPS1LaunchBackground(files, data)
		}
	}
	ensurePS1LaunchBackground(files, titleID)

	discCount := 1 + len(opts.ExtraDiscs)
	runtimeProfile := resolvePS1RuntimeProfile(opts, files)
	if runtimeProfile == PS1RuntimeProfileSIEALua {
		files["sce_sys/param.sfo"] = NewPS1SIEAParamSfo(title, titleID, contentID).Serialize()
	} else {
		files["sce_sys/param.sfo"] = NewPS1ParamSfo(title, titleID, contentID).Serialize()
	}

	configOptions := ps1EmuConfigOptions{
		RuntimeProfile:    runtimeProfile,
		Template:          files["config-title.txt"],
		BiosDir:           ps1BiosDirOverride(files),
		HideSceOSD:        opts.SkipBootLogo || ps1HasRetailBIOS(files),
		HasGlobalGameData: ps1HasGlobalGameData(files),
	}
	files["config-title.txt"] = []byte(ps1RuntimeEmuConfig(opts, titleID, discCount, configOptions))
	if runtimeProfile == PS1RuntimeProfileSIEALua {
		applyPS1SIEALuaProfile(files, opts, discCount)
	}

	// 3. Merge and add disc images
	tmpDir := filepath.Join(os.TempDir(), "pkg-forge-ps1")

	// Main disc
	tracks := disc.Info.Tracks

	// Merge bins into a single file
	mergedPath := filepath.Join(tmpDir, "disc01.bin")
	os.MkdirAll(filepath.Join(tmpDir, "data"), 0755)

	// Check if we actually need to merge (multi-bin vs single bin)
	if opts.OnProgress != nil {
		opts.OnProgress(30, "Reading Disc 1 image")
	}
	binFiles := getUniqueBinFiles(tracks)
	var discData []byte
	if len(binFiles) == 1 {
		// Single bin — just reference it directly
		data, err := os.ReadFile(binFiles[0])
		if err != nil {
			return nil, nil, fmt.Errorf("read bin: %w", err)
		}
		discData = data
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
		discData = data
	}
	files[ps1DataBinPath(1)] = discData
	files[ps1DataCuePath(1)] = []byte(RewritePS1CueForPackage(tracks, 1))
	files[ps1DataTOCPath(1)] = GeneratePS1TOC(tracks, int64(len(discData)))

	// Extra discs
	for i, extraCue := range opts.ExtraDiscs {
		discNum := i + 2
		if opts.OnProgress != nil {
			opts.OnProgress(35+float64(i)*5, fmt.Sprintf("Reading Disc %d image", discNum))
		}
		extraTracks, err := ParseCUE(extraCue)
		if err != nil {
			return nil, nil, fmt.Errorf("parse extra disc %d: %w", discNum, err)
		}

		mergedPathN := filepath.Join(tmpDir, fmt.Sprintf("disc%02d.bin", discNum))
		binFilesN := getUniqueBinFiles(extraTracks)
		var extraData []byte
		if len(binFilesN) == 1 {
			data, err := os.ReadFile(binFilesN[0])
			if err != nil {
				return nil, nil, fmt.Errorf("read extra disc %d bin: %w", discNum, err)
			}
			extraData = data
		} else {
			_, err := MergeBins(extraTracks, mergedPathN)
			if err != nil {
				return nil, nil, fmt.Errorf("merge extra disc %d: %w", discNum, err)
			}
			data, err := os.ReadFile(mergedPathN)
			if err != nil {
				return nil, nil, fmt.Errorf("read merged extra disc %d: %w", discNum, err)
			}
			extraData = data
		}
		files[ps1DataBinPath(discNum)] = extraData
		files[ps1DataCuePath(discNum)] = []byte(RewritePS1CueForPackage(extraTracks, discNum))
		files[ps1DataTOCPath(discNum)] = GeneratePS1TOC(extraTracks, int64(len(extraData)))
	}

	if opts.OnProgress != nil {
		opts.OnProgress(60, "Project files ready")
	}
	return files, disc, nil
}

// CreatePS1FPKG is the main entry point for PS1 fPKG creation.
// It orchestrates the full pipeline: disc parsing → project setup → PFS → PKG.
func CreatePS1FPKG(opts PS1FPKGOptions) error {
	if opts.OnProgress != nil {
		opts.OnProgress(0, "Preparing PS1 package")
	}
	files, disc, err := BuildPS1Project(opts)
	if err != nil {
		return err
	}
	if opts.OnProgress != nil {
		opts.OnProgress(65, "Building PS4 package")
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
