package fpkg

// This file implements project directory setup and file assembly for fPKG creation.
//
// The project builder takes emulator files (eboot.bin, libc.prx, etc.) and
// game-specific files (disc images, configs, assets) and assembles them into
// the file map that gets fed into the PFS → PKG pipeline.
//
// Emulator files are extracted from the bundled encrypted archive on first use.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// Emulator file management
// ---------------------------------------------------------------------------

// EmulatorSet represents the collection of emulator binary files needed for an fPKG.
type EmulatorSet struct {
	// Name is the emulator folder name (e.g. "ps1hd", "Jak v2", "Rogue v1").
	Name string

	// Files maps virtual path → local file path (or embedded content).
	// Virtual paths are relative to the PFS root.
	Files map[string]string
}

// DefaultPS1EmulatorSet returns the default PS1 emulator file set.
// Uses the archive folder name "ps1hd" and the real file paths from
// the PS-Classics-fPKG-Builder archive.
func DefaultPS1EmulatorSet() *EmulatorSet {
	return &EmulatorSet{
		Name: "ps1hd",
		Files: map[string]string{
			"eboot.bin":                       "eboot.bin",
			"sce_module/libc.prx":             "sce_module/libc.prx",
			"sce_module/libSceFios2.prx":      "sce_module/libSceFios2.prx",
			"sce_module/libSceNpToolkit2.prx": "sce_module/libSceNpToolkit2.prx",
		},
	}
}

// DefaultPS2EmulatorSets returns the available PS2 emulator file sets.
// Uses archive folder names ("Jak v2", "Rogue v1") and real file paths.
func DefaultPS2EmulatorSets() map[PS2EmulatorType]*EmulatorSet {
	jakFiles := map[string]string{
		"eboot.bin":                  "eboot.bin",
		"sce_module/libc.prx":        "sce_module/libc.prx",
		"sce_module/libSceFios2.prx": "sce_module/libSceFios2.prx",
		"ps2-emu-compiler.self":      "ps2-emu-compiler.self",
		"PS20220WD20050620.crack":    "PS20220WD20050620.crack",
	}

	return map[PS2EmulatorType]*EmulatorSet{
		EmuJakV2: {
			Name:  "Jak v2",
			Files: jakFiles,
		},
		EmuRogue: {
			Name:  "Rogue v1",
			Files: jakFiles, // same file layout, different binary content
		},
		EmuSiren: {
			Name:  "Siren v2",
			Files: jakFiles,
		},
	}
}

// LoadEmulatorFiles loads emulator binary files from a directory.
// The directory should contain the emulator files for a specific emulator type.
// Returns a map of virtual path → file content.
func LoadEmulatorFiles(emuDir string, emuSet *EmulatorSet) (map[string][]byte, error) {
	files := make(map[string][]byte)

	for virtualPath, localName := range emuSet.Files {
		localPath := filepath.Join(emuDir, emuSet.Name, localName)

		data, err := os.ReadFile(localPath)
		if err != nil {
			// Try without the emulator name subdirectory
			localPath = filepath.Join(emuDir, localName)
			data, err = os.ReadFile(localPath)
			if err != nil {
				return nil, fmt.Errorf("missing emulator file %s (looked in %s): %w",
					localName, emuDir, err)
			}
		}

		files[virtualPath] = data
	}

	return files, nil
}

// ps2CoreOverlayPaths are replaced from EmuCore when using a hybrid launcher/core layout.
var ps2CoreOverlayPaths = []string{
	"ps2-emu-compiler.self",
	"PS20220WD20050620.crack",
	"sce_discmap.plt",
}

func overlayPS2EmulatorCore(files map[string][]byte, emuDir string, emuSets map[PS2EmulatorType]*EmulatorSet, coreEmu PS2EmulatorType) error {
	coreSet, ok := emuSets[coreEmu]
	if !ok {
		return fmt.Errorf("unknown emulator core %q", coreEmu)
	}
	for _, virtualPath := range ps2CoreOverlayPaths {
		localName := coreSet.Files[virtualPath]
		if localName == "" {
			continue
		}
		localPath := filepath.Join(emuDir, coreSet.Name, localName)
		data, err := os.ReadFile(localPath)
		if err != nil {
			localPath = filepath.Join(emuDir, localName)
			data, err = os.ReadFile(localPath)
			if err != nil {
				return fmt.Errorf("missing core file %s: %w", localName, err)
			}
		}
		files[virtualPath] = data
	}
	return nil
}

