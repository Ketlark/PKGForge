# Inner PFS wrapped in PFSC — not raw

The inner PFS image is wrapped in a PFSC container before being stored as `pfs_image.dat` in the outer PFS. This is required for PkgTool.Core compatibility — it always accesses `pfs_image.dat` through `PFSCReader`.

PFSC adds a 0x10000-byte header containing magic (`PFSC`), block size, a block offset table, and a data-length field. Data blocks are stored uncompressed (each block is exactly 0x10000 bytes, matching `sectorMap[i+1] - sectorMap[i] == BlockSize` for the fast path in PFSCReader).

The outer PFS inode for `pfs_image.dat` stores:
- `size` = total PFSC-wrapped size (header + padded data blocks)
- `size_compressed` = original inner PFS size (before PFSC wrapping)
- `flags` includes the compressed bit (`0x1`)

This matches the C# `FSFile(PfsBuilder)` constructor which sets `Compress = true`, `_compressedSize = b.CalculatePfsSize()`, and `Size = _compressedSize + pfsc.HeaderSize`.
