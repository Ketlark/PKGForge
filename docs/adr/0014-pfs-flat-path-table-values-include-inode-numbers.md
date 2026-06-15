# ADR 0014: PFS flat path table values include inode numbers

Date: 2026-06-13

## Status

Accepted

## Context

PS1 fPKGs installed successfully but still failed at launch with `CE-30002-5`. Klog showed the failing system call:

`sceFsMountGamePkg() -> sceKernelOpen() failed 0x80020002 /mnt/sandbox/pfsmnt/SLES02887-app0-nest/pfs_image.dat`

That means the failure happened while mounting game data, before the PS1HD `eboot.bin` could run. PkgTool.Core could still validate and extract the package, including `pfs_image.dat`, so a simple file-presence check was not enough.

Comparing with LibOrbisPkg showed that `flat_path_table` entries must map `hash(fullPath)` to `inodeNumber | directoryFlag`. PKG Forge wrote only the directory flag, leaving file entries as `0`. For `pfs_image.dat`, that made the fast path lookup point at inode 0 instead of the file inode.

## Decision

PKG Forge now writes the inode number into every non-colliding flat path table value:

- files: `inodeNumber`
- directories: `inodeNumber | 0x20000000`

A regression test asserts that `/pfs_image.dat` maps to its file inode rather than `0`.

## Consequences

The PS4 kernel lookup path for `/app0-nest/pfs_image.dat` now matches LibOrbisPkg. This addresses the klog-observed `CE-30002-5` path where PkgTool.Core extraction succeeded but runtime `sceKernelOpen()` returned `ENOENT`.

PkgTool.Core validation remains useful, but it is not sufficient evidence that the kernel flat-path lookup will succeed.
