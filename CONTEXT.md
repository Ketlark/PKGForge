# PKG Forge

Desktop app for merging, splitting, inspecting, and creating PlayStation package files (PS4/PS5). Written in Go + Svelte via Wails v2.

## Language

### Package formats

**PKG**:
A PlayStation archive format used for digital distribution on PS4 and PS5. Contains a header, entry metadata, encrypted entries, and an embedded PFS image.
_Avoid_: package, archive

**fPKG**:
A fake PKG — a PKG built with publicly known keys (FakeKeyset) rather than Sony's private keys. Used for homebrew and custom game installs on modified consoles.
_Avoid_: fake pkg, custom pkg

**Split PKG**:
A PKG file divided into sequential parts for distribution (e.g. `Game_001.pkgpart`, `Game.pkg.001`). PKG Forge can re-merge them.

**Disc image**:
A logical optical disc source used to build a PS1 or PS2 fPKG. For CUE/BIN dumps, the `.cue` file and every `.bin` file it references describe one disc.
_Avoid_: treating each companion `.bin` file as a separate disc.

**Disc set**:
The ordered set of logical discs for a multi-disc game. Disc 2 means the second CD or DVD in the game, not the second file referenced by a CUE sheet.
_Avoid_: file set

### PFS layering

**PFS** (PlayStation File System):
The filesystem used inside PKG files. A FAT-like structure with inodes, dirents, and a flat path table. Supports signed and encrypted modes.

**Inner PFS**:
The PFS image containing the actual game files (disc images, `sce_sys/`, `keystone`). Always unsigned and unencrypted. Stored as `pfs_image.dat` inside the outer PFS.

**Outer PFS**:
The PFS image embedded directly in the PKG. Signed and encrypted. Contains exactly one file: `pfs_image.dat` (which is the PFSC-wrapped inner PFS).

**PFSC** (PFS Compressed):
A seekable compression wrapper around PFS data. Consists of a header with per-block offset table followed by data blocks. Blocks may be individually deflated or stored uncompressed. Required because PkgTool.Core unconditionally reads `pfs_image.dat` through a PFSCReader.

### PKG internals

**Content ID**:
A 36-character identifier for a PKG (e.g. `UP9000-SLUS00100_00-TESTGAME00000001`). Encodes region, title ID, and discriminator.

**EKPFS** (Encrypted Key for PFS):
The 32-byte key used to derive PFS encryption and signing keys. For fPKGs, EKPFS = `ComputeKeys(contentID, passcode, 1)`.

**Passcode**:
A 32-character string used as the master key seed for fPKG key derivation. All-zeros by default for fPKGs.

**Entry**:
A named data section in a PKG body. Entries include ENTRY_KEYS, IMAGE_KEY, GENERAL_DIGESTS, METAS, DIGESTS, ENTRY_NAMES, LICENSE_DAT, LICENSE_INFO, PARAM_SFO, PSRESERVED_DAT, and optional `sce_sys/` files.

**IMAGE_KEY**:
A PKG entry containing the EKPFS, double-encrypted: first RSA-2048 with FakeKeyset, then AES-128-CBC with a key derived from dk3 and the entry's meta bytes. The reader reverses this chain to recover the EKPFS.

**ENTRY_KEYS**:
A PKG entry containing 7 RSA-encrypted derived keys and their digests. Key index 3 (dk3) is RSA-encrypted with PkgDerivedKey3Keyset; decrypting it yields the seed for IMAGE_KEY decryption.

**Meta entry**:
A 32-byte big-endian record in the METAS table describing one PKG entry: id, name table offset, flags1, flags2, data offset, data size. The flags encode encryption status and key index.

**PlayGo metadata**:
PKG body metadata that describes install chunks and their SHA values. Even single-image fPKGs carry PlayGo metadata so the PS4 installer can reason about package chunks.
_Avoid_: progress bar data

**Debug RIF**:
The debug license blob used by fPKG packages. It is part of package installability, not proof of retail entitlement.
_Avoid_: retail license

