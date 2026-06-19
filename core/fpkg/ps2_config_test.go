package fpkg

import (
	"strings"
	"testing"
)

// Fixture title IDs backed by ps2configs/* — used only as bundled data samples.
const (
	profiledTitleID = "SCES-51719"
	aliasedTitleID  = "SCUS-97328"
)

func TestPS2EmuConfigContainsRuntimeKeys(t *testing.T) {
	cfg := PS2EmuConfig(PS2FPKGOptions{
		ISOPaths: []string{"disc.iso"},
	}, "SLUS-20062", 1)

	for _, want := range []string{
		"--ps2-title-id=SLUS-20062",
		"--max-disc-num=1",
		`--rom="PS20220WD20050620.crack"`,
		"--host-audio=1",
		"--gs-uprender=2x2",
		"--host-display-mode=full",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config missing %q:\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "--uprender=") {
		t.Fatalf("config uses obsolete flag --uprender:\n%s", cfg)
	}
}

func TestPS2EmuConfigAppliesBundledProfile(t *testing.T) {
	cfg := PS2EmuConfig(PS2FPKGOptions{
		ISOPaths: []string{"disc.iso"},
	}, profiledTitleID, 1)

	if !strings.Contains(cfg, "--gs-uprender=2x2") {
		t.Fatalf("retail Siren profile should keep 2x2 uprender:\n%s", cfg)
	}
	if !strings.Contains(cfg, "--host-display-mode=16:9") {
		t.Fatalf("profile should set host display mode:\n%s", cfg)
	}
	if !strings.Contains(cfg, "--trophy-support=1") {
		t.Fatalf("retail Siren profile should keep trophy support:\n%s", cfg)
	}
}

func TestPS2EmuConfigProfileSelectsSirenEmulator(t *testing.T) {
	opts := PS2FPKGOptions{ISOPaths: []string{"disc.iso"}}
	opts = applyPS2GameDefaults(opts, profiledTitleID)
	if opts.Emulator != EmuSiren {
		t.Fatalf("profile launcher = %q, want %q", opts.Emulator, EmuSiren)
	}
	if opts.EmuCore != EmuSiren {
		t.Fatalf("profile core = %q, want %q", opts.EmuCore, EmuSiren)
	}
}

func TestPS2ProfileSirenAliasUsesJakLauncherOnly(t *testing.T) {
	profile := ps2GameProfile{Emulator: "siren"}
	launch, core := resolvePS2ProfileEmulators(profile)
	if launch != EmuJakV2 || core != EmuJakV2 {
		t.Fatalf("siren alias = (%q,%q), want (jakv2,jakv2)", launch, core)
	}

	retail := ps2GameProfile{Emulator: "siren", RetailLauncher: true}
	launch, core = resolvePS2ProfileEmulators(retail)
	if launch != EmuSiren || core != EmuSiren {
		t.Fatalf("retail siren = (%q,%q), want (siren,siren)", launch, core)
	}
}

func TestPS2HybridEmulatorOverlayUsesSirenCore(t *testing.T) {
	emuRoot := "/tmp/emus-ref/emus"
	launchSet := DefaultPS2EmulatorSets()[EmuJakV2]
	files, err := LoadEmulatorFiles(emuRoot, launchSet)
	if err != nil {
		t.Skip("reference emus not available:", err)
	}
	jakCompiler := append([]byte(nil), files["ps2-emu-compiler.self"]...)
	if err := overlayPS2EmulatorCore(files, emuRoot, DefaultPS2EmulatorSets(), EmuSiren); err != nil {
		t.Skip("siren emulator not available:", err)
	}
	files2, err := LoadEmulatorFiles(emuRoot, DefaultPS2EmulatorSets()[EmuSiren])
	if err != nil {
		t.Fatalf("load siren set: %v", err)
	}
	sirenCompiler := files2["ps2-emu-compiler.self"]
	if len(jakCompiler) == 0 || len(sirenCompiler) == 0 {
		t.Fatal("missing compiler binaries")
	}
	if string(jakCompiler) == string(sirenCompiler) {
		t.Fatal("expected hybrid overlay to replace compiler with siren build")
	}
	if string(files["ps2-emu-compiler.self"]) != string(sirenCompiler) {
		t.Fatal("hybrid project should use siren compiler")
	}
	jakEboot, err := LoadEmulatorFiles(emuRoot, launchSet)
	if err != nil {
		t.Fatalf("reload jak: %v", err)
	}
	if string(files["eboot.bin"]) != string(jakEboot["eboot.bin"]) {
		t.Fatal("hybrid project should keep jak eboot")
	}
}

