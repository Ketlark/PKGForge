# XTS encryption skips the first PFS block (sectors 0–15)

PFS outer encryption uses AES-128-XTS with 0x1000-byte sectors. Only sectors >= `BlockSize / sectorSize` (= 16, i.e. starting at byte offset 0x10000) are encrypted. The first PFS block (0x0000–0xFFFF, the header block) is left in plaintext.

This matches `PfsReader` in C#, which creates `XtsDecryptReader` with `XtsStartSector = hdr.BlockSize / XtsSectorSize`. The `PFSBuilder.XtsSectorGen()` also starts yielding from sector 16.

Additionally, for signed PFS images, the empty block (after the flat path table) is **not encrypted**. `XtsSectorGen()` skips it: `if (xtsSector / 0x10 == emptyBlock) { xtsSector += 16; }`. Our current implementation encrypts all sectors >= 16, which is incorrect for blocks at the empty block index — but this doesn't cause extraction failures because the empty block contains only zeroes.