**Application Type**:
The `param.sfo` field that tells the PS4 launcher what commercial model the application uses. Base PS1/PS2 fPKGs use "Paid standalone full app", not "Freemium app".
_Avoid_: content category

**PS1HD runtime config**:
The `config-title.txt` file read by the PS1HD emulator at launch. It must include the disc image path (`--image="data/disc1.bin"`) and can include PS1HD flags such as `--ps1-title-id`, `--title-id`, `--region`, and `--sim-analog-pad`. With bundled OpenBIOS, it must not include `--bios-hide-sce-osd`.
_Avoid_: assuming `config-emu-ps4.txt` is used by PS1HD.

**PS1HD runtime modules**:
The PRX modules loaded by the PS1HD emulator from `/app0/sce_module/`, notably `libc.prx`, `libSceFios2.prx`, and `libSceNpToolkit2.prx`.
_Avoid_: placing PS1HD PRX files at the `/app0` root.

**Runtime param.sfo**:
The `sce_sys/param.sfo` file available inside `/app0` at launch. It duplicates the PKG `PARAM_SFO` entry's game metadata role because the installer needs the PKG entry and the runtime filesystem may need the `/app0/sce_sys/param.sfo` path.
_Avoid_: treating the PKG body `PARAM_SFO` entry as sufficient for runtime file lookups.

**Runtime artwork**:
The `sce_sys/icon0.png`, `sce_sys/save_data.png`, and `sce_sys/pic1.png` files available inside `/app0` and also emitted as PKG body entries when applicable. They come from user inputs or automatic cover lookup first where applicable, then deterministic fallback PNGs.
_Avoid_: depending on network cover lookup for required `sce_sys` artwork.

**Bundled emulator assets**:
The runtime emulator files embedded in `core/fpkg/assets.dat` and extracted to the user config cache on first use. The cache is validated against the embedded manifest so app updates can refresh stale runtime files automatically.
_Avoid_: requiring users to provide a `ps1hd` directory for normal PS1 fPKG creation.

**PS1HD data layout**:
The PS1HD-compatible disc layout under `/app0/data/`. Each logical disc is stored as `discN.bin`, `discN.cue`, and `discN.toc`; `config-title.txt` points to `data/discN.bin`.
_Avoid_: using `/app0/image/disc01.bin` for PS1HD packages.

**PS1 cover art**:
The optional image used for `sce_sys/icon0.png`. PKG Forge resolves it from local files next to the CUE first, then known serial-based internet sources, and caches a normalized 512x512 PNG.
_Avoid_: blocking fPKG creation when cover lookup fails.

### PFS internals

**Inode**:
A PFS metadata record describing a file or directory. Two variants: `dinodeD32` (unsigned, 0xA8 bytes) and `dinodeS32` (signed, 0x2C8 bytes with per-block HMAC signatures). Each inode stores mode, size, block count, direct/indirect block pointers.

**Dinode**:
Synonym for inode within PFS terminology (the `d_` prefix stands for "disk").

**Dirent** (directory entry):
A variable-size record in a directory's data block. Contains inode number, type (file/dir/dot/dotdot), name, and padding. Alignment is to 8-byte boundaries.

**Super root**:
The outermost directory in a PFS image (inode 0). Contains `flat_path_table`, optionally `collision_resolver`, and `uroot`.

**uroot**:
The user-visible root directory inside a PFS image. Contains the actual game files. Its name is literally `uroot`.

**Flat path table**:
A hash table mapping path hashes to inode metadata. Used for fast path lookup within PFS.
Keys hash LibOrbis-style absolute PFS paths (`/pfs_image.dat`, `/sce_sys/param.sfo`), not relative names. For each non-colliding path, the value is `inodeNumber | directoryFlag`. A wrong key or a file value of `0` can make the PS4 kernel return `ENOENT` even if PkgTool.Core can list the file through dirents.
_Avoid_: hashing relative names, writing only the directory flag, or writing zero for files.

