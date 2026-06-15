package fpkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeGameID(t *testing.T) {
	tests := map[string]string{
		"BOOT = cdrom:\\SCES_027.52;1":       "SCES-02752",
		"BOOT = cdrom:\\SYSTEM\\SLUS_200.62": "SLUS-20062",
		"licensed SCUS-94244":                "SCUS-94244",
		"plain SLPM12345":                    "SLPM-12345",
	}

	for input, want := range tests {
		if got := normalizeGameID(input); got != want {
			t.Fatalf("normalizeGameID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeTitleIDForPackageMetadata(t *testing.T) {
	if got := normalizeTitleID("SCES-02752"); got != "SCES02752" {
		t.Fatalf("normalizeTitleID = %q, want SCES02752", got)
	}
	if got := PS1ContentID("SCES-02752"); len(got) != 36 || got[7:16] != "SCES02752" {
		t.Fatalf("PS1 content ID = %q, want 36 chars with normalized title ID", got)
	}
	if got := PS2ContentID("SLUS-20062"); got != "UP9000-SLUS20062_00-SLUS200620000001" {
		t.Fatalf("PS2 content ID = %q", got)
	}

	sfo := NewPS1ParamSfo("Cool Game", "SCES-02752", PS1ContentID("SCES-02752"))
	for _, value := range sfo.Values {
		if value.Name == "TITLE_ID" && value.Text != "SCES02752" {
			t.Fatalf("SFO TITLE_ID = %q, want SCES02752", value.Text)
		}
	}
}

func TestParamSfoUsesPaidStandaloneAppType(t *testing.T) {
	for name, sfo := range map[string]*ParamSfo{
		"ps1": NewPS1ParamSfo("Cool Game", "SCES-02752", PS1ContentID("SCES-02752")),
		"ps2": NewPS2ParamSfo("Cool Game", "SLUS-20062", PS2ContentID("SLUS-20062")),
	} {
		appType := sfo.Get("APP_TYPE")
		if appType == nil {
			t.Fatalf("%s SFO missing APP_TYPE", name)
		}
		if appType.IntValue != 1 {
			t.Fatalf("%s APP_TYPE = %d, want 1 (paid standalone full app)", name, appType.IntValue)
		}
	}
}

func TestPS1ParamSfoMatchesPSClassicsRuntimeShape(t *testing.T) {
	sfo := NewPS1ParamSfo("Cool Game", "SCES-02752", PS1ContentID("SCES-02752"))
	for name, want := range map[string]int32{
		"ATTRIBUTE":      0,
		"ATTRIBUTE2":     0x400,
		"PARENTAL_LEVEL": 5,
		"PUBTOOLMINVER":  0x02990000,
		"SYSTEM_VER":     0x05050000,
	} {
		if got := sfo.Get(name); got == nil || got.IntValue != want {
			t.Fatalf("%s = %#v, want 0x%08x", name, got, uint32(want))
		}
	}
	if got := sfo.Get("REMOTE_PLAY_KEY_ASSIGN"); got != nil {
		t.Fatalf("modern PS1 SFO should not include REMOTE_PLAY_KEY_ASSIGN, got %#v", got)
	}
	if got := sfo.Get("TITLE_03"); got != nil {
		t.Fatalf("modern PS1 SFO should not include TITLE_03, got %#v", got)
	}
}

func TestEmulatorSetsUseRuntimeModulePaths(t *testing.T) {
	ps1 := DefaultPS1EmulatorSet()
	for _, required := range []string{
		"sce_module/libc.prx",
		"sce_module/libSceFios2.prx",
		"sce_module/libSceNpToolkit2.prx",
	} {
		if _, ok := ps1.Files[required]; !ok {
			t.Fatalf("PS1 emulator set missing virtual path %q", required)
		}
	}
	for _, obsoleteRootPath := range []string{"libc.prx", "libSceFios2.prx", "libSceNpToolkit2.prx"} {
		if _, ok := ps1.Files[obsoleteRootPath]; ok {
			t.Fatalf("PS1 emulator set should not place %q at app0 root", obsoleteRootPath)
		}
	}

	for name, ps2 := range DefaultPS2EmulatorSets() {
		for _, required := range []string{"sce_module/libc.prx", "sce_module/libSceFios2.prx"} {
			if _, ok := ps2.Files[required]; !ok {
				t.Fatalf("%s PS2 emulator set missing virtual path %q", name, required)
			}
		}
		for _, obsoleteRootPath := range []string{"libc.prx", "libSceFios2.prx"} {
			if _, ok := ps2.Files[obsoleteRootPath]; ok {
				t.Fatalf("%s PS2 emulator set should not place %q at app0 root", name, obsoleteRootPath)
			}
		}
	}
}

func TestPS1EmuConfigIncludesRuntimeImageAndKnownFlags(t *testing.T) {
	cfg := PS1EmuConfig(PS1FPKGOptions{
		AnalogSticks: true,
		SkipBootLogo: true,
		Force60Hz:    true,
	}, "SCES-02752", 2)

	for _, want := range []string{
		"--scale=6",
		"--bios-hide-sce-osd=1",
		"--ps4-trophies=0",
		"--ps5-uds=0",
		"--trophies=0",
		"--has-shown-start-select-help=0",
		"--ps1-title-id=SCES02752",
		"--title-id=SCES02752",
		`--region="SCEE"`,
		`--image="data/disc1.bin"`,
		`--image="data/disc2.bin"`,
		`--image0="data/disc1.bin"`,
		`--image1="data/disc2.bin"`,
		`--imageName1="Disc 2"`,
		"--sim-analog-pad=0x2020",
		"--gpu-scanout-fps-override=60",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("PS1 config missing %q:\n%s", want, cfg)
		}
	}

	for _, obsolete := range []string{"--simulate-analog-sticks", "--skip-bootlogo", "--force-60hz", `--bios-dir="bios"`} {
		if strings.Contains(cfg, obsolete) {
			t.Fatalf("PS1 config contains unsupported legacy flag %q:\n%s", obsolete, cfg)
		}
	}
}

func TestPS1EmuConfigPreservesTemplatePrefixAndRootBiosFallback(t *testing.T) {
	template := []byte(`# Syphon Filter (all regions)

--sim-analog-pad=0x2020
--bios-hide-sce-osd=1
--has-shown-start-select-help=1
--pace-gpu-dma=true

# following settings are machine-generated
--image="data/game.bin"
`)
	cfg := ps1RuntimeEmuConfig(PS1FPKGOptions{}, "SLUS-00558", 1, ps1EmuConfigOptions{
		Template: template,
		BiosDir:  "bios",
	})

	for _, want := range []string{
		"# Syphon Filter (all regions)",
		"--sim-analog-pad=0x2020",
		"--bios-hide-sce-osd=1",
		"--has-shown-start-select-help=1",
		"--pace-gpu-dma=true",
		"--ps1-title-id=SLUS00558",
		"--title-id=SLUS00558",
		`--region="SCEA"`,
		`--image="data/disc1.bin"`,
		`--bios-dir="bios"`,
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config missing %q:\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, `--image="data/game.bin"`) {
		t.Fatalf("config should replace stale template image:\n%s", cfg)
	}
	if strings.Contains(cfg, "--has-shown-start-select-help=0") {
		t.Fatalf("config should preserve template start/select help state:\n%s", cfg)
	}
}

func TestEnsurePS1LaunchBackgroundMirrorsPic1AndPic0(t *testing.T) {
	files := map[string][]byte{
		"sce_sys/pic1.png": []byte("official-background"),
	}
	ensurePS1LaunchBackground(files, "SLUS-00558")
	if string(files["sce_sys/pic0.png"]) != "official-background" {
		t.Fatalf("pic0 was not mirrored from pic1")
	}

	files = map[string][]byte{
		"sce_sys/pic0.png": []byte("launcher-background"),
	}
	ensurePS1LaunchBackground(files, "SLUS-00558")
	if string(files["sce_sys/pic1.png"]) != "launcher-background" {
		t.Fatalf("pic1 was not mirrored from pic0")
	}
}

func TestPS1SIEALuaProfileGeneratesImageDirAndAppBoot(t *testing.T) {
	files := map[string][]byte{
		"SIEA/app_boot.lua":           []byte(`require "disc-selection"`),
		"SIEA/config-region.txt":      []byte("--force-frame-blend=false\n"),
		"global/presets/Default.json": []byte("{}"),
	}
	profile := resolvePS1RuntimeProfile(PS1FPKGOptions{}, files)
	if profile != PS1RuntimeProfileSIEALua {
		t.Fatalf("profile = %q, want %q", profile, PS1RuntimeProfileSIEALua)
	}

	cfg := ps1RuntimeEmuConfig(PS1FPKGOptions{Force60Hz: true}, "SLUS-00382", 2, ps1EmuConfigOptions{
		RuntimeProfile:    profile,
		Template:          []byte{},
		HasGlobalGameData: true,
	})
	for _, want := range []string{
		"--image-dir=data",
		"--region-dir=SIEA",
		"--globalgamedata-dir=global",
		"--gpu-scanout-fps-override=ntsc",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("SIEA config missing %q:\n%s", want, cfg)
		}
	}

	applyPS1SIEALuaProfile(files, PS1FPKGOptions{SkipBootLogo: true, Force60Hz: true}, 2)
	lua := string(files["SIEA/app_boot.lua"])
	for _, want := range []string{`"disc1.bin"`, `"disc2.bin"`, "EM_SetCDRom(disc, i)"} {
		if !strings.Contains(lua, want) {
			t.Fatalf("SIEA app_boot.lua missing %q:\n%s", want, lua)
		}
	}
	region := string(files["SIEA/config-region.txt"])
	for _, want := range []string{"--force-frame-blend=false", "--userui-region-selector=true", "--bios-hide-sce-osd=1", "--gpu-scanout-fps-override=ntsc"} {
		if !strings.Contains(region, want) {
			t.Fatalf("SIEA config-region missing %q:\n%s", want, region)
		}
	}
}

func TestGenerateGP4SupportsExternalToolchainShape(t *testing.T) {
	data, err := GenerateGP4(GP4Options{
		ContentID:    "UP9000-SLUS00382_00-SCUS946400000000",
		Files:        []string{"sce_sys/param.sfo", "eboot.bin"},
		OmitVolumeID: true,
		Timestamp:    "2023-09-04 22:00:00",
		PackageDate:  "2023-09-05",
	})
	if err != nil {
		t.Fatalf("GenerateGP4 failed: %v", err)
	}
	gp4 := string(data)
	if strings.Contains(gp4, "<volume_id>") {
		t.Fatalf("external GP4 should omit volume_id:\n%s", gp4)
	}
	for _, want := range []string{
		`<volume_ts>2023-09-04 22:00:00</volume_ts>`,
		`content_id="UP9000-SLUS00382_00-SCUS946400000000"`,
		`c_date="2023-09-05"`,
		`default_id="0"`,
	} {
		if !strings.Contains(gp4, want) {
			t.Fatalf("GP4 missing %q:\n%s", want, gp4)
		}
	}
}

func TestGeneratePS1TOCUsesPS1HDFormat(t *testing.T) {
	tracks := []CueTrack{
		{Number: 1, Mode: "MODE2/2352", StartLBA: 0, PregapLBA: -1},
		{Number: 2, Mode: "AUDIO", StartLBA: 15000, PregapLBA: 14850},
	}
	toc := GeneratePS1TOC(tracks, 2352*18000)

	if len(toc) != 50 {
		t.Fatalf("TOC size = %d, want 50", len(toc))
	}
	for i, want := range []byte{0x41, 0x00, 0xa0, 0x00} {
		if toc[i] != want {
			t.Fatalf("TOC header byte %d = 0x%02x, want 0x%02x", i, toc[i], want)
		}
	}
	if toc[17] != 0x02 {
		t.Fatalf("TOC track count BCD = 0x%02x, want 0x02", toc[17])
	}
	first := toc[30:40]
	if first[0] != 0x41 || first[2] != 0x01 || first[7] != 0x00 || first[8] != 0x02 || first[9] != 0x00 {
		t.Fatalf("track 1 descriptor = % x", first)
	}
	second := toc[40:50]
	if second[0] != 0x01 || second[2] != 0x02 {
		t.Fatalf("track 2 descriptor = % x", second)
	}
}

func TestRewritePS1CueForPackageUsesPackagedDiscName(t *testing.T) {
	cue := RewritePS1CueForPackage([]CueTrack{
		{Number: 1, Mode: "MODE2/2352", StartLBA: 0, PregapLBA: -1},
		{Number: 2, Mode: "AUDIO", StartLBA: 15000, PregapLBA: 14850},
	}, 2)

	for _, want := range []string{
		`FILE "disc2.bin" BINARY`,
		"TRACK 01 MODE2/2352",
		"TRACK 02 AUDIO",
		"INDEX 00 03:18:00",
		"INDEX 01 03:20:00",
	} {
		if !strings.Contains(cue, want) {
			t.Fatalf("rewritten cue missing %q:\n%s", want, cue)
		}
	}
}

func TestParsePS1DiscExtractsBootIDAndFallbackTitle(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "Cool Game.bin")
	cuePath := filepath.Join(dir, "Cool Game.cue")

	data := make([]byte, 2352*32)
	copy(data[2*2352+24:], []byte("BOOT = cdrom:\\SCES_027.52;1"))
	if err := os.WriteFile(binPath, data, 0644); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	cue := `FILE "Cool Game.bin" BINARY
  TRACK 01 MODE2/2352
    INDEX 01 00:00:00
`
	if err := os.WriteFile(cuePath, []byte(cue), 0644); err != nil {
		t.Fatalf("write cue: %v", err)
	}

	disc, err := ParsePS1Disc(cuePath)
	if err != nil {
		t.Fatalf("parse ps1 disc: %v", err)
	}
	if disc.Info.GameID != "SCES-02752" {
		t.Fatalf("game ID = %q, want SCES-02752", disc.Info.GameID)
	}
	if disc.Info.Title != "Cool Game" {
		t.Fatalf("title = %q, want Cool Game", disc.Info.Title)
	}
}

