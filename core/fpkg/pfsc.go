package fpkg

import "encoding/binary"

// This file implements the PFSC (PFS Compressed) wrapper.
//
// PFSC adds a header with per-block offsets around a PFS image for seekable
// access. Each 64 KiB block may be stored uncompressed or zlib-compressed
// (windowBits=12, matching the PS4 kernel's decompressor).
//
// Block classification (by on-disk size = offset[i+1] - offset[i]):
//   - size == BlockSize → uncompressed (copy as-is)
//   - size <  BlockSize → compressed: zlib stream (2-byte header + deflate + adler32)
//   - size >  BlockSize → zero/sparse block (not used by us)

const pfscBlockSize = 0x10000

// wrapPFSC wraps a PFS image in a PFSC container with per-block zlib
// compression (windowBits=12). Blocks that don't benefit from compression
// are stored uncompressed.
func wrapPFSC(data []byte) []byte {
	size := int64(len(data))
	numBlocks := (size + pfscBlockSize - 1) / pfscBlockSize

	// Compress each block and collect the encoded data.
	encoded := make([][]byte, numBlocks)
	var totalDataSize int64

	for i := int64(0); i < numBlocks; i++ {
		start := i * pfscBlockSize
		end := start + pfscBlockSize
		if end > size {
			end = size
		}
		block := data[start:end]

		// Try zlib compression with windowBits=12
		compressed := compressPFSCBlock(block)
		if compressed != nil && int64(len(compressed)) < pfscBlockSize {
			encoded[i] = compressed
		} else {
			// Store uncompressed, padded to full block size
			buf := make([]byte, pfscBlockSize)
			copy(buf, block)
			encoded[i] = buf
		}
		totalDataSize += int64(len(encoded[i]))
	}

	// Header size calculation (PFSCWriter.cs:26-32)
	pointerTableSize := 8 + numBlocks*8
	additionalPointerBlocks := (pointerTableSize - 0xFC00 + 0xFFFF) / 0x10000
	if additionalPointerBlocks < 0 {
		additionalPointerBlocks = 0
	}
	hdrSize := int64(0x10000)
	if additionalPointerBlocks > 0 {
		hdrSize += int64(pfscBlockSize) * additionalPointerBlocks
	}

	totalSize := hdrSize + totalDataSize
	buf := make([]byte, totalSize)
	w := newBytesWriteSeeker(buf)

	// PFSC header (0x30 bytes)
	var magic [4]byte
	binary.BigEndian.PutUint32(magic[:], 0x50465343) // "PFSC"
	w.Write(magic[:])
	writeLE(w, int32(0))                       // Unk4
	writeLE(w, int32(6))                       // Unk8
	writeLE(w, int32(pfscBlockSize))           // BlockSz
	writeLE(w, int64(pfscBlockSize))           // BlockSz2
	writeLE(w, int64(0x400))                   // BlockOffsets pointer
	writeLE(w, int64(hdrSize))                 // DataStart
	writeLE(w, int64(numBlocks*pfscBlockSize)) // DataLength (logical uncompressed size)

	// Block offset table at 0x400 — variable-size entries
	seekTo(w, 0x400)
	offset := hdrSize
	for i := int64(0); i < numBlocks; i++ {
		writeLE(w, int64(offset))
		offset += int64(len(encoded[i]))
	}
	writeLE(w, int64(offset)) // sentinel: end of last block

	// Write encoded blocks at DataStart
	seekTo(w, hdrSize)
	for i := int64(0); i < numBlocks; i++ {
		w.Write(encoded[i])
	}

	return buf
}
