// Package fpkg implements PS4 fPKG creation from PS1 and PS2 disc images.
//
// The pipeline is:
//  1. Parse disc image → extract Game ID and title
//  2. Build project directory (emulator files + game data)
//  3. Generate param.sfo (SFO writer)
//  4. Build PFS image (PFS builder with signed inodes)
//  5. Compress PFS with PFSC (zlib block compression)
//  6. Assemble PKG (header + entries + encrypted PFS)
//
// Ported from LibOrbisPkg (C#, LGPL v3) by maxton.
// Keys from flatz's fPKG writeup and the PS4 homebrew community.
package fpkg
