# PKG Forge

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Wails-v2-ff3e00?style=flat)](https://wails.io/)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey?style=flat)]()
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**PKG Forge** is a cross-platform desktop application for **merging** and **splitting** PlayStation 4 and PlayStation 5 `.pkg` archives, inspecting package metadata, and creating PS1/PS2 PS4 fPKGs from disc images. The UI is built with **Svelte**; the backend is **Go**. Distribution is a **single native binary** (or app bundle on macOS) via [Wails](https://wails.io/).

Repository: [github.com/Ketlark/PKGForge](https://github.com/Ketlark/PKGForge)

---

## Features

| Area | What you get |
|------|----------------|
| **Merge** | Recombine split parts into one `.pkg`. Auto-detects related files from a single selected part. |
| **Split** | Split a `.pkg` into chunks with configurable size and output naming schemes. |
| **Inspect** | Read PKG header metadata (content ID, title ID, region, content type, DRM, sizes). |
| **Checksum** | Compute **SHA-256** with progress and cancellation. |
| **Rename** | Suggest and apply renames based on inspected metadata (when valid). |
| **PS1 fPKG** | Build PS4 fPKGs from PS1 `.cue`/`.bin` disc images, with title/title ID detection, automatic cover art, multi-disc input, emulator assets, PlayGo metadata, and Debug RIF generation. |
| **PS2 fPKG** | Build PS4 fPKGs from PS2 `.iso`, `.cue`, or `.bin` images with SYSTEM.CNF detection and emulator configuration support. |
| **UX** | Drag-and-drop or file picker, **progress** with speed and ETA, **cancel** long operations, **activity log**, **settings** (including language). |
| **Validation** | PKG magic / header checks for PS4/PS5-style packages. |
| **i18n** | English and French (configurable in Settings). |

**Keyboard shortcuts (macOS: ⌘, Windows/Linux: Ctrl):** `⌘/Ctrl+1` … `⌘/Ctrl+5` switch between Merge, Split, Inspect, Activity, and Settings.

---

## Supported split filename patterns

These patterns are used for **detection** and **ordering** when merging split releases:

| Pattern | Example |
|---------|---------|
| `*_NNN.pkgpart` | `Game_001.pkgpart` |
| `*.pkg.NNN` | `Game.pkg.001` |
| `*.pkg_N` | `Game.pkg_0` |
| `*_N.pkg` | `Game_0.pkg` |
| `*.partN.pkg` | `Game.part0.pkg` |

---

## Supported fPKG disc inputs

| Platform | Accepted input | Notes |
|----------|----------------|-------|
| PS1 | `.cue` or `.bin` | A `.cue` plus its referenced `.bin` files represents one disc. Use Disc 2 only for a second logical CD, not for the companion BIN of Disc 1. |
| PS2 | `.iso`, `.cue`, or `.bin` | `.bin` inputs are resolved through a companion `.cue` when present. |

PS1 cover art is optional. A local `<cue>_cover.png`, `<cue>_cover.jpg`, or `<cue>-cover.jpg` next to the CUE takes priority; otherwise PKG Forge tries known serial-based cover sources and caches a 512x512 PNG.

PS1 packages include PCSX-Redux OpenBIOS as a redistributable BIOS fallback for PS1HD. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

Runtime emulator files are bundled with the app by default. The emulator directory setting is an override for development or diagnostics, not a normal requirement for creating PS1/PS2 fPKGs.

If no icon or background image is supplied or found automatically, PKG Forge generates fallback `icon0.png`, `save_data.png`, and `pic1.png` files so required `sce_sys` artwork is still present.

The fPKG builder is native Go and follows LibOrbisPkg layout rules for PKG entries, PlayGo metadata, Debug RIF, and signed/encrypted outer PFS images. Tests compare generated package entry layout and sizes against PkgTool.Core output for regression coverage.

---

## Requirements

- **Go** 1.23 or newer  
- **Node.js** 18+ (for the frontend toolchain)  
- **Wails CLI** v2:  

  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

Platform-specific build dependencies (compiler, webview, etc.) follow [Wails’ installation guide](https://wails.io/docs/gettingstarted/installation).

---

## Quick start

### Development (hot reload)

```bash
wails dev
```

### Production build

```bash
wails build
```

Artifacts appear under `build/bin/` (e.g. macOS `.app`, Windows `.exe`, Linux binary). The exact layout depends on your OS and Wails version.

### Tests

```bash
go test ./...
npm --prefix frontend run build
```

---

## CI/CD and releases

GitHub Actions run on every push / PR to `main` (or `master`): `go vet` and `go test` under `core/`, frontend `npm ci`, `npm run build`, and `svelte-check`.

**Automatic releases:** push an annotated tag matching `v*` (for example `v1.0.0`). The [Release workflow](.github/workflows/release.yml) builds **Windows (amd64)**, **macOS (universal Intel + Apple Silicon)**, **Linux (amd64 + arm64)** and publishes a **GitHub Release** with **direct download assets** (no nested zip/tar for Windows and Linux):

| Asset | Contents |
|-------|----------|
| `pkg-forge-windows-amd64.exe` | Windows executable |
| `pkg-forge-linux-amd64` | Linux x86_64 binary (chmod +x after download) |
| `pkg-forge-linux-arm64` | Linux ARM64 binary |
| `pkg-forge-macos-universal.zip` | macOS `.app` bundle (zip required for the app folder) |
| `SHA256SUMS.txt` | Checksums for the above files |

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

**Replace an existing release** (same version, new binaries): delete the release on GitHub, remove the tag locally and on the remote, then tag and push again:

```bash
git tag -d v1.0.0
git push origin :refs/tags/v1.0.0
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

The Linux **arm64** job uses the hosted runner `ubuntu-24.04-arm` (available for **public** repositories on GitHub). For a private repo, remove or adjust that matrix entry if the runner is unavailable.

---

## Project layout

```
pkg-forge/
├── main.go                 # Wails entry, embedded frontend assets
├── app.go                  # Wails bindings (Go ↔ Svelte)
├── wails.json              # Wails app metadata and frontend scripts
├── core/                   # Pure Go logic (no Wails import)
│   ├── merge.go            # Merge pipeline
│   ├── split.go            # Split pipeline
│   ├── detect.go           # Split part detection & ordering
│   ├── validate.go         # PKG header validation
│   ├── inspect.go          # Metadata extraction
│   ├── checksum.go         # SHA-256 with progress
│   ├── rename.go           # Rename suggestions / apply
│   ├── diskspace*.go       # Free space helpers (OS-specific)
│   ├── history.go          # Local activity/history persistence
│   ├── config.go           # User config
│   ├── fpkg/               # Native PS1/PS2 PS4 fPKG builder
│   ├── format.go, progress.go, options.go
│   └── *_test.go
└── frontend/               # Svelte + Vite
    └── src/
        ├── App.svelte      # Shell, tabs, shortcuts
        ├── app.css
        └── lib/
            ├── components/ # Merge, Split, Inspect, Activity, Settings, …
            ├── stores/     # i18n, activity, merge/split state
            ├── utils/
            └── types/
```

Generated bindings under `frontend/wailsjs/` are produced by Wails during `wails dev` / `wails build` (do not edit by hand).

---

## Legal notice

This tool is intended for **legitimate** uses such as managing backups or archives you are **entitled** to handle. You are responsible for complying with applicable laws, platform terms, and intellectual property rules. The authors do not endorse piracy or circumvention of DRM.

---

## Contributing

Issues and pull requests are welcome. Please run `go test ./core/ -v` before submitting changes, and match existing code style in both Go and Svelte.

---

## License

[MIT](LICENSE) © Ketlark.

---

## Acknowledgements

Built with [Wails](https://wails.io/), [Svelte](https://svelte.dev/), and [Vite](https://vitejs.dev/).
