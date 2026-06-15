# ADR 0016: Bundle OpenBIOS for PS1HD packages

Date: 2026-06-13

## Status

Accepted

## Context

PS1HD loads a PlayStation BIOS at runtime. The emulator binary exposes
`--bios-dir`, `--bios`, and the autodetected filenames `SCPH5500.bin`,
`SCPH5501.bin`, and `SCPH5502.bin`. A PS4 launch log for `SLES02887` progressed
past install and PFS mount, launched `/app0/eboot.bin`, then crashed in
`EmuCorePS1` with `SIGILL`. The generated package did not include any BIOS
files, so PS1HD had no `/app0/bios/...` file to load.

Sony retail BIOS dumps are copyright-protected and must not be bundled by PKG
Forge. PCSX-Redux OpenBIOS is an open-source PlayStation BIOS replacement, and
Libreboot distributes a prebuilt `openbios.bin` under the MIT license.

## Decision

PKG Forge bundles BIOS files in the PS1HD emulator asset set at:

- `assets/PS1HD/bios/SCPH5500.bin`
- `assets/PS1HD/bios/SCPH5501.bin`
- `assets/PS1HD/bios/SCPH5502.bin`

These are stored under `emus/ps1hd/` in the encrypted `assets.dat` cache and
extract to the same relative paths inside `/app0/`. PS1HD autodetects them by
filename at this default location, so `config-title.txt` does **not** emit
`--bios-dir` for the bundled layout. The `--bios-dir="bios"` override is only
emitted when the user supplies BIOS files in a legacy `bios/` directory.

Because OpenBIOS is not Sony's retail boot ROM, PKG Forge does not emit
PS1HD's `--bios-hide-sce-osd=1` flag when OpenBIOS is detected. That flag is
intended to hide the Sony BIOS startup logos and can target the wrong ROM
instructions when the bundled BIOS is OpenBIOS. When the user supplies a
retail Sony BIOS, the flag is emitted.

The OpenBIOS notice is recorded in `THIRD_PARTY_NOTICES.md`.

## Consequences

Generated PS1 packages no longer depend on users supplying Sony BIOS dumps, and
the app can redistribute the bundled fallback under a compatible MIT license.

OpenBIOS may not behave identically to retail BIOS revisions for every game.
If a title still fails, a future enhancement can allow users to override the
packaged BIOS with their own legally obtained files while keeping OpenBIOS as
the default.
