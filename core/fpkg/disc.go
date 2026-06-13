package fpkg

// This file implements disc image parsing for PS1 (.cue/.bin) and PS2 (.iso) formats.
//
// PS1 games use CUE/BIN format:
//   - .cue is a text file listing tracks (DATA + AUDIO)
//   - .bin is raw sector data (2352 bytes/sector for mode 2)
//   - Multi-bin: multiple .bin files referenced by one .cue
//
// PS2 games use ISO format:
//   - .iso is a plain ISO 9660 image (2048 bytes/sector)
//   - Contains SYSTEM.CNF with BOOT2= and other metadata
//   - CD-based PS2 games may use .bin/.cue (2352 bytes/sector)
//
// For fPKG creation, we need:
//   - Game ID (e.g. SCUS-97264, SLUS-20062)
//   - Game title (for param.sfo)
//   - Merged disc image data (single binary for PKG inclusion)

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Disc info structures
// ---------------------------------------------------------------------------

// DiscInfo contains metadata extracted from a disc image.
type DiscInfo struct {
	// GameID is the PlayStation title ID (e.g. "SCUS-97264", "SLUS-20062").
	GameID string

	// Title is the game title extracted from disc or database.
	Title string

	// Region inferred from the GameID prefix.
	Region string

	// Format is "bin" (2352 bps) or "iso" (2048 bps).
	Format string

	// IsMultiBin is true when a .cue references multiple .bin files.
	IsMultiBin bool

	// Tracks lists the CUE tracks (only for PS1).
	Tracks []CueTrack

	// SystemCNF contains parsed SYSTEM.CNF entries (only for PS2).
	SystemCNF map[string]string
}

// CueTrack represents a single track in a CUE sheet.
type CueTrack struct {
	Number   int
	Mode     string // "MODE2/2352", "AUDIO", etc.
	File     string
	StartLBA int
}

// ---------------------------------------------------------------------------
// CUE sheet parser
// ---------------------------------------------------------------------------

var cueTrackRe = regexp.MustCompile(`^\s*TRACK\s+(\d+)\s+(.+)`)
var cueFileRe = regexp.MustCompile(`^\s*FILE\s+"(.+?)"\s+(.+)`)
var cueIndex01Re = regexp.MustCompile(`^\s*INDEX\s+01\s+(\d+):(\d+):(\d+)`)

// ParseCUE reads a .cue file and returns the list of tracks.
// The .bin file paths are resolved relative to the cue file's directory.
func ParseCUE(cuePath string) ([]CueTrack, error) {
	f, err := os.Open(cuePath)
	if err != nil {
		return nil, fmt.Errorf("open cue: %w", err)
	}
	defer f.Close()

	var tracks []CueTrack
	cueDir := filepath.Dir(cuePath)

	scanner := bufio.NewScanner(f)
	var cur *CueTrack
	currentFile := ""

	for scanner.Scan() {
		line := scanner.Text()

		// FILE "name.bin" BINARY
		if m := cueFileRe.FindStringSubmatch(line); m != nil {
			currentFile = resolvePath(cueDir, m[1])
			continue
		}

		// TRACK 01 MODE2/2352
		if m := cueTrackRe.FindStringSubmatch(line); m != nil {
			num := 0
			fmt.Sscanf(m[1], "%d", &num)
			tracks = append(tracks, CueTrack{
				Number: num,
				Mode:   strings.TrimSpace(m[2]),
				File:   currentFile,
			})
			cur = &tracks[len(tracks)-1]
			continue
		}

		// INDEX 01 MM:SS:FF
		if cur != nil {
			if m := cueIndex01Re.FindStringSubmatch(line); m != nil {
				mm, ss, ff := 0, 0, 0
				fmt.Sscanf(m[1], "%d", &mm)
				fmt.Sscanf(m[2], "%d", &ss)
				fmt.Sscanf(m[3], "%d", &ff)
				cur.StartLBA = mm*60*75 + ss*75 + ff
			}
		}
	}

	if len(tracks) == 0 {
		return nil, fmt.Errorf("no tracks found in %s", cuePath)
	}

	return tracks, nil
}

