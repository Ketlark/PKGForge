# ADR 0012: PS1HD disc layout and cover art follow existing tools

Date: 2026-06-13

## Status

Accepted

## Context

After PS1 packages installed successfully, launching them could still fail with `CE-30002-5`. Sony describes that family of failures as corrupted application data, and PS4 error tables map `CE-30002-5` to `SCE_KERNEL_ERROR_ENOENT`, so missing runtime files remained the most likely cause.

Comparing existing PS1 fPKG tooling showed consistent PS1HD expectations:

- PS-Classics-fPKG-Builder packages PS1 discs under `data/` as `discN.bin` and `discN.cue`, writes `config-title.txt`, disables trophy/UDS features, and points `--image` at `data/discN.bin`.
- Cue2toc generates the binary `.toc` sidecar format used by PS1HD, including BCD track counts, A0/A1/A2 descriptors, track descriptors, pregap descriptors, and the two-second compatibility offset.
- PS1HD CLI documentation accepts repeated `--image` options, numbered `--image%d` options, title IDs, region, and emulator flags from `config-title.txt`.
- pop-fe uses local `<cue>_cover.png`-style overrides and PSXDataCenter for artwork.
- xlenore/psx-covers provides serial-based cover URLs that avoid scraping when a title ID is known.

## Decision

PKG Forge PS1 packages now:

- store each logical PS1 disc under `/app0/data/` as `discN.bin`, `discN.cue`, and `discN.toc`;
- rewrite CUE sheets so their `FILE` entry points to the package-local `discN.bin`;
- generate Cue2toc-compatible binary TOC sidecars for every PS1 disc;
- write `config-title.txt` with PS1HD-recognized options, including `--ps4-trophies=0`, `--ps5-uds=0`, `--trophies=0`, `--ps1-title-id`, `--title-id`, `--region`, and `--image="data/discN.bin"`;
- copy all available files from the bundled PS1HD emulator directory, while generated metadata, icons, config, and disc files take precedence;
- resolve cover art best-effort from local CUE-adjacent files, xlenore/psx-covers, then PSXDataCenter, caching the result as a 512x512 PNG.

## Consequences

The runtime layout now matches the public PS1HD-oriented tools more closely and removes another likely missing-file source for `CE-30002-5`.

Cover download failures do not block package creation. A user can still supply `icon0.png` manually, and local CUE-adjacent cover files override remote sources.

Runtime emulator files must be shipped through the bundled `assets.dat`; asking users to provide a `ps1hd` directory is only a development or diagnostic override. The asset cache is validated against the embedded bundle manifest so old caches are refreshed after a release repacks emulator files.

## Alternatives Considered

### Keep only a BIN under `image/`

This was simpler, but it did not match PS-Classics-style PS1HD projects and left no CUE/TOC sidecars for the emulator to consume.

### Generate TOC only for CDDA discs

Cue2toc-compatible TOC generation is cheap and deterministic. Always emitting the sidecar makes single-data-track and mixed-mode discs follow the same runtime contract.

### Require manual cover art

Manual cover selection is still supported, but automatic serial-based cover lookup gives normal PS1 workflows a usable icon without making the build depend on the network.
