package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pkg-forge/core"
	"pkg-forge/core/fpkg"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Response types for Wails bindings (avoid anonymous structs).

type DetectResult struct {
	Parts      []string `json:"parts"`
	OutputName string   `json:"outputName"`
}

type ValidationResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

type RenameResult struct {
	NewName string       `json:"newName"`
	Info    core.PKGInfo `json:"info"`
}

type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Dir  string `json:"dir"`
}

type FPKGProgress struct {
	Percentage     float64 `json:"percentage"`
	Phase          string  `json:"phase"`
	BytesProcessed int64   `json:"bytesProcessed"`
	TotalBytes     int64   `json:"totalBytes"`
	SpeedBPS       float64 `json:"speedBPS"`
	ETASeconds     float64 `json:"etaSeconds"`
}

// App bridges the Svelte frontend with the core logic via Wails bindings.
type App struct {
	ctx context.Context

	mu       sync.Mutex
	cancelFn map[string]chan struct{}
}

func NewApp() *App {
	return &App{cancelFn: make(map[string]chan struct{})}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func resolveCompanionCue(binPath string) (string, error) {
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

	return "", fmt.Errorf("no .cue file found for %s; please provide the .cue file instead", filepath.Base(binPath))
}

func estimateDiscBytes(paths []string) int64 {
	seen := make(map[string]bool)
	var total int64

	addFile := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			total += info.Size()
		}
	}

	for _, path := range paths {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".cue":
			tracks, err := fpkg.ParseCUE(path)
			if err != nil {
				addFile(path)
				continue
			}
			for _, track := range tracks {
				addFile(track.File)
			}
		case ".bin":
			cuePath, err := resolveCompanionCue(path)
			if err != nil {
				addFile(path)
				continue
			}
			tracks, err := fpkg.ParseCUE(cuePath)
			if err != nil {
				addFile(path)
				continue
			}
			for _, track := range tracks {
				addFile(track.File)
			}
		default:
			addFile(path)
		}
	}

	return total
}

func (a *App) emitFPKGProgress(start time.Time, totalBytes int64, percent float64, phase string) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	var bytesProcessed int64
	var speedBPS, etaSeconds float64
	if totalBytes > 0 {
		bytesProcessed = int64(float64(totalBytes) * percent / 100)
		if bytesProcessed > totalBytes {
			bytesProcessed = totalBytes
		}
		speedBPS, etaSeconds = core.SpeedETA(bytesProcessed, totalBytes, start)
	} else if percent > 0 && percent < 100 {
		elapsed := time.Since(start).Seconds()
		etaSeconds = elapsed * (100 - percent) / percent
	}

	runtime.EventsEmit(a.ctx, "fpkg-progress", FPKGProgress{
		Percentage:     percent,
		Phase:          phase,
		BytesProcessed: bytesProcessed,
		TotalBytes:     totalBytes,
		SpeedBPS:       speedBPS,
		ETASeconds:     etaSeconds,
	})
}

// acquireCancel creates a per-operation cancel channel keyed by operation type,
// preventing merge/split/checksum from stomping each other's channel.
func (a *App) acquireCancel(op string) <-chan struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	ch := make(chan struct{})
	a.cancelFn[op] = ch
	return ch
}

func (a *App) releaseCancel(op string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.cancelFn, op)
}

// --- Merge bindings ---

func (a *App) DetectParts(filePath string) DetectResult {
	parts, name := core.DetectParts(filePath)
	return DetectResult{parts, name}
}

func (a *App) SuggestOutputPath(parts []string, detectedName string) string {
	return core.SuggestOutputPath(parts, detectedName)
}

func (a *App) MergeFiles(parts []string, outputPath string, bufferLabel string) error {
	cancel := a.acquireCancel("merge")
	defer a.releaseCancel("merge")

	return core.Merge(core.MergeOptions{
		Parts:      parts,
		OutputPath: outputPath,
		BufferSize: core.BufferBytes(bufferLabel),
		OnProgress: func(p core.MergeProgress) {
			runtime.EventsEmit(a.ctx, "merge-progress", p)
		},
		Cancel: cancel,
	})
}