// ---------------------------------------------------------------------------
// PS1 disc parsing
// ---------------------------------------------------------------------------

// PS1DiscResult holds the result of parsing a PS1 disc image.
type PS1DiscResult struct {
	Info     DiscInfo
	TrackNum int // total number of tracks
	HasCDDA  bool // true if audio tracks present
}

// ParsePS1Disc parses a PS1 .cue file and extracts game metadata.
// It reads the .bin data to find the game ID from the disc header.
func ParsePS1Disc(cuePath string) (*PS1DiscResult, error) {
	tracks, err := ParseCUE(cuePath)
	if err != nil {
		return nil, err
	}

	result := &PS1DiscResult{
		Info: DiscInfo{
			Format:     "bin",
			Tracks:     tracks,
			IsMultiBin: len(getUniqueBinFiles(tracks)) > 1,
		},
		TrackNum: len(tracks),
	}

	// Detect CDDA (audio tracks after data track)
	for i, t := range tracks {
		if i > 0 && strings.HasPrefix(t.Mode, "AUDIO") {
			result.HasCDDA = true
			break
		}
	}

	// Read the data track to extract game ID
	// PS1 game ID is in the "Licensed by" string at sector 0, offset 0x4C
	// Format: "Licensed  by  SONY    COMPUTER ENTERTAINMENT" followed by region marker
	// The actual TITLE_ID is at offset 0x236 in the SYSTEM.CNF sector (sector 2 for Mode 2)
	gameID, err := extractPS1GameID(tracks)
	if err == nil && gameID != "" {
		result.Info.GameID = gameID
		result.Info.Region = regionFromID(gameID)
	}

	return result, nil
}

// extractPS1GameID reads the first data track to find the PS1 game ID.
// PS1 stores the executable name in the disc header at sector 2 (SYSTEM.CNF area).
// The ID can be found by reading the PS-X EXE header or searching SYSTEM.CNF.
func extractPS1GameID(tracks []CueTrack) (string, error) {
	if len(tracks) == 0 {
		return "", fmt.Errorf("no tracks")
	}

	// Find the data track
	var dataTrack *CueTrack
	for i := range tracks {
		if strings.Contains(tracks[i].Mode, "MODE") {
			dataTrack = &tracks[i]
			break
		}
	}
	if dataTrack == nil {
		return "", fmt.Errorf("no data track found")
	}

	f, err := os.Open(dataTrack.File)
	if err != nil {
		return "", fmt.Errorf("open bin: %w", err)
	}
	defer f.Close()

	// Read sector size: MODE2/2352 = 2352 bytes, MODE1/2048 = 2048 bytes
	sectorSize := 2352
	if strings.Contains(dataTrack.Mode, "2048") {
		sectorSize = 2048
	}

	// For Mode 2, data starts at offset 24 within each sector (sync + header + subheader)
	dataOffset := 0
	if sectorSize == 2352 {
		dataOffset = 24
	}

	// Strategy: read the first 16 sectors and search for game ID patterns
	// The PS1 boot string is at sector 0, offset 0 (sync) then 0x000C-0x004F has the "Licensed by" text
	// Game ID format: [A-Z]{4}-[0-9]{5}
	idRe := regexp.MustCompile(`[A-Z]{4}-\d{5}`)

	buf := make([]byte, sectorSize)

	// Check sector 0 for boot ID (offset 0x236 in data area contains boot filename)
	// Also scan early sectors for SYSTEM.CNF-like content
	for sector := 0; sector < 16; sector++ {
		offset := int64(sector * sectorSize)
		if sectorSize == 2352 {
			offset += int64(dataOffset)
		}
		_, err := f.Seek(offset, io.SeekStart)
		if err != nil {
			break
		}
		n, err := f.Read(buf)
		if err != nil || n < 256 {
			continue
		}

		// Search for the ID pattern in the sector data
		text := string(buf[:n])
		if matches := idRe.FindAllString(text, -1); len(matches) > 0 {
			return matches[0], nil
		}
	}

	return "", fmt.Errorf("game ID not found in disc image")
}

