# PKG entry encryption uses AES-128-CBC with no padding

Encrypted PKG entries (IMAGE_KEY, LICENSE_DAT, LICENSE_INFO) are encrypted with AES-128-CBC using `PaddingMode.None`. The entry data must already be a multiple of 16 bytes.

The key and IV are derived from: `iv_key = SHA256(meta.GetBytes() || keySeed)`, where `keySeed` is either `ComputeKeys(cid, pass, keyIndex)` or the RSA-decrypted dk3 (for key index 3). The IV is `iv_key[0:16]` and the AES key is `iv_key[16:32]`.

This is important because `AES128CBCEncryptPad` (which zero-pads to block size) was initially used, but the C# code uses `PaddingMode.None` — the data is already aligned. Using padding would produce incorrect ciphertext.
