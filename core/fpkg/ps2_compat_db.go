package fpkg

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed ps2configs/*
var ps2CompatFS embed.FS

const ps2CompatRoot = "ps2configs"

type ps2GameProfile struct {
	Uprender         string   `json:"uprender"`
	CLI              string   `json:"cli"` // optional alternate cli filename, e.g. "SLUS-21004_cli.conf"
	DisplayMode      string   `json:"displayMode"`
	SkipUniversalCLI bool     `json:"skipUniversalCLI"`
	Emulator         string   `json:"emulator"` // launcher: jakv2, rogue, siren
	EmuCore          string   `json:"emuCore"`  // optional PS2 core overlay: siren, rogue, …
	RetailLauncher   bool     `json:"retailLauncher"`
	Emu              []string `json:"emu"` // extra config-emu-ps4.txt flags (Jak-only flags ignored on Siren)
}

// PS2ProfileHint holds per-game defaults from ps2configs/profiles.json.
type PS2ProfileHint struct {
	Emulator    string `json:"emulator,omitempty"`
	EmuCore     string `json:"emuCore,omitempty"`
	DisplayMode string `json:"displayMode,omitempty"`
	Uprender    string `json:"uprender,omitempty"`
}

var (
	ps2Profiles     map[string]ps2GameProfile
	ps2ProfilesOnce sync.Once
	ps2ProfilesErr  error
)

func loadPS2Profiles() (map[string]ps2GameProfile, error) {
	ps2ProfilesOnce.Do(func() {
		data, err := ps2CompatFS.ReadFile(ps2CompatRoot + "/profiles.json")
		if err != nil {
			ps2ProfilesErr = fmt.Errorf("read ps2 profiles: %w", err)
			return
		}
		ps2Profiles = make(map[string]ps2GameProfile)
		if err := json.Unmarshal(data, &ps2Profiles); err != nil {
			ps2ProfilesErr = fmt.Errorf("parse ps2 profiles: %w", err)
		}
	})
	return ps2Profiles, ps2ProfilesErr
}

func readPS2CompatFile(name string) ([]byte, bool) {
	data, err := ps2CompatFS.ReadFile(ps2CompatRoot + "/" + name)
	if err != nil {
		return nil, false
	}
	return data, true
}

// PS2CompatCLI returns a merged cli.conf patch for a game ID, if bundled.
func PS2CompatCLI(patchID string) (string, bool) {
	var parts []string
	skipUniversal := false
	if profiles, err := loadPS2Profiles(); err == nil {
		if profile, ok := profiles[patchID]; ok {
			skipUniversal = profile.SkipUniversalCLI
		}
	}
	if !skipUniversal {
		if universal, ok := readPS2CompatFile("_universal_cli.conf"); ok {
			parts = append(parts, strings.TrimSpace(string(universal)))
		}
	}

	cliName := patchID + "_cli.conf"
	if profiles, err := loadPS2Profiles(); err == nil {
		if profile, ok := profiles[patchID]; ok && profile.CLI != "" {
			cliName = profile.CLI
		}
	}
	game, ok := readPS2CompatFile(cliName)
	if !ok {
		if len(parts) == 0 {
			return "", false
		}
		return strings.Join(parts, "\n") + "\n", true
	}
	parts = append(parts, strings.TrimSpace(string(game)))
	return strings.Join(parts, "\n") + "\n", true
}

// PS2CompatFeatureLUA returns a bundled feature_data lua script for a game ID.
func PS2CompatFeatureLUA(patchID string) (string, bool) {
	data, ok := readPS2CompatFile(patchID + "_features.lua")
	if !ok {
		return "", false
	}
	return string(data), true
}

// PS2CompatPackageConf returns the default package-ps4.conf sidecar.
func PS2CompatPackageConf() string {
	if data, ok := readPS2CompatFile("package-ps4.conf"); ok {
		return string(data)
	}
	return "--scale=6\n"
}

// PS2ProfileForGame returns bundled profile defaults for a title ID, if any.
func PS2ProfileForGame(titleID string) (*PS2ProfileHint, bool) {
	patchID := ps2PatchID(titleID)
	profiles, err := loadPS2Profiles()
	if err != nil {
		return nil, false
	}
	profile, ok := profiles[patchID]
	if !ok {
		return nil, false
	}
	launch, core := resolvePS2ProfileEmulators(profile)
	hint := &PS2ProfileHint{
		Emulator:    string(launch),
		EmuCore:     string(core),
		DisplayMode: profile.DisplayMode,
		Uprender:    profile.Uprender,
	}
	if hint.Emulator == "" && hint.EmuCore == "" && hint.DisplayMode == "" && hint.Uprender == "" {
		return nil, false
	}
	return hint, true
}

// applyPS2GameDefaults adjusts build options using ps2configs/profiles.json.
func applyPS2GameDefaults(opts PS2FPKGOptions, patchID string) PS2FPKGOptions {
	profiles, err := loadPS2Profiles()
	if err != nil {
		return opts
	}
	profile, ok := profiles[patchID]
	if !ok {
		return opts
	}
	if profile.Uprender != "" {
		opts.Uprender = profile.Uprender
	}
	if profile.DisplayMode != "" {
		opts.DisplayMode = profile.DisplayMode
	}
	if profile.Emulator != "" || profile.EmuCore != "" {
		launch, core := resolvePS2ProfileEmulators(profile)
		opts.Emulator = launch
		opts.EmuCore = core
	}
	return opts
}

func resolvePS2ProfileEmulators(profile ps2GameProfile) (launch, core PS2EmulatorType) {
	if profile.Emulator == "" && profile.EmuCore == "" {
		return EmuJakV2, EmuJakV2
	}
	launch = normalizePS2ProfileEmulator(profile.Emulator)
	core = launch
	if profile.EmuCore != "" {
		core = normalizePS2ProfileEmulator(profile.EmuCore)
	} else if launch == EmuSiren && !profile.RetailLauncher {
		// Non-retail Siren selection without sidecars: fall back to Jak shell + lua/cli tuning.
		launch = EmuJakV2
		core = EmuJakV2
	}
	return launch, core
}

func ps2ProfileForPatchID(patchID string) (ps2GameProfile, bool) {
	profiles, err := loadPS2Profiles()
	if err != nil {
		return ps2GameProfile{}, false
	}
	profile, ok := profiles[patchID]
	return profile, ok
}

func ps2ProfileUsesRetailLauncher(titleID string) bool {
	profile, ok := ps2ProfileForPatchID(ps2PatchID(titleID))
	if !ok {
		return false
	}
	return profile.RetailLauncher && strings.EqualFold(profile.Emulator, string(EmuSiren))
}

func normalizePS2ProfileEmulator(name string) PS2EmulatorType {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case string(EmuSiren):
		return EmuSiren
	case string(EmuRogue):
		return EmuRogue
	default:
		return EmuJakV2
	}
}

// ps2ProfileEmuFlags returns extra config-emu-ps4.txt lines from profiles.json.
func ps2ProfileEmuFlags(patchID string) []string {
	profiles, err := loadPS2Profiles()
	if err != nil {
		return nil
	}
	profile, ok := profiles[patchID]
	if !ok || len(profile.Emu) == 0 {
		return nil
	}
	return append([]string{}, profile.Emu...)
}