**Block signature**:
In signed PFS mode, each data block has a 32-byte HMAC-SHA256 signature. The signing key is derived from EKPFS and the PFS seed via `PfsGenSignKey`.

**Indirect block signature**:
A signed PFS signature entry for data blocks beyond the inode's direct block slots. Indirect signature blocks can themselves be signed.
_Avoid_: PKG header signature

### Crypto

**FakeKeyset**:
The RSA key pair used for fPKG encryption/decryption. Public modulus is well-known; private key allows both encryption and decryption.

**PkgDerivedKey3Keyset**:
The RSA key pair used for dk3 (derived key index 3). Used to encrypt/decrypt the seed that protects IMAGE_KEY.

**ComputeKeys**:
Key derivation function: `SHA256(SHA256(index_BE) || SHA256(contentID_padded_48) || passcode_bytes)`. Index 0 = passcode key, index 1 = EKPFS, index 3 = dk3.

**XTS** (XEX-based Tweaked-codebook mode):
The AES mode used for PFS encryption. Sectors 0–15 (first 64 KiB = header block) are left in plaintext. Data and tweak keys are derived from EKPFS via `PfsGenEncKey`.

## Relationships

- A **PKG** contains one **Outer PFS**
- An **Outer PFS** contains one file: `pfs_image.dat` (a **PFSC**-wrapped **Inner PFS**)
- An **Inner PFS** contains game files and `sce_sys/` metadata
- A **Disc set** contains one or more **Disc images**
- The **EKPFS** is derived from **Content ID** and **Passcode** via `ComputeKeys(cid, pass, 1)`
- **PFS encryption keys** (tweak, data) are derived from **EKPFS** and PFS **Seed** via HMAC-SHA256
- **ENTRY_KEYS** holds 7 RSA-encrypted derived keys; key[3] decrypts to dk3, which feeds **IMAGE_KEY** decryption
- **IMAGE_KEY** holds the **EKPFS**, double-encrypted (RSA then AES-CBC)
- **PlayGo metadata** and **Debug RIF** are PKG body entries required by PS4 install workflows
- **Application Type** must match the package's role at launch; base emulator fPKGs are standalone applications
- **Bundled emulator assets** provide PS1/PS2 runtimes by default
- **Runtime param.sfo** is duplicated into the **Inner PFS** even though PARAM_SFO is also a PKG body entry
- **Runtime artwork** is always present through user art, automatic cover art, or generated fallback PNGs
- **PS1HD runtime config** points the emulator to the packaged **Disc image**
- **PS1HD runtime modules** live under `/app0/sce_module/` inside the **Inner PFS**
- **PS1HD data layout** stores each PS1 **Disc image** with its CUE and TOC sidecars
- **PS1 cover art** may populate `sce_sys/icon0.png`, but it is not required for package creation

## Example dialogue

> **Dev:** "Why does `pkg_extract` fail with 'inode 0 is corrupt'?"
> **Domain expert:** "That means the XTS decryption produced garbage for the super root inode. Check that `PfsGenCryptoKey` uses little-endian for the HMAC index — C#'s `BitConverter` is LE."

> **Dev:** "Why do we PFSC-wrap the inner PFS if the data isn't compressed?"
> **Domain expert:** "Because PkgTool.Core always reads `pfs_image.dat` through `PFSCReader`. Without the PFSC header, it throws 'missing PFSC magic'. The blocks are stored uncompressed — the wrapper is purely structural."

> **Dev:** "The outer PFS inode for `pfs_image.dat` has size=71693312 but compressed_size=716172240. Why are they different?"
> **Domain expert:** "Size is the PFSC-wrapped size (inner PFS + PFSC header + padded blocks). Compressed_size is the original inner PFS size before PFSC wrapping. PkgTool.Core uses `compressed_size` when extracting via PFSCReader — it tells the reader the logical length of the uncompressed data."

