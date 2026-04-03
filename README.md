# PKG Forge

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Wails-v2-ff3e00?style=flat)](https://wails.io/)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey?style=flat)]()
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**PKG Forge** is a cross-platform desktop application for **merging** and **splitting** PlayStation 4 and PlayStation 5 `.pkg` archives, with helpers for inspection, integrity checks, and safe renaming. The UI is built with **Svelte**; the backend is **Go**. Distribution is a **single native binary** (or app bundle on macOS) via [Wails](https://wails.io/).

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

### Tests (Go core)

```bash
go test ./core/ -v
```

---

## CI/CD and releases

GitHub Actions run on every push / PR to `main` (or `master`): `go vet` and `go test` under `core/`, frontend `npm ci`, `npm run build`, and `svelte-check`.

**Automatic releases:** push an annotated tag matching `v*` (for example `v1.0.0`). The [Release workflow](.github/workflows/release.yml) builds **Windows (amd64)**, **macOS (universal Intel + Apple Silicon)**, **Linux (amd64 + arm64)**, uploads archives to a **GitHub Release**, and attaches **SHA256SUMS.txt**.

```bash
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
