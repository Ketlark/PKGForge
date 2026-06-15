# ADR 0009: Signed outer PFS layout matches LibOrbisPkg

Date: 2026-06-13

## Status

Accepted

## Context

PS4 installation rejected generated PS1 fPKGs even after PkgTool.Core reported all package hashes and signatures as valid. Earlier errors included content digest and entry issues. The remaining install failure traced to signed outer PFS layout.

PkgTool.Core validates the PKG header, entry digests, PFS image digest, and signatures, but it can still extract a package whose signed PFS omits indirect signature blocks. LibOrbisPkg allocates those blocks for files larger than the inode direct block slots. A real PS1 package with a large `pfs_image.dat` needs those indirect signature blocks, and their absence changes `pfs_image_size`, total package size, and later entry offsets.

## Decision

PKG Forge mirrors LibOrbisPkg's signed outer PFS layout:

- reserve direct data block signatures in signed inodes;
- allocate single and double indirect signature blocks when file data exceeds direct block capacity;
- sign data blocks first, then sign indirect blocks, inode blocks, super root, flat path table, and header-level signatures;
- reserve the LibOrbisPkg empty block after the flat path table;
- derive final PFS size from the full signed block graph, not only from file data blocks.

Tests lock the minimal fPKG package size and `pfs_image_size`, then run PkgTool.Core validation. Manual comparison also checks `pkg_listentries` and total package size against a LibOrbisPkg/PkgTool.Core reference package.

## Consequences

Generated PS1 fPKGs now match LibOrbisPkg entry offsets and total package size for the tested Spider-Man PS1 package. The minimal package also matches the LibOrbisPkg reference size.

The PFS builder carries more signing state: data signatures, final signatures, indirect block counts, and a skipped empty block. This is less compact than the earlier direct-block-only implementation, but it documents the format behavior that the PS4 installer appears to require.

## Alternatives Considered

### Trust PkgTool.Core validation alone

`pkg_validate --verbose` did not catch the missing indirect PFS signature blocks. It remains useful, but it cannot be the only compatibility gate.

### Sign only direct data blocks

This kept the implementation simpler and produced extractable packages, but large packages had smaller PFS images than LibOrbisPkg output and failed PS4 installation.

### Shell out to PkgTool.Core

This would avoid porting the PFS layout, but it violates the native Go decision and would make progress reporting, cancellation, and cross-platform packaging dependent on external binaries.