// ---------------------------------------------------------------------------
// PS2 disc parsing
// ---------------------------------------------------------------------------

// ParsePS2Disc parses a PS2 .iso file and extracts game metadata.
// It reads the ISO 9660 to find SYSTEM.CNF and extract the BOOT2 game ID.
func ParsePS2Disc(isoPath string) (*DiscInfo, error) {
	f, err := os.Open(isoPath)
	if err != nil {
		return nil, fmt.Errorf("open iso: %w", err)
	}
	defer f.Close()

	info := &DiscInfo{
		Format: "iso",
	}

	// Read SYSTEM.CNF from the ISO
	cnfData, err := readISOFile(f, "SYSTEM.CNF")
	if err == nil && len(cnfData) > 0 {
		info.SystemCNF = parseSystemCNF(cnfData)

		// Extract game ID from BOOT2 parameter
		// BOOT2 = cdrom0:\SLUS_200.62;1  →  SLUS-20062
		if boot2, ok := info.SystemCNF["BOOT2"]; ok {
			gameID := extractPS2GameIDFromPath(boot2)
			if gameID != "" {
				info.GameID = gameID
				info.Region = regionFromID(gameID)
			}
		}
	}

	// Fallback: try to detect game ID from the disc header directly
	if info.GameID == "" {
		gameID, err := extractPS2GameIDFromISO(f)
		if err == nil && gameID != "" {
			info.GameID = gameID
			info.Region = regionFromID(gameID)
		}
	}

	return info, nil
}

// ParsePS2DiscFromCUE parses a PS2 CD-based game from .cue/.bin format.
func ParsePS2DiscFromCUE(cuePath string) (*DiscInfo, error) {
	tracks, err := ParseCUE(cuePath)
	if err != nil {
		return nil, err
	}

	info := &DiscInfo{
		Format:     "bin",
		Tracks:     tracks,
		IsMultiBin: len(getUniqueBinFiles(tracks)) > 1,
	}

	// Find data track
	var dataTrack *CueTrack
	for i := range tracks {
		if strings.Contains(tracks[i].Mode, "MODE") {
			dataTrack = &tracks[i]
			break
		}
	}

	if dataTrack != nil {
		binFile, err := os.Open(dataTrack.File)
		if err == nil {
			cnfData, err := readSystemCNFFromBin(binFile, dataTrack)
			binFile.Close()
			if err == nil && len(cnfData) > 0 {
				info.SystemCNF = parseSystemCNF(cnfData)
				if boot2, ok := info.SystemCNF["BOOT2"]; ok {
					gameID := extractPS2GameIDFromPath(boot2)
					if gameID != "" {
						info.GameID = gameID
						info.Region = regionFromID(gameID)
					}
				}
			}
		}
	}

	return info, nil
}

// ---------------------------------------------------------------------------
// ISO 9660 reader (minimal, just enough to read SYSTEM.CNF)
// ---------------------------------------------------------------------------

// readISOFile reads a file from an ISO 9660 image.
// Only reads from the primary volume descriptor's root directory.
func readISOFile(f *os.File, filename string) ([]byte, error) {
	// Read Primary Volume Descriptor at sector 16 (LBA 16)
	pvd := make([]byte, 2048)
	if _, err := f.ReadAt(pvd, 16*2048); err != nil {
		return nil, fmt.Errorf("read PVD: %w", err)
	}

	// Check PVD signature
	if string(pvd[0:5]) != "CD001" {
		return nil, fmt.Errorf("not a valid ISO 9660 image")
	}

	// Root directory record at offset 156 in PVD, length 34
	rootLBA := binary.LittleEndian.Uint32(pvd[156+2 : 156+6])
	rootSize := binary.LittleEndian.Uint32(pvd[156+10 : 156+14])

	// Read root directory
	dirData := make([]byte, rootSize)
	if _, err := f.ReadAt(dirData, int64(rootLBA)*2048); err != nil {
		return nil, fmt.Errorf("read root dir: %w", err)
	}

	// Search for the file in directory entries
	return findISOFileEntry(f, dirData, filename)
}

