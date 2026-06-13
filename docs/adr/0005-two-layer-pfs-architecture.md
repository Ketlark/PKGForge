# Two-layer PFS architecture — inner unsigned, outer signed+encrypted

fPKGs use two nested PFS images:

1. **Inner PFS** (unsigned, unencrypted): contains the game files (`image/disc01.bin`, `sce_sys/param.sfo`, `sce_sys/keystone`). Block size 0x10000, minimum 0x55 blocks.

2. **Outer PFS** (signed, encrypted): contains a single file `pfs_image.dat` (the PFSC-wrapped inner PFS). Signed with HMAC-SHA256 per block, encrypted with AES-128-XTS starting at sector 16.

The outer PFS seed is all-zeros (16 bytes). The encryption and signing keys are derived from the EKPFS via HMAC-SHA256 with specific index values: index 1 for encryption keys, index 2 for signing key.

This matches the C# `PfsProperties.MakeOuterPFSProps` and `PfsProperties.MakeInnerPFSProps` conventions.
