package fpkg

// This file implements the PFSC (PFS Compressed) wrapper.
// Ported from LibOrbisPkg/PFS/PFSCWriter.cs.
// PFSC adds a header with block offsets around a PFS image for seekable access.
// Data blocks are stored uncompressed (the reader handles both compressed and uncompressed).

import (
	"encoding/binary"
)

const pfscBlockSize = 0x10000

// wrapPFSC wraps a PFS image in a PFSC container.
// Ported from PFSCWriter.
func wrapPFSC(data []byte) []byte {
	size := int64(len(data))
	numBlocks := (size + pfscBlockSize - 1) / pfscBlockSize

	// Calculate header size
	pointerTableSize := 8 + numBlocks*8
	additionalPointerBlocks := (pointerTableSize - 0xFC00 + 0xFFFF) / 0x10000
	if additionalPointerBlocks < 0 {
		additionalPointerBlocks = 0
	}
	hdrSize := int64(0x10000)
	if additionalPointerBlocks > 0 {
		hdrSize += int64(pfscBlockSize) * additionalPointerBlocks
	}

	totalSize := hdrSize + numBlocks*pfscBlockSize
	buf := make([]byte, totalSize)
	w := newBytesWriteSeeker(buf)

	// PFSC header (0x30 bytes)
	// Magic: "PFSC" as big-endian int32 (0x50465343)
	var magic [4]byte
	binary.BigEndian.PutUint32(magic[:], 0x50465343)
	w.Write(magic[:])
	writeLE(w, int32(0))          // Unk4
	writeLE(w, int32(6))          // Unk8
	writeLE(w, int32(pfscBlockSize)) // BlockSz
	writeLE(w, int64(pfscBlockSize)) // BlockSz2
	writeLE(w, int64(0x400))      // BlockOffsets pointer
	writeLE(w, int64(hdrSize))    // DataStart
	writeLE(w, int64(numBlocks*pfscBlockSize)) // DataLength = padded size (matches C# PFSCWriter)

	// Block offset table at 0x400
	seekTo(w, 0x400)
	for i := int64(0); i <= numBlocks; i++ {
		writeLE(w, int64(hdrSize+i*pfscBlockSize))
	}

	// Copy data blocks at DataStart
	seekTo(w, hdrSize)
	w.Write(data)

	return buf
}
