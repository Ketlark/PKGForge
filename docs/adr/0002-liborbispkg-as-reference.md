# Port from LibOrbisPkg (C#) as reference — not from PkgTool.Core

We port from LibOrbisPkg's C# source (the library), not from PkgTool.Core (the CLI tool). LibOrbisPkg is the authoritative open-source implementation with clean class separation. PkgTool.Core is a thin CLI wrapper around it.

This matters because PkgTool.Core has its own conventions (e.g. always wrapping inner PFS in PFSCReader) that aren't part of the format itself but are de facto requirements for compatibility.
