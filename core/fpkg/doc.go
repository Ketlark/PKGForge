// Package fpkg implements PS4 fPKG creation from PS1 and PS2 disc images.
//
// The pipeline is:
//  1. Parse disc image → extract Game ID and title
//  2. Build project directory (emulator files + game data)
//  3. Generate param.sfo (SFO writer)
//  4. Build inner PFS image
//  5. Wrap inner PFS as PFSC data
//  6. Build signed and encrypted outer PFS image
//  7. Assemble PKG entries, PlayGo metadata, Debug RIF, digests, and header
//
// The package writer follows LibOrbisPkg's layout, including signed PFS
// direct and indirect block signatures.
//
// Ported from LibOrbisPkg (C#, LGPL v3) by maxton.
// Keys from flatz's fPKG writeup and the PS4 homebrew community.
package fpkg