func TestParseCUECapturesPregapIndex(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "Cool Game.bin")
	cuePath := filepath.Join(dir, "Cool Game.cue")

	if err := os.WriteFile(binPath, make([]byte, 2352), 0644); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	cue := `FILE "Cool Game.bin" BINARY
  TRACK 01 MODE2/2352
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    INDEX 00 03:18:00
    INDEX 01 03:20:00
`
	if err := os.WriteFile(cuePath, []byte(cue), 0644); err != nil {
		t.Fatalf("write cue: %v", err)
	}
	tracks, err := ParseCUE(cuePath)
	if err != nil {
		t.Fatalf("parse cue: %v", err)
	}
	if tracks[1].PregapLBA != 14850 {
		t.Fatalf("pregap LBA = %d, want 14850", tracks[1].PregapLBA)
	}
	if tracks[1].StartLBA != 15000 {
		t.Fatalf("start LBA = %d, want 15000", tracks[1].StartLBA)
	}
}

func TestExtractPS2GameIDFromPathWithSubdirectory(t *testing.T) {
	got := extractPS2GameIDFromPath(`cdrom0:\SYSTEM\SLUS_200.62;1`)
	if got != "SLUS-20062" {
		t.Fatalf("game ID = %q, want SLUS-20062", got)
	}
}