> **Dev:** "I dropped both a CUE and a BIN. Should Disc 2 be the BIN?"
> **Domain expert:** "No. The CUE and its BIN files are one disc image. Disc 2 is only for the next logical game disc."

> **Dev:** "`pkg_validate` passes. Why can the PS4 installer still reject the package?"
> **Domain expert:** "PkgTool.Core validates many hashes and signatures, but it can miss signed outer PFS layout details such as missing indirect block signature blocks. Compare entry offsets, PFS size, and package size against LibOrbisPkg output."

> **Dev:** "The package installs but launch fails with CE-39929-2. What does that point to?"
> **Domain expert:** "The PS4 launcher is rejecting a freemium SKU mismatch. Check Application Type in param.sfo; base PS1/PS2 fPKGs should be paid standalone full apps."

> **Dev:** "The package installs but launch fails with CE-30002-5. What does that point to?"
> **Domain expert:** "Treat it as a missing or unreadable runtime file first. For PS1HD, check that `/app0/sce_module/libSceNpToolkit2.prx` exists, that `config-title.txt` contains `--image="data/disc1.bin"`, and that `/app0/data/disc1.bin`, `/app0/data/disc1.cue`, `/app0/data/disc1.toc`, `/app0/sce_sys/param.sfo`, `/app0/sce_sys/icon0.png`, `/app0/sce_sys/save_data.png`, and `/app0/sce_sys/pic1.png` are present."

> **Dev:** "The PS1 package reaches SaveData mount, repeats `0x809f8022`, then crashes with CE-34878-0 / SIGILL. What should we check first?"
> **Domain expert:** "Check that `sce_sys/save_data.png` exists in both `/app0/sce_sys` and the PKG body entries. PS1HD mounts `PS1.VMC` and `PS1HDSNAP` through the host save-data API before gameplay."

> **Dev:** "The PS1 package reaches `EmuCorePS1` then crashes with CE-34878-0 / SIGILL while OpenBIOS is bundled. What should we check first?"
> **Domain expert:** "Inspect `config-title.txt` for BIOS patch flags. With bundled OpenBIOS, do not emit `--bios-hide-sce-osd=1`; that flag targets Sony BIOS startup-logo instructions, not a replacement ROM."

> **Dev:** "Klog says `sceFsMountGamePkg` cannot open `/mnt/sandbox/pfsmnt/<TITLE>-app0-nest/pfs_image.dat`, but PkgTool.Core extracts it."
> **Domain expert:** "Check the outer PFS flat path table. The PS4 kernel lookup needs `hash('/pfs_image.dat')` to map to the file inode number. PkgTool.Core may still extract by walking dirents, so extraction alone does not prove runtime lookup works."

## Flagged ambiguities

- "PFS" is used loosely to mean either the filesystem format or a specific PFS image instance. Resolved: capitalize "PFS" for the format, use "inner PFS" / "outer PFS" for specific images.
- "Encryption key" was used ambiguously for both EKPFS and the derived XTS keys. Resolved: EKPFS is always "EKPFS", XTS keys are "tweak key" and "data key".
- "Signed" in PFS context means HMAC-signed blocks (not RSA-signed). Each block gets an HMAC-SHA256 signature using the PFS signing key.
- "Disc" was used ambiguously for files inside a CUE/BIN dump and logical game discs. Resolved: use "disc image" for one logical disc source and "disc set" for multi-disc games.
- "Application Type" should not be confused with `CATEGORY=gd` or PKG content type. It is the SFO app model used by the launcher.
- "PS1 emulator config" was used ambiguously for `config-emu-ps4.txt` and `config-title.txt`. Resolved: PS1HD uses `config-title.txt`; PS2 uses `config-emu-ps4.txt`.
- "PS1 disc file" was used ambiguously for either the raw BIN or the PS1HD runtime bundle. Resolved: PS1HD packages include the BIN plus package-local CUE and TOC sidecars under `data/`.