// --- Split bindings ---

func (a *App) SplitFile(sourcePath, outputDir, chunkLabel, formatLabel, bufferLabel string) error {
	cancel := a.acquireCancel("split")
	defer a.releaseCancel("split")

	_, err := core.Split(core.SplitOptions{
		SourcePath: sourcePath,
		OutputDir:  outputDir,
		ChunkSize:  core.ChunkBytes(chunkLabel),
		Format:     core.SplitFormatByLabel(formatLabel),
		BufferSize: core.BufferBytes(bufferLabel),
		OnProgress: func(p core.SplitProgress) {
			runtime.EventsEmit(a.ctx, "split-progress", p)
		},
		Cancel: cancel,
	})
	return err
}

// --- Validation & Inspection ---

func (a *App) ValidatePKG(path string) ValidationResult {
	valid, msg := core.ValidatePKG(path)
	return ValidationResult{valid, msg}
}

func (a *App) InspectPKG(path string) core.PKGInfo {
	return core.InspectPKG(path)
}

// --- Checksum ---

func (a *App) CalculateChecksum(path string) (core.ChecksumResult, error) {
	cancel := a.acquireCancel("checksum")
	defer a.releaseCancel("checksum")

	return core.CalculateChecksum(path, func(pct float64) {
		runtime.EventsEmit(a.ctx, "checksum-progress", pct)
	}, cancel)
}

// --- Disk Space ---

func (a *App) CheckDiskSpace(path string) core.DiskSpaceInfo {
	info, err := core.GetDiskSpace(path)
	if err != nil {
		return core.DiskSpaceInfo{}
	}
	return info
}

// --- Rename ---

func (a *App) SuggestRenamePKG(path string) RenameResult {
	name, info := core.SuggestRename(path)
	return RenameResult{name, info}
}

func (a *App) RenamePKG(path string) (string, error) {
	return core.RenamePKG(path)
}

// --- Config ---

func (a *App) LoadConfig() core.Config          { return core.LoadConfig() }
func (a *App) SaveConfig(cfg core.Config) error { return core.SaveConfig(cfg) }

// --- History ---

func (a *App) GetHistory() []core.HistoryEntry               { return core.LoadHistory() }
func (a *App) AddHistoryEntry(entry core.HistoryEntry) error { return core.AddHistory(entry) }
func (a *App) ClearHistory() error                           { return core.ClearHistory() }

// --- Options ---

func (a *App) BufferLabels() []string      { return core.BufferLabels() }
func (a *App) ChunkLabels() []string       { return core.ChunkLabels() }
func (a *App) SplitFormatLabels() []string { return core.SplitFormatLabels() }
func (a *App) FormatSize(b int64) string   { return core.FormatSize(b) }

// --- Auto-update ---

func (a *App) AppVersion() string { return Version }

func (a *App) UpdateBackend() string { return core.UpdateBackend() }

func (a *App) ConfigureUpdateOnStartup(enabled bool) {
	core.ConfigureUpdateOnStartup(enabled)
}

func (a *App) CheckForUpdates() (*core.UpdateInfo, error) {
	return core.CheckForUpdate(a.ctx, Version)
}

func (a *App) DownloadAndApplyUpdate(info *core.UpdateInfo) error {
	return core.DownloadAndApplyUpdate(a.ctx, info, func(p float64) {
		runtime.EventsEmit(a.ctx, "update-progress", p)
	})
}

// ---------------------------------------------------------------------------
// PS1 fPKG bindings
// ---------------------------------------------------------------------------

// PS1FPKGRequest is the frontend request struct for PS1 fPKG creation.
type PS1FPKGRequest struct {
	CuePath        string   `json:"cuePath"`
	ExtraDiscs     []string `json:"extraDiscs"`
	OutputPath     string   `json:"outputPath"`
	Title          string   `json:"title"`
	TitleID        string   `json:"titleID"`
	Icon0          string   `json:"icon0"`
	Pic1           string   `json:"pic1"`
	Emulator       string   `json:"emulator"`
	AnalogSticks   bool     `json:"analogSticks"`
	SkipBootLogo   bool     `json:"skipBootLogo"`
	Force60Hz      bool     `json:"force60Hz"`
	EnableCDDATOC  bool     `json:"enableCddaToc"`
	RuntimeProfile string   `json:"runtimeProfile"`
}

