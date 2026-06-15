# ADR 0013: Runtime sce_sys files are present in app0

Date: 2026-06-13

## Status

Accepted

## Context

Generated PS1 fPKGs could install and pass PkgTool.Core validation while launch still failed with `CE-30002-5`. The remaining failure mode looked like a runtime `ENOENT`: files that exist as PKG metadata entries are not automatically guaranteed to exist as `/app0/sce_sys/...` files inside the inner PFS.

The previous builder removed `sce_sys/param.sfo` from the inner PFS and only emitted artwork when the user supplied it or automatic cover lookup succeeded. That left packages with no `/app0/sce_sys/param.sfo`, no `/app0/sce_sys/icon0.png`, no `/app0/sce_sys/save_data.png`, and no `/app0/sce_sys/pic1.png`.

## Decision

PKG Forge now duplicates runtime metadata into the inner PFS:

- `sce_sys/param.sfo` is included in `/app0` and still emitted as the PKG `PARAM_SFO` body entry.
- `sce_sys/icon0.png`, `sce_sys/save_data.png`, and `sce_sys/pic1.png` are always included in `/app0`.
- `ICON0_PNG`, `SAVE_DATA_PNG`, and `PIC1_PNG` PKG body entries are emitted from the same normalized file map.
- User-supplied images and automatic cover art take precedence over generated fallback PNGs.

## Consequences

The installed app filesystem now exposes the `sce_sys` files that a runtime process may open from `/app0`. This removes another likely `CE-30002-5` missing-file class while preserving the PKG metadata entries needed by the installer.

Fallback artwork is intentionally simple and deterministic. It is not a replacement for game-specific art, but it prevents network or user input failures from producing structurally incomplete packages.