// LoadEmulatorDirectoryFiles loads every regular file from an emulator folder.
// It is used as a best-effort companion to the required file map so complete
// ps1hd/ps2 emulator directories can carry their sce_sys sidecar files too.
func LoadEmulatorDirectoryFiles(emuDir string, emuSet *EmulatorSet) (map[string][]byte, error) {
	root := filepath.Join(emuDir, emuSet.Name)
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		root = emuDir
	}

	files := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&fs.ModeType != 0 {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.Contains(rel, "..") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// ---------------------------------------------------------------------------
// Lua include files
// ---------------------------------------------------------------------------

// LuaIncludeFiles returns the standard lua_include files for PS2 fPKG.
// These are small Lua scripts that define aliases and helpers for the PS2 emulator.
// They are typically shipped with PS2-FPKG tools.
var LuaIncludeFiles = map[string]string{
	"lua_include/ee-cpr0-alias.lua": `-- EE Coprocessor 0 (COP0) Register Aliases
-- EE CPR0 aliases for Lua scripting

EE_COP0_Index      = 0
EE_COP0_Random     = 1
EE_COP0_EntryLo0   = 2
EE_COP0_EntryLo1   = 3
EE_COP0_Context    = 4
EE_COP0_PageMask   = 5
EE_COP0_Wired      = 6
EE_COP0_BadVAddr   = 8
EE_COP0_Count      = 9
EE_COP0_EntryHi    = 10
EE_COP0_Compare    = 11
EE_COP0_Status     = 12
EE_COP0_Cause      = 13
EE_COP0_EPC        = 14
EE_COP0_PRid       = 15
EE_COP0_Config     = 16
EE_COP0_XContext   = 20
EE_COP0_TagLo      = 28
EE_COP0_TagHi      = 29
EE_COP0_ErrorEPC   = 30
`,

	"lua_include/ee-gpr-alias.lua": `-- EE General Purpose Register Aliases
-- EE GPR aliases for Lua scripting

EE_R0  = 0   -- always 0
EE_AT  = 1   -- assembler temporary
EE_V0  = 2   -- return value
EE_V1  = 3
EE_A0  = 4   -- function arguments
EE_A1  = 5
EE_A2  = 6
EE_A3  = 7
EE_T0  = 8   -- temporaries
EE_T1  = 9
EE_T2  = 10
EE_T3  = 11
EE_T4  = 12
EE_T5  = 13
EE_T6  = 14
EE_T7  = 15
EE_S0  = 16  -- saved registers
EE_S1  = 17
EE_S2  = 18
EE_S3  = 19
EE_S4  = 20
EE_S5  = 21
EE_S6  = 22
EE_S7  = 23
EE_T8  = 24
EE_T9  = 25
EE_K0  = 26  -- kernel
EE_K1  = 27
EE_GP  = 28  -- global pointer
EE_SP  = 29  -- stack pointer
EE_FP  = 30  -- frame pointer
EE_RA  = 31  -- return address
`,

	"lua_include/ee-hwaddr.lua": `-- EE Hardware Address Definitions

EE_BASE       = 0x70000000
EE_TIMER      = 0x70000000
EE_INTC       = 0x70001000
EE_DMAC       = 0x70002000
EE_VU0        = 0x70004000
EE_VIF0       = 0x70003800
EE_VIF1       = 0x70003C00
EE_GIF        = 0x70003000
EE_IPU        = 0x70005000
EE_SIO        = 0x7000D000
EE_SCR        = 0x7000F000
EE_RAM_SIZE   = 0x02000000
`,

	"lua_include/language.lua": `-- Language codes for PS2 emulator configuration

LANG_JAPANESE       = 0
LANG_ENGLISH        = 1
LANG_FRENCH         = 2
LANG_SPANISH        = 3
LANG_GERMAN         = 4
LANG_ITALIAN        = 5
LANG_DUTCH          = 6
LANG_PORTUGUESE     = 7
LANG_RUSSIAN        = 8
LANG_KOREAN         = 9
LANG_CHINESE_T      = 10
LANG_CHINESE_S      = 11
LANG_FINNISH        = 12
LANG_SWEDISH        = 13
LANG_DANISH         = 14
LANG_NORWEGIAN      = 15
LANG_POLISH         = 16
LANG_PORTUGUESE_BR  = 17
LANG_ENGLISH_GB     = 18
LANG_TURKISH        = 19
`,

	"lua_include/pad-and-key.lua": `-- Pad and Key definitions for PS2 emulator

PAD_L2       = 0x0001
PAD_R2       = 0x0002
PAD_L1       = 0x0004
PAD_R1       = 0x0008
PAD_TRIANGLE = 0x0010
PAD_CIRCLE   = 0x0020
PAD_CROSS    = 0x0040
PAD_SQUARE   = 0x0080
PAD_SELECT   = 0x0100
PAD_L3       = 0x0200
PAD_R3       = 0x0400
PAD_START    = 0x0800
PAD_UP       = 0x1000
PAD_RIGHT    = 0x2000
PAD_DOWN     = 0x4000
PAD_LEFT     = 0x8000
`,

	"lua_include/utils.lua": `-- Utility functions for PS2 emulator Lua scripting

function printf(fmt, ...)
    print(string.format(fmt, ...))
end

function hex(val)
    return string.format("0x%08X", val)
end

function read8(addr)
    return eeObj.ReadMem8(addr)
end

function read16(addr)
    return eeObj.ReadMem16(addr)
end

function read32(addr)
    return eeObj.ReadMem32(addr)
end

function write8(addr, val)
    eeObj.WriteMem8(addr, val)
end

function write16(addr, val)
    eeObj.WriteMem16(addr, val)
end

function write32(addr, val)
    eeObj.WriteMem32(addr, val)
end
`,
}

// GetLuaIncludeData returns the lua_include file contents for PS2 fPKGs.
// It prefers the full lua_include directory from the assets cache, then falls
// back to embedded stubs for any files that are still missing.
func GetLuaIncludeData(cacheDir string) map[string][]byte {
	result := make(map[string][]byte)
	if cacheDir != "" {
		luaRoot := filepath.Join(cacheDir, "lua_include")
		_ = filepath.WalkDir(luaRoot, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			if entry.Type()&fs.ModeType != 0 || strings.HasPrefix(entry.Name(), ".") {
				return nil
			}
			rel, err := filepath.Rel(luaRoot, path)
			if err != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if strings.Contains(rel, "..") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			result["lua_include/"+rel] = data
			return nil
		})
	}
	for virtualPath, content := range LuaIncludeFiles {
		if _, exists := result[virtualPath]; !exists {
			result[virtualPath] = []byte(content)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// PS1 game ID database (embedded from ps1.idlst)
// ---------------------------------------------------------------------------

// PS1GameEntry represents a game in the PS1 ID database.
type PS1GameEntry struct {
	ID    string // e.g. "SCUS-94244"
	Title string // e.g. "Crash Bandicoot"
}

// LookupPS1GameByID searches for a game title by its ID.
// Returns the game entry and true if found.
func LookupPS1GameByID(gameID string, db []PS1GameEntry) (PS1GameEntry, bool) {
	for _, entry := range db {
		if entry.ID == gameID {
			return entry, true
		}
	}
	return PS1GameEntry{}, false
}

// LoadPS1IDDatabase loads the PS1 game ID database from an idlst file.
// Format: "<ID> \"<Title>\"" (one per line).
func LoadPS1IDDatabase(path string) ([]PS1GameEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read idlst: %w", err)
	}

	var entries []PS1GameEntry
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse: SCUS-94244 "Crash Bandicoot"
		spaceIdx := strings.Index(line, " ")
		if spaceIdx < 0 {
			continue
		}

		id := line[:spaceIdx]
		title := strings.Trim(line[spaceIdx:], " \"")

		entries = append(entries, PS1GameEntry{ID: id, Title: title})
	}

	return entries, nil
}

// ---------------------------------------------------------------------------
// PS2 game ID database (from CSV)
// ---------------------------------------------------------------------------

// PS2GameEntry represents a game in the PS2 ID database.
type PS2GameEntry struct {
	ID    string // e.g. "SLUS-20062"
	Title string // e.g. "Grand Theft Auto III"
}

// LoadPS2IDDatabase loads the PS2 game ID database from a CSV file.
// Format: GameID;Name;... (semicolon-separated, first two fields used).
func LoadPS2IDDatabase(path string) ([]PS2GameEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}

	var entries []PS2GameEntry
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || i == 0 { // skip header
			continue
		}

		fields := strings.SplitN(line, ";", 3)
		if len(fields) < 2 {
			continue
		}

		id := strings.TrimSpace(fields[0])
		title := strings.TrimSpace(fields[1])

		if id == "" || title == "" {
			continue
		}

		entries = append(entries, PS2GameEntry{ID: id, Title: title})
	}

	return entries, nil
}

// LookupPS2GameByID searches for a game title by its ID.
func LookupPS2GameByID(gameID string, db []PS2GameEntry) (PS2GameEntry, bool) {
	for _, entry := range db {
		if entry.ID == gameID {
			return entry, true
		}
	}
	return PS2GameEntry{}, false
}
