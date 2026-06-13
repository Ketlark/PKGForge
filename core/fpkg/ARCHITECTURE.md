# fPKG Module Architecture

This directory implements PS4 fPKG creation from PS1 and PS2 disc images.

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
   │ pfs.go      │ Build PFS image (inodes, dirents, DTC, signed inodes)
   │ pfsc.go     │ Compress PFS blocks with zlib
   └──────┬──────┘
          │
          ▼
   ┌─────────────┐
   │ pkg.go      │ Assemble PKG: header + entry table + PFS image + encryption
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
| `crypto.go` | 350 | AES-128-CBC, AES-128-XTS, RSA-2048, HMAC-SHA256 |
| `sfo.go` | 200 | param.sfo binary writer |
| `pfs.go` | 500 | PFS filesystem builder |
| `pfsc.go` | 100 | PFSC zlib compression wrapper |
| `pkg.go` | 400 | PKG header, entries, assembly |
| `gp4.go` | 150 | GP4 XML manifest generation |
| `disc.go` | 200 | Disc image parsing (.cue/.bin, .iso) |
| `ps1.go` | 150 | PS1-specific: Game ID lookup, emulator packaging |
| `ps2.go` | 150 | PS2-specific: SYSTEM.CNF parsing, LIMG, configs |
| `project.go` | 100 | Project directory setup |

## Dependencies

- `crypto/rsa`, `crypto/aes`, `crypto/sha256`, `crypto/hmac` — Go stdlib
- `compress/zlib` — PFSC block compression
- `encoding/binary` — Binary format read/write
- `encoding/xml` — GP4 generation

## Testing Strategy

1. Unit test crypto functions against known C# outputs
2. Build a minimal fPKG and verify with PkgTool.Core (the macOS binary)
3. Test full PS1 pipeline with a sample .cue/.bin
4. Test full PS2 pipeline with a sample .iso