// findISOFileEntry searches directory entries for a file and reads its data.
func findISOFileEntry(f *os.File, dirData []byte, filename string) ([]byte, error) {
	targetUpper := strings.ToUpper(filename)

	for offset := 0; offset < len(dirData); {
		recLen := int(dirData[offset])
		if recLen == 0 {
			// Padding — skip to next sector
			offset = (offset/2048 + 1) * 2048
			if offset >= len(dirData) {
				break
			}
			continue
		}

		// File identifier location
		idLen := int(dirData[offset+32])
		idStart := offset + 33

		// Check if this is a directory (bit 1 of flags)
		flags := dirData[offset+25]
		isDir := (flags & 0x02) != 0

		if idLen > 0 && idLen+idStart <= len(dirData) {
			identifier := strings.ToUpper(string(dirData[idStart : idStart+idLen]))
			// Strip version number (;1)
			if idx := strings.Index(identifier, ";"); idx >= 0 {
				identifier = identifier[:idx]
			}

			if identifier == targetUpper && !isDir {
				// Found the file
				fileLBA := binary.LittleEndian.Uint32(dirData[offset+2 : offset+6])
				fileSize := binary.LittleEndian.Uint32(dirData[offset+10 : offset+14])

				fileData := make([]byte, fileSize)
				if _, err := f.ReadAt(fileData, int64(fileLBA)*2048); err != nil {
					return nil, fmt.Errorf("read file %s: %w", filename, err)
				}
				return fileData, nil
			}
		}

		offset += recLen
	}

	return nil, fmt.Errorf("file %s not found in ISO", filename)
}

// ---------------------------------------------------------------------------
// SYSTEM.CNF parsing
// ---------------------------------------------------------------------------

// parseSystemCNF parses SYSTEM.CNF content into a key=value map.
// Format example:
//
//	BOOT2 = cdrom0:\SLUS_200.62;1
//	VER = 1.01
//	VMODE = NTSC
func parseSystemCNF(data []byte) map[string]string {
	result := make(map[string]string)

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[strings.ToUpper(key)] = value
		}
	}

	return result
}

// extractPS2GameIDFromPath extracts a PS2 game ID from a BOOT2 path.
// Input:  "cdrom0:\SLUS_200.62;1" or "cdrom0:\\SLUS_200.62;1"
// Output: "SLUS-20062"
func extractPS2GameIDFromPath(path string) string {
	// Remove cdrom0: prefix and path separators
	path = strings.TrimPrefix(path, "cdrom0:")
	path = strings.TrimPrefix(path, "\\")
	path = strings.TrimPrefix(path, "/")

	// Extract filename before semicolon
	if idx := strings.Index(path, ";"); idx >= 0 {
		path = path[:idx]
	}

	// Convert from SLUS_200.62 format to SLUS-20062
	// The dot separates disc number from the serial suffix
	id := strings.ReplaceAll(path, "_", "-")
	id = strings.ReplaceAll(id, ".", "")

	// Validate format: 4 letters, dash, 5 digits
	re := regexp.MustCompile(`^[A-Z]{4}-\d{5}$`)
	if re.MatchString(id) {
		return id
	}

	return ""
}

