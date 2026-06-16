# PKG Forge

<div align="center">

**A native desktop toolbox for PlayStation PKG archives and PS1/PS2 PS4 fPKG workflows.**

Merge, split, inspect, checksum, rename, and build PS1/PS2 fPKGs from one cross-platform app.

[![Latest release](https://img.shields.io/github/v/release/Ketlark/PKGForge?style=flat-square&logo=github)](https://github.com/Ketlark/PKGForge/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](LICENSE)
[![Platforms](https://img.shields.io/badge/macOS%20%7C%20Windows%20%7C%20Linux-lightgrey?style=flat-square)](https://github.com/Ketlark/PKGForge/releases/latest)

[Download latest release](https://github.com/Ketlark/PKGForge/releases/latest) ·
[Features](#features) ·
[Downloads](#downloads) ·
[Supported inputs](#supported-inputs) ·
[Build from source](#build-from-source) ·
[Legal notice](#legal-notice)

</div>

<p align="center">
  <img src="docs/assets/pkg-forge-overview.png" alt="PKG Forge desktop interface with Merge, Split, Inspect, PS1 fPKG, PS2 fPKG, Activity, and Settings tabs" width="920" loading="lazy">
</p>

Native desktop app ([Wails](https://wails.io/) + Go + Svelte). Archive and fPKG logic is pure Go — no external packaging binaries for the main workflows.

## Features

| Area | Capabilities |
|------|--------------|
| **Merge** | Recombine split PKG parts into one file. Selecting one part can auto-detect related files. |
| **Split** | Split a PKG into chunks with configurable size, buffer, and filename format. |
| **Inspect** | Read PKG header metadata such as content ID, title ID, region, content type, DRM, and sizes. |
| **Checksum** | Calculate SHA-256 with progress updates and cancellation. |
| **Rename** | Suggest and apply clean names based on inspected metadata when available. |
| **PS1 fPKG** | Build PS4 fPKGs from PS1 `.cue` / `.bin` images, including multi-disc input, title detection, cover art, emulator assets, PlayGo metadata, and Debug RIF generation. |
| **PS2 fPKG** | Build PS4 fPKGs from PS2 `.iso`, `.cue`, or `.bin` images, with SYSTEM.CNF detection and emulator configuration support. |
| **UX** | Dark desktop UI, drag-and-drop, file picker, progress, ETA, cancellation, activity log, settings, and keyboard navigation. |
| **Updates** | In-app checks from About (Sparkle on macOS; built-in updater on Windows/Linux). |
| **i18n** | English and French interface strings. |

Keyboard shortcuts: `Cmd/Ctrl+1` through `Cmd/Ctrl+7` switch between the main workflow tabs; `Cmd/Ctrl+8` opens About.

## Downloads

Binaries are published on [GitHub Releases](https://github.com/Ketlark/PKGForge/releases/latest).

| Platform | Asset |
|----------|-------|
| Windows amd64 | [`pkg-forge-windows-amd64.exe`](https://github.com/Ketlark/PKGForge/releases/latest/download/pkg-forge-windows-amd64.exe) |
| macOS universal | [`pkg-forge-macos-universal.zip`](https://github.com/Ketlark/PKGForge/releases/latest/download/pkg-forge-macos-universal.zip) |
| Linux amd64 | [`pkg-forge-linux-amd64`](https://github.com/Ketlark/PKGForge/releases/latest/download/pkg-forge-linux-amd64) |
| Linux arm64 | [`pkg-forge-linux-arm64`](https://github.com/Ketlark/PKGForge/releases/latest/download/pkg-forge-linux-arm64) |
| Checksums | [`SHA256SUMS.txt`](https://github.com/Ketlark/PKGForge/releases/latest/download/SHA256SUMS.txt) |
| Sparkle feed (macOS) | [`appcast.xml`](https://github.com/Ketlark/PKGForge/releases/latest/download/appcast.xml) |

Linux binaries may need execute permission after download:

```bash
chmod +x pkg-forge-linux-amd64
```

Auto-update setup and troubleshooting: **[docs/auto-update.md](docs/auto-update.md)**.

## Supported inputs

### Split package detection

These patterns are used for detection and ordering when merging split releases:

| Pattern | Example |
|---------|---------|
| `*_NNN.pkgpart` | `Game_001.pkgpart` |
| `*.pkg.NNN` | `Game.pkg.001` |
| `*.pkg_N` | `Game.pkg_0` |
| `*_N.pkg` | `Game_0.pkg` |
| `*.partN.pkg` | `Game.part0.pkg` |

### fPKG disc inputs

| Platform | Accepted input | Notes |
|----------|----------------|-------|
| PS1 | `.cue` or `.bin` | A `.cue` plus its referenced `.bin` files represents one disc. Use Disc 2 only for a second logical CD, not for the companion BIN of Disc 1. |
| PS2 | `.iso`, `.cue`, or `.bin` | `.bin` inputs are resolved through a companion `.cue` when present. |

## fPKG builder notes

PS1 cover art is optional. A local `<cue>_cover.png`, `<cue>_cover.jpg`, or `<cue>-cover.jpg` next to the CUE takes priority. If no local cover is found, PKG Forge tries known serial-based cover sources and caches a 512x512 PNG.

PS1 launch backgrounds follow this priority:

1. User-supplied background.
2. Local game background next to the CUE.
3. Bundled official PS1HD default background.
4. Cover-derived generated background.
5. Generated fallback artwork.

Runtime emulator files are bundled with the app by default. The emulator directory setting is an override for development or diagnostics, not a normal requirement for creating PS1/PS2 fPKGs.

PS1 packages include PCSX-Redux OpenBIOS as a redistributable BIOS fallback for PS1HD. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

The fPKG builder is native Go and follows LibOrbisPkg layout rules for PKG entries, PlayGo metadata, Debug RIF, and signed/encrypted outer PFS images. Tests compare generated package entry layout and sizes against PkgTool.Core output for regression coverage.

## Build from source

### Requirements

- Go 1.23 or newer
- Node.js 18+ for the frontend toolchain
- Wails CLI v2

Install Wails:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Platform-specific compiler and webview dependencies follow the [Wails installation guide](https://wails.io/docs/gettingstarted/installation).

### Development

```bash
wails dev
```

### Production build

```bash
wails build
```

Artifacts appear under `build/bin/`.

**macOS release build (with Sparkle auto-update):**

```bash
CGO_ENABLED=1 CGO_LDFLAGS='-Wl,-rpath,@loader_path/../Frameworks' \
  wails build -platform darwin/universal -tags sparkle -clean
```

The post-build hook in `wails.json` embeds `Sparkle.framework` into the `.app`.

Releases are cut by pushing an annotated `v*` tag; CI builds all platforms. Maintainer details: **[docs/auto-update.md](docs/auto-update.md)**.

## Contributing

Issues and pull requests are welcome. Before submitting changes:

```bash
go test ./...
npm --prefix frontend run build
```

Please match the existing Go and Svelte style, keep changes focused, and include tests for behavior that affects archive or fPKG generation.

## Legal notice

This tool is intended for legitimate uses such as managing backups or archives you are entitled to handle. You are responsible for complying with applicable laws, platform terms, and intellectual property rules. The authors do not endorse piracy or DRM circumvention.

PKG Forge is an independent open-source project and is not affiliated with Sony Interactive Entertainment.

## License

[MIT](LICENSE) © Ketlark.
