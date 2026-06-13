package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"

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

func (a *App) LoadConfig() core.Config  { return core.LoadConfig() }
func (a *App) SaveConfig(cfg core.Config) error { return core.SaveConfig(cfg) }

// --- History ---

func (a *App) GetHistory() []core.HistoryEntry       { return core.LoadHistory() }
func (a *App) AddHistoryEntry(entry core.HistoryEntry) error { return core.AddHistory(entry) }
func (a *App) ClearHistory() error                    { return core.ClearHistory() }

// --- Options ---

func (a *App) BufferLabels() []string      { return core.BufferLabels() }
func (a *App) ChunkLabels() []string       { return core.ChunkLabels() }
func (a *App) SplitFormatLabels() []string { return core.SplitFormatLabels() }
func (a *App) FormatSize(b int64) string   { return core.FormatSize(b) }

// ---------------------------------------------------------------------------
// PS1 fPKG bindings
// ---------------------------------------------------------------------------

// PS1FPKGRequest is the frontend request struct for PS1 fPKG creation.
type PS1FPKGRequest struct {
	CuePath       string   `json:"cuePath"`
	ExtraDiscs    []string `json:"extraDiscs"`
	OutputPath    string   `json:"outputPath"`
	Title         string   `json:"title"`
	TitleID       string   `json:"titleID"`
	Icon0         string   `json:"icon0"`
	Pic1          string   `json:"pic1"`
	Emulator      string   `json:"emulator"`
	AnalogSticks  bool     `json:"analogSticks"`
	SkipBootLogo  bool     `json:"skipBootLogo"`
	Force60Hz     bool     `json:"force60Hz"`
	EnableCDDATOC bool     `json:"enableCddaToc"`
}

// PS1DiscDetectResult holds auto-detection results for a PS1 disc.
type PS1DiscDetectResult struct {
	GameID    string `json:"gameID"`
	Title     string `json:"title"`
	Region    string `json:"region"`
	TrackNum  int    `json:"trackNum"`
	HasCDDA   bool   `json:"hasCdda"`
	IsMultiBin bool  `json:"isMultiBin"`
}

// DetectPS1Disc parses a PS1 .cue file and returns disc metadata.
func (a *App) DetectPS1Disc(cuePath string) (*PS1DiscDetectResult, error) {
	disc, err := fpkg.ParsePS1Disc(cuePath)
	if err != nil {
		return nil, err
	}
	return &PS1DiscDetectResult{
		GameID:     disc.Info.GameID,
		Title:      disc.Info.Title,
		Region:     disc.Info.Region,
		TrackNum:   disc.TrackNum,
		HasCDDA:    disc.HasCDDA,
		IsMultiBin: disc.Info.IsMultiBin,
	}, nil
}

// CreatePS1FPKG creates a PS1 fPKG from the given options.
func (a *App) CreatePS1FPKG(req PS1FPKGRequest) error {
	opts := fpkg.PS1FPKGOptions{
		CuePath:          req.CuePath,
		ExtraDiscs:       req.ExtraDiscs,
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
		EmulatorFilesDir: core.LoadConfig().EmulatorFilesDir,
	}

	runtime.EventsEmit(a.ctx, "fpkg-progress", 0.0)

	// Ensure emulator assets are downloaded
	if err := fpkg.EnsureAssets(func(pct float64) {
		runtime.EventsEmit(a.ctx, "fpkg-progress", pct*0.3) // assets = 0..30%
	}); err != nil {
		runtime.EventsEmit(a.ctx, "fpkg-error", "Failed to download emulator assets: "+err.Error())
		return err
	}

	err := fpkg.CreatePS1FPKG(opts)

	if err != nil {
		runtime.EventsEmit(a.ctx, "fpkg-error", err.Error())
		return err
	}

	runtime.EventsEmit(a.ctx, "fpkg-progress", 1.0)
	runtime.EventsEmit(a.ctx, "fpkg-complete", req.OutputPath)
	return nil
}

// OpenCUEFileDialog opens a file dialog for selecting PS1 .cue files.
func (a *App) OpenCUEFileDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select a PS1 CUE file",
		Filters: []runtime.FileFilter{
			{DisplayName: "CUE files", Pattern: "*.cue"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

// ---------------------------------------------------------------------------
// PS2 fPKG bindings
// ---------------------------------------------------------------------------

// PS2FPKGRequest is the frontend request struct for PS2 fPKG creation.
type PS2FPKGRequest struct {
	ISOPaths       []string `json:"isoPaths"`
	OutputPath     string   `json:"outputPath"`
	Title          string   `json:"title"`
	TitleID        string   `json:"titleID"`
	Icon0          string   `json:"icon0"`
	Pic1           string   `json:"pic1"`
	Emulator       string   `json:"emulator"`
	ConfigTXT      string   `json:"configTxt"`
	ConfigLUA      string   `json:"configLua"`
	MemoryCardPath string   `json:"memoryCardPath"`
	WidescreenPatch string  `json:"widescreenPatch"`
	Uprender       string   `json:"uprender"`
	DisplayMode    string   `json:"displayMode"`
}

// PS2DiscDetectResult holds auto-detection results for a PS2 disc.
type PS2DiscDetectResult struct {
	GameID    string            `json:"gameID"`
	Title     string            `json:"title"`
	Region    string            `json:"region"`
	SystemCNF map[string]string `json:"systemCNF"`
}

// DetectPS2Disc parses a PS2 .iso file and returns disc metadata.
func (a *App) DetectPS2Disc(isoPath string) (*PS2DiscDetectResult, error) {
	ext := filepath.Ext(isoPath)
	var discInfo *fpkg.DiscInfo
	var err error

	switch ext {
	case ".cue":
		discInfo, err = fpkg.ParsePS2DiscFromCUE(isoPath)
	default:
		discInfo, err = fpkg.ParsePS2Disc(isoPath)
	}

	if err != nil {
		return nil, err
	}
	return &PS2DiscDetectResult{
		GameID:    discInfo.GameID,
		Title:     discInfo.Title,
		Region:    discInfo.Region,
		SystemCNF: discInfo.SystemCNF,
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

	runtime.EventsEmit(a.ctx, "fpkg-progress", 0.0)

	// Ensure emulator assets are downloaded
	if err := fpkg.EnsureAssets(func(pct float64) {
		runtime.EventsEmit(a.ctx, "fpkg-progress", pct*0.3) // assets = 0..30%
	}); err != nil {
		runtime.EventsEmit(a.ctx, "fpkg-error", "Failed to download emulator assets: "+err.Error())
		return err
	}

	err := fpkg.CreatePS2FPKG(opts)

	if err != nil {
		runtime.EventsEmit(a.ctx, "fpkg-error", err.Error())
		return err
	}

	runtime.EventsEmit(a.ctx, "fpkg-progress", 1.0)
	runtime.EventsEmit(a.ctx, "fpkg-complete", req.OutputPath)
	return nil
}

// OpenISOFileDialog opens a file dialog for selecting PS2 .iso files.
func (a *App) OpenISOFileDialog() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select PS2 ISO files",
		Filters: []runtime.FileFilter{
			{DisplayName: "ISO files", Pattern: "*.iso"},
			{DisplayName: "BIN/CUE files", Pattern: "*.bin;*.cue"},
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