// extractPS2GameIDFromISO attempts to find the game ID by scanning ISO sectors directly.
func extractPS2GameIDFromISO(f *os.File) (string, error) {
	buf := make([]byte, 2048)
	idRe := regexp.MustCompile(`[A-Z]{4}_\d{3}\.\d{2}`)

	// Scan first 32 sectors
	for sector := 0; sector < 32; sector++ {
		_, err := f.ReadAt(buf, int64(sector)*2048)
		if err != nil {
			break
		}
		text := string(buf)
		if matches := idRe.FindAllString(text, -1); len(matches) > 0 {
			// Convert SLUS_200.62 to SLUS-20062
			id := matches[0]
			id = strings.ReplaceAll(id, "_", "-")
			id = strings.ReplaceAll(id, ".", "")
			if len(id) == 9 {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("PS2 game ID not found")
}

// readSystemCNFFromBin reads SYSTEM.CNF from a BIN track (Mode 2, 2352 bytes/sector).
func readSystemCNFFromBin(f *os.File, track *CueTrack) ([]byte, error) {
	sectorSize := 2352
	if strings.Contains(track.Mode, "2048") {
		sectorSize = 2048
	}

	// SYSTEM.CNF is typically at sector 2 in the directory
	// For Mode 2: data starts at offset 24, data length at offset 16-17 (2 bytes LE)
	buf := make([]byte, sectorSize)

	// Search sectors 0-16 for SYSTEM.CNF content
	for sector := 0; sector < 16; sector++ {
		offset := int64(sector * sectorSize)
		_, err := f.Seek(offset, io.SeekStart)
		if err != nil {
			break
		}
		_, err = f.Read(buf)
		if err != nil {
			break
		}

		// Extract data portion
		var data []byte
		if sectorSize == 2352 {
			// Mode 2 Form 1: sync(12) + header(4) + subheader(8) + data(2048) + edc(4) + ecc(276)
			data = buf[24 : 24+2048]
		} else {
			data = buf
		}

		text := string(data)
		if strings.Contains(strings.ToUpper(text), "BOOT2") {
			return data, nil
		}
	}

	return nil, fmt.Errorf("SYSTEM.CNF not found in BIN track")
}

// ---------------------------------------------------------------------------
// BIN merging (multi-bin → single bin)
// ---------------------------------------------------------------------------

// MergeBins concatenates multiple .bin files from a CUE sheet into a single file.
// Returns the path to the merged file and the total size.
func MergeBins(tracks []CueTrack, outputPath string) (int64, error) {
	out, err := os.Create(outputPath)
	if err != nil {
		return 0, fmt.Errorf("create merged bin: %w", err)
	}
	defer out.Close()

	var total int64
	for _, track := range tracks {
		f, err := os.Open(track.File)
		if err != nil {
			return 0, fmt.Errorf("open track %d bin %s: %w", track.Number, track.File, err)
		}

		n, err := io.Copy(out, f)
		f.Close()
		if err != nil {
			return 0, fmt.Errorf("copy track %d: %w", track.Number, err)
		}
		total += n
	}

	return total, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resolvePath resolves a CUE-referenced filename relative to the CUE file's directory.
func resolvePath(baseDir, filename string) string {
	// Try as-is first (absolute path)
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join(baseDir, filename)
}

// getUniqueBinFiles returns the unique .bin file paths from a track list.
func getUniqueBinFiles(tracks []CueTrack) []string {
	seen := make(map[string]bool)
	var result []string
	for _, t := range tracks {
		if !seen[t.File] {
			seen[t.File] = true
			result = append(result, t.File)
		}
	}
	return result
}

// regionFromID returns the region code based on the game ID prefix.
func regionFromID(id string) string {
	if len(id) < 4 {
		return "unknown"
	}
	prefix := strings.ToUpper(id[:4])
	switch prefix[:1] {
	case "S":
		switch prefix[1:3] {
		case "CE", "CO":
			return "europe" // SCES, SCOS
		case "CU":
			return "america" // SCUS
		case "PS", "CP":
			return "japan" // SPS, SCP
		case "LE", "LJ":
			return "europe" // SLES, SLUS → SL can be US/EU/JP
		case "LU":
			return "america" // SLUS
		case "LP":
			return "japan" // SLPS
		}
		// SLES = Europe, SLUS = America, SLPS = Japan, SLPM = Japan
		if len(prefix) >= 2 {
			switch prefix[1] {
			case 'C':
				return "europe" // SCES
			case 'L':
				switch prefix[2] {
				case 'E':
					return "europe" // SLES
				case 'U':
					return "america" // SLUS
				case 'P':
					return "japan" // SLPS, SLPM
				}
			}
		}
	case "E":
		return "europe" // ES, EP, EL
	case "U":
		return "america" // US
	case "J":
		return "japan" // JP
	case "H":
		return "asia" // HK, HP
	case "K":
		return "asia" // KP, KA
	}
	return "unknown"
}

// DetectDiscType determines if a file is a PS1 or PS2 disc image.
// Returns "ps1", "ps2", or an error if it can't determine.
func DetectDiscType(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".cue":
		// Could be PS1 or PS2 CD
		tracks, err := ParseCUE(path)
		if err != nil {
			return "", err
		}

		// If SYSTEM.CNF can be parsed from the data track, it's PS2
		for i := range tracks {
			if strings.Contains(tracks[i].Mode, "MODE") {
				f, err := os.Open(tracks[i].File)
				if err != nil {
					continue
				}
				cnf, err := readSystemCNFFromBin(f, &tracks[i])
				f.Close()
				if err == nil && len(cnf) > 0 {
					cnfMap := parseSystemCNF(cnf)
					if _, ok := cnfMap["BOOT2"]; ok {
						return "ps2", nil
					}
				}
			}
		}
		// Default: PS1
		return "ps1", nil

	case ".iso":
		// PS2 ISO — read SYSTEM.CNF to verify
		f, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("open: %w", err)
		}
		defer f.Close()

		cnfData, err := readISOFile(f, "SYSTEM.CNF")
		if err == nil && len(cnfData) > 0 {
			cnfMap := parseSystemCNF(cnfData)
			if _, ok := cnfMap["BOOT2"]; ok {
				return "ps2", nil
			}
		}
		return "ps2", nil // Assume PS2 for ISOs

	case ".bin":
		// Need to check if there's a matching .cue
		cuePath := strings.TrimSuffix(path, ".bin") + ".cue"
		cuePathCI := strings.TrimSuffix(path, ".BIN") + ".cue"
		if _, err := os.Stat(cuePath); err == nil {
			return DetectDiscType(cuePath)
		}
		if _, err := os.Stat(cuePathCI); err == nil {
			return DetectDiscType(cuePathCI)
		}
		// Try to detect from content
		return detectFromBinContent(path)

	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// detectFromBinContent tries to determine if a .bin file is PS1 or PS2.
func detectFromBinContent(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Read first sector
	buf := make([]byte, 2352)
	n, err := f.Read(buf)
	if err != nil || n < 2048 {
		return "", fmt.Errorf("cannot read first sector")
	}

	// Check for PS1 header at offset 0 in the data area
	// PS1 discs have "Licensed  by" or "PS-X EXE" signature
	dataArea := buf
	if n >= 2352 {
		dataArea = buf[24:] // Skip sync + header for Mode 2
	}
	text := string(dataArea[:256])

	if strings.Contains(text, "PlayStation") || strings.Contains(text, "PS-X") {
		return "ps1", nil
	}

	// Check for PS2 SYSTEM.CNF
	for sector := 0; sector < 16; sector++ {
		offset := int64(sector * 2352)
		f.Seek(offset, io.SeekStart)
		n, _ := f.Read(buf)
		if n < 2048 {
			continue
		}
		dataArea = buf[24:]
		text = string(dataArea[:2048])
		if strings.Contains(strings.ToUpper(text), "BOOT2") {
			return "ps2", nil
		}
	}

	return "", fmt.Errorf("cannot determine disc type from .bin content")
}
