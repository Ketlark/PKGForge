# fPKG Module Architecture

This directory implements native Go PS4 fPKG creation from PS1 and PS2 disc images.

## Pipeline

```
Disc Image (.cue/.bin or .iso)
        │
        ▼
   ┌─────────────┐
   │ disc.go     │ Parse disc → extract Game ID, Title, Region
   └──────┬──────┘
          │
          ▼
   ┌─────────────┐
   │ project.go  │ Build project directory (emulator files + game data)
   └──────┬──────┘
          │
          ▼
   ┌─────────────┐
   │ sfo.go      │ Generate param.sfo with game metadata
   └──────┬──────┘
          │
          ▼
   ┌─────────────┐
   │ gp4.go      │ Generate .gp4 XML manifest (file list + directories)
   └──────┬──────┘
          │
          ▼
   ┌─────────────┐
   │ pfs.go      │ Build inner and outer PFS images
   │ pfsc.go     │ Wrap inner PFS as PFSC data
   └──────┬──────┘
          │
          ▼
   ┌─────────────┐
   │ pkg.go      │ Assemble PKG body, PlayGo, RIF, digests, outer PFS
   └──────┬──────┘
          │
          ▼
      output.pkg
```

## Files

| File | Lines (est.) | Description |
|------|-------------|-------------|
| `doc.go` | 15 | Package documentation |
| `keys.go` | 200 | RSA keysets and symmetric keys |
| `crypto.go` | 480 | AES-128-CBC, AES-128-XTS, RSA-2048, HMAC-SHA256 |
| `sfo.go` | 300 | param.sfo parser and writer |
| `pfs.go` | 700 | PFS filesystem builder with signed outer PFS layout |
| `pfsc.go` | 100 | PFSC zlib compression wrapper |
| `pkg.go` | 1000 | PKG header, entries, PlayGo metadata, RIF, digests, assembly |
| `gp4.go` | 150 | GP4 XML manifest generation |
| `disc.go` | 200 | Disc image parsing (.cue/.bin, .iso) |
| `ps1.go` | 300 | PS1-specific: Game ID lookup, emulator packaging, progress |
| `ps2.go` | 300 | PS2-specific: SYSTEM.CNF parsing, LIMG, configs, CUE loading |
| `project.go` | 100 | Project directory setup |

## Compatibility Invariants

The fPKG writer follows LibOrbisPkg behavior, then uses PkgTool.Core as a validation and comparison tool.

### Disc input

- A PS1 `.cue` and every `.bin` it references are one logical disc image.
- A multi-disc PS1 game passes each logical disc as a separate input. The Disc 2 slot is not for the companion BIN of Disc 1.
- A `.bin` input resolves to a companion `.cue` when possible so title and title ID detection can use the CUE sheet.

### Package body

- `sce_sys/param.sfo` is both a PKG body entry for install metadata and an inner PFS file for `/app0/sce_sys/param.sfo` runtime access.
- `sce_sys/icon0.png`, `sce_sys/save_data.png`, `sce_sys/pic1.png`, and `sce_sys/pic0.png` are always present for PS1 launch/runtime artwork. User or local game art inputs take precedence where applicable; otherwise bundled emulator artwork or deterministic fallback PNGs are used.
- Base PS1/PS2 fPKGs use `APP_TYPE=1` in `param.sfo` ("Paid standalone full app"). `APP_TYPE=4` is "Freemium app" and can trigger `CE-39929-2` at launch.
- PS1HD runtime modules are loaded from `/app0/sce_module/`; `libc.prx`, `libSceFios2.prx`, and `libSceNpToolkit2.prx` must not be placed at `/app0`.
- PS1HD reads `config-title.txt` and requires an `--image` setting that points to the packaged disc image, e.g. `--image="data/disc1.bin"`. PKG Forge also writes `--ps1-title-id`, `--title-id`, `--region`, and `--has-shown-start-select-help=0` unless a template already provides those keys.
- PS1HD also loads a BIOS from `/app0/bios/`; PKG Forge bundles PCSX-Redux OpenBIOS under the `SCPH5500.bin`, `SCPH5501.bin`, and `SCPH5502.bin` filenames and sets `--bios-dir="bios"`.
- Do not emit `--bios-hide-sce-osd=1` while using bundled OpenBIOS. It is a Sony BIOS startup-logo patch and can target the wrong ROM instructions against OpenBIOS.
- PS1 discs are packaged under `/app0/data/` as `discN.bin`, a rewritten `discN.cue`, and a Cue2toc-format `discN.toc`.
- Bundled emulator asset directories provide the runtime files by default; generated `param.sfo`, `config-title.txt`, icons, and disc files take precedence.
- PS1 cover art is resolved best-effort from local `<cue>_cover.*` files, xlenore psx-covers, and PSXDataCenter, then cached as a 512x512 PNG.
- `sce_sys/keystone` is generated from the package passcode and stays inside the inner PFS.
- PlayGo entries are emitted for base packages: `playgo-chunk.dat`, `playgo-chunk.sha`, and `playgo-manifest.xml`.
- `license.dat` is a signed Debug RIF. `license.info` carries structured license metadata.
- General digests cover content, game/PFS image, header, major param fields, and the full `param.sfo`.

### Outer PFS

- The inner PFS is PFSC-wrapped before being stored as `pfs_image.dat`.
- The outer PFS is signed and encrypted. The first PFS block remains plaintext.
- Flat path table keys hash LibOrbis-style absolute PFS paths such as `/pfs_image.dat`, not relative names. Values must include the target inode number; a wrong key or a file value of `0` makes PS4 runtime lookup fail even though directory extraction can still list the file.
- LibOrbisPkg reserves the empty block after the flat path table and leaves that block plaintext.
- Signed outer PFS files larger than the direct block slots must allocate indirect signature blocks. Those blocks consume PFS space and must be signed after data block signatures.
- Tests lock the minimal package size and `pfs_image_size` because PkgTool.Core can validate hashes while still missing a malformed signed PFS layout.

## Dependencies

- `crypto/rsa`, `crypto/aes`, `crypto/sha256`, `crypto/hmac` — Go stdlib
- `compress/zlib` — PFSC block compression
- `encoding/binary` — Binary format read/write
- `encoding/xml` — GP4 generation

## Testing Strategy

1. Unit test crypto functions against known C# outputs
2. Build a minimal fPKG and verify with PkgTool.Core (the macOS binary)
3. Compare `pkg_listentries`, total package size, and PFS size against LibOrbisPkg/PkgTool.Core output
4. Test full PS1 pipeline with a sample .cue/.bin
5. Unit test PS2 CUE loading and LIMG requirements
6. Run `go test ./...` and `npm --prefix frontend run build`