// PS1DiscDetectResult holds auto-detection results for a PS1 disc.
type PS1DiscDetectResult struct {
	GameID     string `json:"gameID"`
	Title      string `json:"title"`
	Region     string `json:"region"`
	TrackNum   int    `json:"trackNum"`
	HasCDDA    bool   `json:"hasCdda"`
	IsMultiBin bool   `json:"isMultiBin"`
	CoverPath  string `json:"coverPath"`
}

// DetectPS1Disc parses a PS1 .cue or .bin file and returns disc metadata.
func (a *App) DetectPS1Disc(discPath string) (*PS1DiscDetectResult, error) {
	cuePath, err := resolveCompanionCue(discPath)
	if err != nil {
		return nil, err
	}

	disc, err := fpkg.ParsePS1Disc(cuePath)
	if err != nil {
		return nil, err
	}
	coverPath := ""
	if disc.Info.GameID != "" {
		if resolvedCover, err := fpkg.ResolvePS1Cover(cuePath, disc.Info.GameID); err == nil {
			coverPath = resolvedCover
		}
	}
	return &PS1DiscDetectResult{
		GameID:     disc.Info.GameID,
		Title:      disc.Info.Title,
		Region:     disc.Info.Region,
		TrackNum:   disc.TrackNum,
		HasCDDA:    disc.HasCDDA,
		IsMultiBin: disc.Info.IsMultiBin,
		CoverPath:  coverPath,
	}, nil
}

// CreatePS1FPKG creates a PS1 fPKG from the given options.
func (a *App) CreatePS1FPKG(req PS1FPKGRequest) error {
	cuePath, err := resolveCompanionCue(req.CuePath)
	if err != nil {
		return err
	}

	extraDiscs := make([]string, 0, len(req.ExtraDiscs))
	for i, extraDisc := range req.ExtraDiscs {
		resolved, err := resolveCompanionCue(extraDisc)
		if err != nil {
			return fmt.Errorf("extra disc %d: %w", i+2, err)
		}
		extraDiscs = append(extraDiscs, resolved)
	}

	opts := fpkg.PS1FPKGOptions{
		CuePath:          cuePath,
		ExtraDiscs:       extraDiscs,
		OutputPath:       req.OutputPath,
		Title:            req.Title,
		TitleID:          req.TitleID,
		Icon0:            req.Icon0,
		Pic1:             req.Pic1,
		Emulator:         req.Emulator,
		AnalogSticks:     req.AnalogSticks,
		SkipBootLogo:     req.SkipBootLogo,
		Force60Hz:        req.Force60Hz,
		EnableCDDATOC:    req.EnableCDDATOC,
		RuntimeProfile:   fpkg.PS1RuntimeProfile(req.RuntimeProfile),
		EmulatorFilesDir: core.LoadConfig().EmulatorFilesDir,
	}

	discPaths := append([]string{cuePath}, extraDiscs...)
	totalBytes := estimateDiscBytes(discPaths)
	start := time.Now()
	a.emitFPKGProgress(start, totalBytes, 0, "Preparing")

	// Ensure emulator assets are downloaded
	if err := fpkg.EnsureAssets(func(pct float64) {
		a.emitFPKGProgress(start, totalBytes, pct*20, "Preparing emulator assets")
	}); err != nil {
		runtime.EventsEmit(a.ctx, "fpkg-error", "Failed to download emulator assets: "+err.Error())
		return err
	}
	a.emitFPKGProgress(start, totalBytes, 20, "Emulator assets ready")

	opts.OnProgress = func(percent float64, phase string) {
		a.emitFPKGProgress(start, totalBytes, 20+percent*0.78, phase)
	}

	err = fpkg.CreatePS1FPKG(opts)

	if err != nil {
		runtime.EventsEmit(a.ctx, "fpkg-error", err.Error())
		return err
	}

	a.emitFPKGProgress(start, totalBytes, 100, "Complete")
	runtime.EventsEmit(a.ctx, "fpkg-complete", req.OutputPath)
	return nil
}