func TestPS2ProfileForGameReturnsBundledDefaults(t *testing.T) {
	hint, ok := PS2ProfileForGame(profiledTitleID)
	if !ok || hint == nil {
		t.Fatal("expected profile for bundled title")
	}
	if hint.Emulator != "siren" {
		t.Fatalf("emulator = %q, want siren", hint.Emulator)
	}
	if hint.EmuCore != "siren" {
		t.Fatalf("emuCore = %q, want siren", hint.EmuCore)
	}
	if hint.DisplayMode != "16:9" {
		t.Fatalf("displayMode = %q, want 16:9", hint.DisplayMode)
	}
}

func TestPS2ProfileForGameUnknownTitle(t *testing.T) {
	if _, ok := PS2ProfileForGame("SLUS-20062"); ok {
		t.Fatal("unexpected profile for unknown title")
	}
}

func TestPS2EmuConfigRespectsExplicitOptionsOverProfile(t *testing.T) {
	// Unknown title: builder options win; no bundled profile overrides.
	cfg := PS2EmuConfig(PS2FPKGOptions{
		Uprender:    "off",
		DisplayMode: "16:9",
	}, "SLUS-20062", 2)

	if !strings.Contains(cfg, "--gs-uprender=none") {
		t.Fatalf("expected gs-uprender=none:\n%s", cfg)
	}
	if !strings.Contains(cfg, "--host-display-mode=16:9") {
		t.Fatalf("expected host-display-mode=16:9:\n%s", cfg)
	}
	if !strings.Contains(cfg, "--max-disc-num=2") {
		t.Fatalf("expected max-disc-num=2:\n%s", cfg)
	}
}

func TestPS2EmuConfigMergesEmulatorTemplate(t *testing.T) {
	template := "# emulator template\n--host-audio=0\n--custom-flag=1\n"
	cfg := ps2RuntimeEmuConfig(PS2FPKGOptions{}, profiledTitleID, 1, template)

	if !strings.Contains(cfg, "--custom-flag=1") {
		t.Fatalf("template flag missing:\n%s", cfg)
	}
	if strings.Contains(cfg, "--host-audio=1") {
		t.Fatalf("should not duplicate host-audio when template sets it:\n%s", cfg)
	}
	if !strings.Contains(cfg, "--ps2-title-id=SCES-51719") {
		t.Fatalf("generated title id missing:\n%s", cfg)
	}
	if !strings.Contains(cfg, `--rom="PS20220WD20050620.crack"`) {
		t.Fatalf("required rom path missing from merged template:\n%s", cfg)
	}
}

func TestPS2CompatCLILoadsBundledPatch(t *testing.T) {
	cli, ok := PS2CompatCLI(profiledTitleID)
	if !ok {
		t.Fatal("expected bundled cli patch")
	}
	if strings.TrimSpace(cli) == "" {
		t.Fatal("bundled cli patch is empty")
	}
}

func TestPS2CompatCLISkipsUniversalWhenProfileRequires(t *testing.T) {
	cli, ok := PS2CompatCLI(profiledTitleID)
	if !ok {
		t.Fatal("expected bundled cli patch")
	}
	if strings.Contains(cli, "--vu1-jr-cache-policy=newprog") {
		t.Fatalf("profile skipUniversalCLI should omit universal defaults:\n%s", cli)
	}
}

func TestPS2CompatFeatureLUALoadsBundledScript(t *testing.T) {
	lua, ok := PS2CompatFeatureLUA(profiledTitleID)
	if !ok {
		t.Fatal("expected bundled feature lua")
	}
	if strings.TrimSpace(lua) == "" {
		t.Fatal("bundled feature lua is empty")
	}
}

func TestPS2CompatCLIResolvesProfileAlias(t *testing.T) {
	cli, ok := PS2CompatCLI(aliasedTitleID)
	if !ok {
		t.Fatal("expected aliased cli patch")
	}
	if !strings.Contains(cli, "--gs-uprender=none") {
		t.Fatalf("aliased cli patch missing expected flag:\n%s", cli)
	}
}

