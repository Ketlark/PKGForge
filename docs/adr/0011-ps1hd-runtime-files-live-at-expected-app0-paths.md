# ADR 0011: PS1HD runtime files live at expected app0 paths

## Status

Accepted.

## Context

After the package became installable, launching a PS1 fPKG failed with PS4 error `CE-30002-5`. Sony documents this code as corrupted application data, while PS4 error tables map it to `SCE_KERNEL_ERROR_ENOENT`, which points to a missing runtime file.

The bundled PS1HD `eboot.bin` exposes runtime errors and flags showing that it expects:

- `/app0/sce_module/libSceNpToolkit2.prx` for NP Toolkit loading
- `config-title.txt` as one of the title config files
- `--image` in config so the emulator can locate the disc image

Our previous PS1 builder copied PRX modules to `/app0` and only wrote `config-emu-ps4.txt` when user options were set. That left default PS1 packages without the config file PS1HD needs to locate its packaged disc image.

## Decision

PS1 packages now:

- place `libc.prx`, `libSceFios2.prx`, and `libSceNpToolkit2.prx` under `sce_module/`
- always write `config-title.txt`
- always include an `--image` entry for the packaged disc image
- use PS1HD-recognized option names such as `--sim-analog-pad`

ADR 0012 refines the exact PS1HD disc layout to `data/discN.bin`, `data/discN.cue`, and `data/discN.toc`.

PS2 packages also keep PRX modules under `sce_module/` to match emulator GP4 layouts.

## Consequences

PkgTool-based extraction tests assert the PS1 runtime files and config are present in the inner PFS. This does not guarantee game compatibility, but it removes a known missing-file launch failure class.