// OpenCUEFileDialog opens a file dialog for selecting PS1 .cue or .bin files.
func (a *App) OpenCUEFileDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select a PS1 disc image",
		Filters: []runtime.FileFilter{
			{DisplayName: "Disc images", Pattern: "*.cue;*.bin"},
			{DisplayName: "CUE files", Pattern: "*.cue"},
			{DisplayName: "BIN files", Pattern: "*.bin"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

// ---------------------------------------------------------------------------
// PS2 fPKG bindings
// ---------------------------------------------------------------------------

// PS2FPKGRequest is the frontend request struct for PS2 fPKG creation.
type PS2FPKGRequest struct {
	ISOPaths        []string `json:"isoPaths"`
	OutputPath      string   `json:"outputPath"`
	Title           string   `json:"title"`
	TitleID         string   `json:"titleID"`
	Icon0           string   `json:"icon0"`
	Pic1            string   `json:"pic1"`
	Emulator        string   `json:"emulator"`
	ConfigTXT       string   `json:"configTxt"`
	ConfigLUA       string   `json:"configLua"`
	MemoryCardPath  string   `json:"memoryCardPath"`
	WidescreenPatch string   `json:"widescreenPatch"`
	Uprender        string   `json:"uprender"`
	DisplayMode     string   `json:"displayMode"`
}

// PS2DiscDetectResult holds auto-detection results for a PS2 disc.
type PS2DiscDetectResult struct {
	GameID     string            `json:"gameID"`
	Title      string            `json:"title"`
	Region     string            `json:"region"`
	SystemCNF  map[string]string `json:"systemCNF"`
	CoverPath  string            `json:"coverPath"`
	Profile    *fpkg.PS2ProfileHint `json:"profile,omitempty"`
}

// DetectPS2Disc parses a PS2 .iso file and returns disc metadata.
func (a *App) DetectPS2Disc(isoPath string) (*PS2DiscDetectResult, error) {
	ext := strings.ToLower(filepath.Ext(isoPath))
	var discInfo *fpkg.DiscInfo
	var err error

	switch ext {
	case ".iso":
		discInfo, err = fpkg.ParsePS2Disc(isoPath)
	case ".cue":
		discInfo, err = fpkg.ParsePS2DiscFromCUE(isoPath)
	case ".bin":
		cuePath, resolveErr := resolveCompanionCue(isoPath)
		if resolveErr != nil {
			return nil, resolveErr
		}
		discInfo, err = fpkg.ParsePS2DiscFromCUE(cuePath)
	default:
		return nil, fmt.Errorf("unsupported file extension: %s (expected .iso, .cue, or .bin)", ext)
	}

	if err != nil {
		return nil, err
	}
	coverPath := ""
	if discInfo.GameID != "" {
		if resolvedCover, err := fpkg.ResolvePS2Cover(isoPath, discInfo.GameID); err == nil {
			coverPath = resolvedCover
		}
	}
	profile, _ := fpkg.PS2ProfileForGame(discInfo.GameID)
	return &PS2DiscDetectResult{
		GameID:    discInfo.GameID,
		Title:     discInfo.Title,
		Region:    discInfo.Region,
		SystemCNF: discInfo.SystemCNF,
		CoverPath: coverPath,
		Profile:   profile,
	}, nil
}

// CreatePS2FPKG creates a PS2 fPKG from the given options.
func (a *App) CreatePS2FPKG(req PS2FPKGRequest) error {
	opts := fpkg.PS2FPKGOptions{
		ISOPaths:         req.ISOPaths,
		OutputPath:       req.OutputPath,
		Title:            req.Title,
		TitleID:          req.TitleID,
		Icon0:            req.Icon0,
		Pic1:             req.Pic1,
		Emulator:         fpkg.PS2EmulatorType(req.Emulator),
		ConfigTXT:        req.ConfigTXT,
		ConfigLUA:        req.ConfigLUA,
		MemoryCardPath:   req.MemoryCardPath,
		WidescreenPatch:  req.WidescreenPatch,
		Uprender:         req.Uprender,
		DisplayMode:      req.DisplayMode,
		EmulatorFilesDir: core.LoadConfig().EmulatorFilesDir,
	}

	totalBytes := estimateDiscBytes(req.ISOPaths)
	start := time.Now()
	a.emitFPKGProgress(start, totalBytes, 0, "Preparing")

	// Ensure emulator assets are downloaded
	if err := fpkg.EnsureAssets(func(pct float64) {
		a.emitFPKGProgress(start, totalBytes, pct*20, "Preparing emulator assets")
	}); err != nil {
		runtime.EventsEmit(a.ctx, "fpkg-error", "Failed to download emulator assets: "+err.Error())
		return err
	}
	a.emitFPKGProgress(start, totalBytes, 20, "Emulator assets ready")

	opts.OnProgress = func(percent float64, phase string) {
		a.emitFPKGProgress(start, totalBytes, 20+percent*0.78, phase)
	}

	err := fpkg.CreatePS2FPKG(opts)

	if err != nil {
		runtime.EventsEmit(a.ctx, "fpkg-error", err.Error())
		return err
	}

	a.emitFPKGProgress(start, totalBytes, 100, "Complete")
	runtime.EventsEmit(a.ctx, "fpkg-complete", req.OutputPath)
	return nil
}

// OpenISOFileDialog opens a file dialog for selecting PS2 disc images.
func (a *App) OpenISOFileDialog() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select PS2 disc images",
		Filters: []runtime.FileFilter{
			{DisplayName: "Disc images", Pattern: "*.iso;*.cue;*.bin"},
			{DisplayName: "ISO files", Pattern: "*.iso"},
			{DisplayName: "CUE files", Pattern: "*.cue"},
			{DisplayName: "BIN files", Pattern: "*.bin"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

// ---------------------------------------------------------------------------
// Common fPKG dialogs
// ---------------------------------------------------------------------------

// OpenImageFileDialog opens a file dialog for selecting PNG images (icons/backgrounds).
func (a *App) OpenImageFileDialog(title string) (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: title,
		Filters: []runtime.FileFilter{
			{DisplayName: "PNG images", Pattern: "*.png"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

// SavePKGFileDialog opens a save dialog for PKG output.
func (a *App) SavePKGFileDialog(defaultName string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save PKG file",
		DefaultFilename: defaultName + ".pkg",
		Filters: []runtime.FileFilter{
			{DisplayName: "PKG files", Pattern: "*.pkg"},
		},
	})
}

// OpenEmulatorDirDialog opens a directory dialog for selecting the emulator files folder.
func (a *App) OpenEmulatorDirDialog() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select emulator files directory",
	})
}

// DetectDiscType auto-detects whether a file is a PS1 or PS2 disc image.
func (a *App) DetectDiscType(path string) (string, error) {
	return fpkg.DetectDiscType(path)
}

// --- Operations ---

// CancelOperation stops all running operations.
func (a *App) CancelOperation() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, ch := range a.cancelFn {
		close(ch)
		delete(a.cancelFn, key)
	}
}

// --- Dialogs ---

func (a *App) OpenFilesDialog() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select PKG split files",
		Filters: []runtime.FileFilter{
			{DisplayName: "PKG files", Pattern: "*.pkg;*.pkgpart;*.pkg.*"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

func (a *App) OpenFileDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select a PKG file",
		Filters: []runtime.FileFilter{
			{DisplayName: "PKG files", Pattern: "*.pkg"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

func (a *App) OpenDirectoryDialog() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select output directory",
	})
}

func (a *App) GetFileInfo(path string) FileInfo {
	name := filepath.Base(path)
	dir := filepath.Dir(path)
	var size int64
	if info, err := os.Stat(path); err == nil {
		size = info.Size()
	}
	return FileInfo{name, size, dir}
}
