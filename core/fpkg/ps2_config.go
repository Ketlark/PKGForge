package fpkg

import (
	"fmt"
	"strings"
)

// ps2PatchID returns the hyphenated PS2 game ID used in patch filenames
// (e.g. "SCES-51719_cli.conf").
func ps2PatchID(titleID string) string {
	if id := normalizeGameID(titleID); id != "" {
		return id
	}
	return strings.ToUpper(strings.TrimSpace(titleID))
}

// PS2EmuConfig generates the default config-emu-ps4.txt for a PS2 fPKG.
func PS2EmuConfig(opts PS2FPKGOptions, titleID string, discCount int) string {
	return ps2RuntimeEmuConfig(opts, titleID, discCount, "")
}

func ps2RuntimeEmuConfig(opts PS2FPKGOptions, titleID string, discCount int, template string) string {
	if discCount < 1 {
		discCount = 1
	}
	patchID := ps2PatchID(titleID)
	opts = applyPS2GameDefaults(opts, patchID)
	retail := ps2ProfileUsesRetailLauncher(titleID)

	prefix := ps1ConfigTemplatePrefix([]byte(template))
	if len(prefix) == 0 {
		prefix = ps2DefaultConfigPrefix(retail)
	} else {
		prefix = ps2EnsureRequiredConfigKeys(prefix, retail)
	}

	lines := append([]string{}, prefix...)
	lines = ps2ForceConfigLine(lines, fmt.Sprintf("--ps2-title-id=%s", patchID))
	lines = ps2ForceConfigLine(lines, fmt.Sprintf("--max-disc-num=%d", discCount))
	if !retail {
		lines = ps2ForceConfigLine(lines, "--trophy-support=0")
		lines = ps2ForceConfigLine(lines, "--host-trophy-support=0")
	}

	switch strings.ToLower(opts.Uprender) {
	case "2x2":
		lines = ps2ForceConfigLine(lines, "--gs-uprender=2x2")
		lines = ps2ForceConfigLine(lines, "--gs-upscale=EdgeSmooth")
	case "4x":
		lines = ps2ForceConfigLine(lines, "--gs-uprender=4x")
		lines = ps2ForceConfigLine(lines, "--gs-upscale=EdgeSmooth")
	case "off":
		lines = ps2ForceConfigLine(lines, "--gs-uprender=none")
		lines = ps2ForceConfigLine(lines, "--gs-upscale=none")
	default:
		if !ps1ConfigHasKey(lines, "--gs-uprender") {
			lines = append(lines, "--gs-uprender=2x2", "--gs-upscale=EdgeSmooth")
		}
	}

	switch strings.ToLower(opts.DisplayMode) {
	case "4:3":
		lines = ps2ForceConfigLine(lines, "--host-display-mode=4:3")
	case "16:9":
		lines = ps2ForceConfigLine(lines, "--host-display-mode=16:9")
	default:
		if retail {
			if !ps1ConfigHasKey(lines, "--host-display-mode") {
				lines = ps2ForceConfigLine(lines, "--host-display-mode=full")
			}
		} else {
			lines = ps2ForceConfigLine(lines, "--host-display-mode=full")
		}
	}

	for _, line := range ps2ProfileEmuFlags(patchID) {
		if isJakOnlyEmuFlag(line) && !ps2ProfileUsesJakEmu(titleID) {
			continue
		}
		lines = ps2AppendGeneratedConfigLine(lines, prefix, line)
	}

	return strings.Join(lines, "\n") + "\n"
}

func ps2DefaultConfigPrefix(retail bool) []string {
	lines := []string{
		"# PKG Forge PS2 emulator defaults",
		"",
		`--path-snaps="/tmp/snapshots"`,
		`--path-recordings="/tmp/recordings"`,
		`--path-vmc="/tmp/vmc"`,
		`--path-emulog="/tmp/recordings"`,
		`--config-local-lua=""`,
		`--load-tooling-lua=0`,
		`--path-patches="/app0/patches"`,
		`--path-featuredata="/app0/feature_data"`,
		`--ps2-lang=system`,
		`--host-audio=1`,
		`--rom="PS20220WD20050620.crack"`,
		`--verbose-cdvd-reads=0`,
		`--host-osd=0`,
		`--playgo-disc-per-chunk=0`,
	}
	if retail {
		lines = append(lines,
			`--path-trophydata="/app0/trophy_data"`,
			`--trophy-support=1`,
		)
	} else {
		lines = append(lines,
			`--trophy-support=0`,
			`--host-trophy-support=0`,
		)
	}
	return lines
}

func ps2EnsureRequiredConfigKeys(lines []string, retail bool) []string {
	out := append([]string{}, lines...)
	for _, line := range ps2DefaultConfigPrefix(retail) {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		out = ps2AppendGeneratedConfigLine(out, lines, line)
	}
	return out
}

func ps2AppendGeneratedConfigLine(lines, prefix []string, line string) []string {
	key := ps1ConfigKey(line)
	if ps1ConfigHasKey(prefix, key) || ps1ConfigHasKey(lines, key) {
		return lines
	}
	return append(lines, line)
}

func shouldSkipPS2EmulatorSidecar(rel string, retailLauncher bool) bool {
	switch strings.ToLower(filepathToSlash(rel)) {
	case "sce_sys/param.sfo":
		return true
	case "config-emu-ps4.txt":
		// Dumped emulator configs are title-specific (wrong ps2-title-id, max-disc-num, trophies).
		return true
	case "sce_discmap_patch.plt":
		// Patch discmap from the source Classic title, not portable to other games.
		return true
	case "formatted.card":
		return !retailLauncher
	default:
		return false
	}
}

func ps2ForceConfigLine(lines []string, line string) []string {
	key := ps1ConfigKey(line)
	if key == "" {
		return lines
	}
	out := lines[:0]
	for _, existing := range lines {
		if ps1ConfigKey(existing) == key {
			continue
		}
		out = append(out, existing)
	}
	return append(out, line)
}

func filepathToSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func ps2ProfileUsesJakEmu(titleID string) bool {
	profiles, err := loadPS2Profiles()
	if err != nil {
		return true
	}
	profile, ok := profiles[ps2PatchID(titleID)]
	if !ok || profile.Emulator == "" {
		return true
	}
	return strings.EqualFold(profile.Emulator, string(EmuJakV2))
}

func isJakOnlyEmuFlag(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "--gs-adaptive-frameskip")
}
