# ADR 0017: Multi-bin CUE INDEX times are recalculated after merge

Date: 2026-06-15

## Status

Accepted

## Context

Single-track PS1 games (one `.bin` per disc) packaged correctly, but multi-track
games froze on the publisher logo with CDDA music continuing in the background.
The data track booted (logo + music), but every data read beyond track 1
returned wrong sectors, so the game stalled waiting for assets it could never
load.

Multi-track PS1 rips typically store each track in its own `.bin` file
(referenced by a single `.cue`). PKG Forge concatenates these files into one
`discN.bin` for the PS1HD data layout (ADR 0012).

The CUE sheet format defines **two time-base conventions**:

- **Single-FILE (single-bin):** All `INDEX` times are absolute disc positions
  within the one file. `INDEX 01 00:03:00` means sector 225 in the BIN.
- **Multi-FILE (multi-bin):** Each `FILE` directive starts a new time base.
  `INDEX` times are **relative to the start of that file**. `INDEX 01 00:00:00`
  on the third track means sector 0 of that track's `.bin`, not sector 0 of the
  disc.

The original `MergeBins` concatenated the raw bytes correctly, but
`RewritePS1CueForPackage` and `GeneratePS1TOC` used the parsed INDEX times
as-is. For multi-bin input, those times were per-file relative values; when
written into a single-FILE CUE, the emulator interpreted them as absolute
positions and read sectors at completely wrong offsets.

## Decision

PKG Forge now recalculates track positions after merging:

- **`ComputeMergedTracks(tracks)`** (`core/fpkg/disc.go`) detects multi-bin
  input, computes each file's absolute sector offset (via `os.Stat`, 2352
  bytes/sector with a 2048 fallback), and adjusts every track's `StartLBA` and
  `PregapLBA` to absolute positions within the concatenated image. Single-bin
  input is returned unchanged because the LBAs are already absolute.

- **`BuildPS1Project`** (`core/fpkg/ps1.go`) calls `ComputeMergedTracks` before
  `RewritePS1CueForPackage` and `GeneratePS1TOC`, for both the main disc and
  every extra disc. The rewritten CUE and generated TOC now reference the
  correct absolute sector positions.

- **`MergeBins`** now iterates over unique BIN files (via
  `getUniqueBinFiles`) rather than over tracks. This prevents duplicate copies
  when multiple tracks share a single file (single-bin layout passed to the
  multi-bin code path).

## Consequences

Multi-track PS1 games (CDDA audio, multi-session, LibCrypt) now produce
correct single-bin images whose CUE and TOC match the actual byte layout. The
PS1HD emulator reads sectors at the right offsets and games progress past the
publisher logo.

The recalculation adds one `os.Stat` call per unique BIN file, which is
negligible compared to the file I/O of the merge itself.

## Alternatives Considered

### Embed subchannel data and let the emulator reconstruct

Tools like CDMage can rebuild a disc image from subchannel data, but PS1 rips
in the wild rarely include `.sub` files. Reconstructing gaps from scratch would
require a full CD-ROM layout engine. Recalculating INDEX times from file sizes
is simpler, deterministic, and matches what every other single-bin converter
does internally.

### Reject multi-bin input and require users to pre-merge

This would shift the burden to users and break existing workflows. The merge
path was already present; only the metadata recalculation was missing.

### Store per-track files instead of merging

PS1HD expects `data/discN.bin` as a single image per disc (ADR 0012). Keeping
multiple track files would require a different runtime contract.
