# Inner PFS wrapped in PFSC — not raw

The inner PFS image is wrapped in a PFSC container before being stored as `pfs_image.dat` in the outer PFS. This is required for PkgTool.Core compatibility — it always accesses `pfs_image.dat` through `PFSCReader`.

PFSC adds a 0x10000-byte header containing magic (`PFSC`), block size, a block offset table, and a data-length field. Each 64 KiB block is zlib-compressed with `windowBits=12` (4 KiB window) via a CGO wrapper around `deflateInit2(level=6, Z_DEFLATED, 12, 8, Z_DEFAULT_STRATEGY)`, matching the PS4 kernel's PFSC decompressor (per flatz/pkg_pfs_tool `src/pfs.h:16`). Blocks that do not benefit from compression are stored uncompressed (on-disk size == BlockSize). The block offset table uses variable-size entries so the reader can distinguish compressed (< BlockSize) from uncompressed (== BlockSize) blocks by comparing consecutive offsets.

The outer PFS inode for `pfs_image.dat` stores:
- `size` = total PFSC-wrapped size (header + padded data blocks)
- `size_compressed` = original inner PFS size (before PFSC wrapping)
- `flags` includes the compressed bit (`0x1`)

This matches the C# `FSFile(PfsBuilder)` constructor which sets `Compress = true`, `_compressedSize = b.CalculatePfsSize()`, and `Size = _compressedSize + pfsc.HeaderSize`.
