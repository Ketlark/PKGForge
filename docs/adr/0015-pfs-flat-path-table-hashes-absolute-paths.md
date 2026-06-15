# ADR 0015: PFS flat path table hashes absolute paths

Date: 2026-06-13

## Status

Accepted

## Context

After fixing flat path table values to include inode numbers, PS1 fPKGs still installed but failed at launch with `CE-30002-5`. Klog continued to show:

`sceKernelOpen() failed 0x80020002 /mnt/sandbox/pfsmnt/SLES02887-app0-nest/pfs_image.dat`

Binary inspection showed that `pfs_image.dat` was present in `uroot`, the inode and PFSC header were valid, and every direct/indirect PFS block signature verified. The remaining mismatch appeared only when comparing a LibOrbisPkg-rebuilt package: LibOrbis wrote the flat path table key `0x7a61bfd3`, which is `hash("/pfs_image.dat")`; PKG Forge wrote `0x02a9bba2`, which is `hash("pfs_image.dat")`.

PkgTool.Core can still extract by walking dirents, so this mismatch is invisible to ordinary extraction and validation.

## Decision

PKG Forge now computes `fsNode.fullPath()` like LibOrbisPkg: the root node has no name, and every child path is returned with a leading slash.

Examples:

- root child: `/pfs_image.dat`
- nested file: `/sce_sys/param.sfo`

Flat path table keys hash those absolute PFS paths. The value still stores `inodeNumber | directoryFlag`.

## Consequences

The PS4 kernel fast lookup for `/app0-nest/pfs_image.dat` now uses the same flat path table key as LibOrbisPkg.

The regression test now checks `hash("/pfs_image.dat")` and explicitly rejects the old relative `hash("pfs_image.dat")` key.
