# PFS key derivation uses little-endian index — not big-endian

`PfsGenCryptoKey(ekpfs, seed, index)` constructs an HMAC input as `[index_bytes(4)] || seed`. The index is encoded in **little-endian** because C#'s `BitConverter.GetBytes(uint)` produces LE bytes on all platforms.

This is different from `ComputeKeys`, where the index is encoded big-endian (because C# explicitly calls `.Reverse()` on the BitConverter output). The inconsistency exists in the original C# code — we preserve it.

Getting this wrong produces PFS encryption keys that don't match the reader's expectations, resulting in "inode 0 is corrupt" errors from XTS garbage decryption. This was the root cause of the phase-4 blocker.
