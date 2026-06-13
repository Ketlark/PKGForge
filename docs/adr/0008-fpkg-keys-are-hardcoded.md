# fPKG keys are hardcoded — all known and public

All RSA key pairs and symmetric keys used for fPKG creation are hardcoded in `keys.go`:

- **FakeKeyset**: RSA key pair for EKPFS encryption (IMAGE_KEY double-encryption)
- **PkgDerivedKey3Keyset**: RSA key pair for dk3 encryption (ENTRY_KEYS key index 3)
- **PkgPublicKeys**: 7 RSA public key moduli (indices 0–6, some nil) for ENTRY_KEYS encryption
- **DefaultPasscode**: 32 null bytes (standard fPKG passcode)
- **Keystone HMAC keys**: `keystone_hmac_key`, `keystone_mac_data`

These are the same keys used by every fPKG tool. They are public knowledge and cannot be used for legitimate PKG signing — only for creating fPKGs that work on modified consoles.
