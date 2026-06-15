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

PKG Forge bundles PCSX-Redux OpenBIOS in the PS1HD emulator asset set as:

- `bios/SCPH5500.bin`
- `bios/SCPH5501.bin`
- `bios/SCPH5502.bin`

The three files contain the same OpenBIOS binary but use the names PS1HD
autodetects for Japan, America, and Europe. `config-title.txt` now includes
`--bios-dir="bios"` so the runtime lookup is explicit.

Because OpenBIOS is not Sony's retail boot ROM, PKG Forge does not emit
PS1HD's `--bios-hide-sce-osd=1` patch flag. That flag is intended to hide the
Sony BIOS startup logos and can target the wrong ROM instructions when the
bundled BIOS is OpenBIOS.

The OpenBIOS notice is recorded in `THIRD_PARTY_NOTICES.md`.

## Consequences

Generated PS1 packages no longer depend on users supplying Sony BIOS dumps, and
the app can redistribute the bundled fallback under a compatible MIT license.

OpenBIOS may not behave identically to retail BIOS revisions for every game.
If a title still fails, a future enhancement can allow users to override the
packaged BIOS with their own legally obtained files while keeping OpenBIOS as
the default.