func TestPS2PatchIDUsesHyphenatedGameID(t *testing.T) {
	if got := ps2PatchID("SCES51719"); got != "SCES-51719" {
		t.Fatalf("ps2PatchID = %q", got)
	}
}

func TestGetLuaIncludeDataLoadsCachedDirectory(t *testing.T) {
	cacheDir, err := AssetsCacheDir()
	if err != nil {
		t.Fatalf("AssetsCacheDir: %v", err)
	}
	if !HasCachedAssets() {
		t.Skip("no cached assets")
	}
	files := GetLuaIncludeData(cacheDir)
	for _, required := range []string{
		"lua_include/MipsInsn.lua",
		"lua_include/sprite.lua",
		"lua_include/utils.lua",
	} {
		if _, ok := files[required]; !ok {
			t.Fatalf("missing cached lua include %s (got %d files)", required, len(files))
		}
	}
}

func TestShouldSkipPS2EmulatorSidecar(t *testing.T) {
	for _, path := range []string{
		"config-emu-ps4.txt",
		"sce_discmap_patch.plt",
		"formatted.card",
	} {
		if !shouldSkipPS2EmulatorSidecar(path, false) {
			t.Fatalf("expected %q to be skipped for homebrew", path)
		}
	}
	if shouldSkipPS2EmulatorSidecar("formatted.card", true) {
		t.Fatal("formatted.card should be kept for retail launcher")
	}
	if shouldSkipPS2EmulatorSidecar("sce_sys/nptitle.dat", true) {
		t.Fatal("nptitle.dat should be kept for retail launcher")
	}
	if shouldSkipPS2EmulatorSidecar("sce_sys/pic1.png", false) {
		t.Fatal("pic1.png should be kept")
	}
}

func TestPS2EmuConfigOverridesEmulatorTemplate(t *testing.T) {
	template := strings.Join([]string{
		"--ps2-title-id=SCES-51920",
		"--max-disc-num=0",
		"--trophy-support=1",
		"--gs-uprender=2x2",
		"--host-display-mode=full",
	}, "\n")
	cfg := ps2RuntimeEmuConfig(PS2FPKGOptions{
		DisplayMode: "16:9",
	}, profiledTitleID, 1, template)

	for _, want := range []string{
		"--ps2-title-id=SCES-51719",
		"--max-disc-num=1",
		"--trophy-support=1",
		"--gs-uprender=2x2",
		"--host-display-mode=16:9",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config missing %q:\n%s", want, cfg)
		}
	}
	for _, bad := range []string{
		"SCES-51920",
		"--max-disc-num=0",
		"--trophy-support=0",
	} {
		if strings.Contains(cfg, bad) {
			t.Fatalf("config should not contain %q:\n%s", bad, cfg)
		}
	}
}

func TestPS2EmuConfigHomebrewOverridesEmulatorTemplate(t *testing.T) {
	template := strings.Join([]string{
		"--ps2-title-id=SCES-51920",
		"--max-disc-num=0",
		"--trophy-support=1",
		"--gs-uprender=2x2",
		"--host-display-mode=full",
	}, "\n")
	cfg := ps2RuntimeEmuConfig(PS2FPKGOptions{
		Uprender:    "off",
		DisplayMode: "16:9",
	}, "SLUS-20062", 1, template)

	for _, want := range []string{
		"--ps2-title-id=SLUS-20062",
		"--max-disc-num=1",
		"--trophy-support=0",
		"--host-trophy-support=0",
		"--gs-uprender=none",
		"--host-display-mode=16:9",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config missing %q:\n%s", want, cfg)
		}
	}
}

func TestLoadPS2EmulatorDirectoryIncludesDiscmap(t *testing.T) {
	emuRoot := "/tmp/emus-ref/emus"
	set := DefaultPS2EmulatorSets()[EmuJakV2]
	files, err := LoadEmulatorDirectoryFiles(emuRoot, set)
	if err != nil {
		t.Skip("reference emus not available:", err)
	}
	if len(files["sce_discmap.plt"]) == 0 {
		t.Fatal("missing sce_discmap.plt from emulator directory")
	}
	if len(files["config-emu-ps4.txt"]) == 0 {
		t.Fatal("missing config-emu-ps4.txt from emulator directory")
	}
}
