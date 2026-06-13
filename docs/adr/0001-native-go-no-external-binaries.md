# Full native Go reimplementation — no external binaries

The entire fPKG pipeline (disc parsing, SFO generation, PFS construction, PKG assembly, RSA/AES/XTS crypto) is implemented in pure Go with zero external binary dependencies. The reference implementation is LibOrbisPkg (C#), which we ported function-by-function.

The alternative was shelling out to PkgTool.Core (a .NET binary requiring Rosetta on ARM macOS) or requiring Wine on Linux/Windows. Both add fragile runtime dependencies and make progress reporting harder. The trade-off is that our crypto must be byte-compatible with the C# implementation — any endianness or padding mismatch produces undecryptable PKGs.

Validation against PkgTool.Core (`pkg_extract`, `pkg_extractinnerpfs`, `pfs_extract`) is done in tests but not at runtime.
