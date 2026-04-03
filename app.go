package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"pkg-forge/core"

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
