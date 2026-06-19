package fpkg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
)

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

const (
	pfscBlockSize         = 0x10000
	pfscParallelMinBlocks = 64
)

type pfscBlockPlan struct {
	onDiskSize int64
}

type pfscBlockWork struct {
	idx        int64
	data       []byte
	onDiskSize int64
	compressed bool
}

var pfscBlockDataPool sync.Pool

func getPFSCBlockBuf() []byte {
	if v := pfscBlockDataPool.Get(); v != nil {
		return v.([]byte)
	}
	return make([]byte, pfscBlockSize)
}

func releasePFSCBlockData(data []byte) {
	if len(data) == pfscBlockSize && cap(data) >= pfscBlockSize {
		pfscBlockDataPool.Put(data[:pfscBlockSize])
	}
}

// wrapPFSC wraps a PFS image in a PFSC container with per-block zlib compression.
// For large images prefer wrapPFSCToFile to avoid holding encoded blocks in memory.
func wrapPFSC(data []byte) []byte {
	out, err := wrapPFSCFromReader(bytes.NewReader(data), int64(len(data)), nil, false)
	if err != nil {
		panic("fpkg: wrapPFSC: " + err.Error())
	}
	return out
}

// wrapPFSCToFile compresses a PFS image on disk into a PFSC file without
// loading the source or all encoded blocks into memory.
func wrapPFSCToFile(srcPath, dstPath string, skipCompress bool) (int64, error) {
	info, err := os.Stat(srcPath)
	if err != nil {
		return 0, fmt.Errorf("pfsc: stat source: %w", err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return 0, fmt.Errorf("pfsc: open source: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return 0, fmt.Errorf("pfsc: create output: %w", err)
	}

	if _, err := wrapPFSCFromReader(src, info.Size(), dst, skipCompress); err != nil {
		dst.Close()
		os.Remove(dstPath)
		return 0, err
	}
	if err := dst.Close(); err != nil {
		return 0, err
	}
	info, err = os.Stat(dstPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// wrapPFSCFromReader compresses a PFS image in a single pass: each 64 KiB block
// is read and compressed once, written immediately, and the header is patched at
// the end from the recorded per-block sizes.
func wrapPFSCFromReader(src io.ReaderAt, size int64, dst io.WriterAt, skipCompress bool) ([]byte, error) {
	if size < 0 {
		return nil, fmt.Errorf("pfsc: negative source size")
	}

	numBlocks := (size + pfscBlockSize - 1) / pfscBlockSize
	hdrSize := pfscHeaderSize(numBlocks)
	plans := make([]pfscBlockPlan, numBlocks)

	var buf []byte
	var out io.WriterAt
	if dst == nil {
		body, plans, err := compressPFSCBody(src, size, numBlocks)
		if err != nil {
			return nil, err
		}
		buf = make([]byte, int(hdrSize)+len(body))
		out = newBytesWriteSeeker(buf)
		if err := writePFSCHeader(out, numBlocks, hdrSize, plans); err != nil {
			return nil, err
		}
		if _, err := out.WriteAt(body, hdrSize); err != nil {
			return nil, err
		}
		return buf, nil
	}

	if file, ok := dst.(*os.File); ok {
		// Pre-size generously; truncate down after the single write pass.
		if err := file.Truncate(size + hdrSize); err != nil {
			return nil, fmt.Errorf("pfsc: truncate output: %w", err)
		}
	}
	out = dst

	if err := writePFSCBlocks(out, src, size, numBlocks, hdrSize, plans, skipCompress); err != nil {
		return nil, err
	}

	var totalSize int64 = hdrSize
	for _, plan := range plans {
		totalSize += plan.onDiskSize
	}

	if file, ok := dst.(*os.File); ok {
		if err := file.Truncate(totalSize); err != nil {
			return nil, fmt.Errorf("pfsc: truncate final output: %w", err)
		}
	}

	if err := writePFSCHeader(out, numBlocks, hdrSize, plans); err != nil {
		return nil, err
	}
	return nil, nil
}

func compressPFSCBody(src io.ReaderAt, size, numBlocks int64) ([]byte, []pfscBlockPlan, error) {
	plans := make([]pfscBlockPlan, numBlocks)
	var body bytes.Buffer
	emit := func(idx int64, data []byte, onDiskSize int64) error {
		plans[idx].onDiskSize = onDiskSize
		_, err := body.Write(data)
		releasePFSCBlockData(data)
		return err
	}
	if err := encodePFSCBlocks(src, size, numBlocks, false, emit); err != nil {
		return nil, nil, err
	}
	return body.Bytes(), plans, nil
}

func pfscHeaderSize(numBlocks int64) int64 {
	pointerTableSize := 8 + numBlocks*8
	additionalPointerBlocks := (pointerTableSize - 0xFC00 + 0xFFFF) / 0x10000
	if additionalPointerBlocks < 0 {
		additionalPointerBlocks = 0
	}
	hdrSize := int64(0x10000)
	if additionalPointerBlocks > 0 {
		hdrSize += int64(pfscBlockSize) * additionalPointerBlocks
	}
	return hdrSize
}

func writePFSCHeader(out io.WriterAt, numBlocks, hdrSize int64, plans []pfscBlockPlan) error {
	header := make([]byte, 0x400)
	var magic [4]byte
	binary.BigEndian.PutUint32(magic[:], 0x50465343) // "PFSC"
	copy(header[0:4], magic[:])
	putLE32(header[4:8], 0)
	putLE32(header[8:12], 6)
	putLE32(header[12:16], pfscBlockSize)
	putLE64(header[16:24], pfscBlockSize)
	putLE64(header[24:32], 0x400)
	putLE64(header[32:40], hdrSize)
	putLE64(header[40:48], numBlocks*pfscBlockSize)
	if _, err := out.WriteAt(header, 0); err != nil {
		return err
	}

	offsetTable := make([]byte, 8+len(plans)*8)
	offset := hdrSize
	for i, plan := range plans {
		putLE64(offsetTable[i*8:(i+1)*8], offset)
		offset += plan.onDiskSize
	}
	putLE64(offsetTable[len(offsetTable)-8:], offset)
	if _, err := out.WriteAt(offsetTable, 0x400); err != nil {
		return err
	}
	return nil
}

func writePFSCBlocks(out io.WriterAt, src io.ReaderAt, size, numBlocks, hdrSize int64, plans []pfscBlockPlan, skipCompress bool) error {
	dataOffset := hdrSize
	emit := func(idx int64, data []byte, onDiskSize int64) error {
		plans[idx].onDiskSize = onDiskSize
		_, err := out.WriteAt(data, dataOffset)
		dataOffset += onDiskSize
		releasePFSCBlockData(data)
		return err
	}
	return encodePFSCBlocks(src, size, numBlocks, skipCompress, emit)
}

func encodePFSCBlocks(src io.ReaderAt, size, numBlocks int64, skipCompress bool, emit func(idx int64, data []byte, onDiskSize int64) error) error {
	if numBlocks >= pfscParallelMinBlocks && runtime.NumCPU() >= 2 {
		return parallelPFSCEncode(src, size, numBlocks, skipCompress, emit)
	}
	return sequentialPFSCEncode(src, size, numBlocks, skipCompress, emit)
}

func sequentialPFSCEncode(src io.ReaderAt, size, numBlocks int64, skipCompress bool, emit func(idx int64, data []byte, onDiskSize int64) error) error {
	blockBuf := make([]byte, pfscBlockSize)
	compressMisses := 0

	for i := int64(0); i < numBlocks; i++ {
		bw, err := readPFSCBlock(src, size, i, blockBuf, skipCompress)
		if err != nil {
			return err
		}
		if err := emit(i, bw.data, bw.onDiskSize); err != nil {
			return err
		}
		if bw.compressed {
			compressMisses = 0
		} else {
			compressMisses++
			if compressMisses >= 8 {
				skipCompress = true
			}
		}
	}
	return nil
}

func parallelPFSCEncode(src io.ReaderAt, size, numBlocks int64, initialSkip bool, emit func(idx int64, data []byte, onDiskSize int64) error) error {
	workers := runtime.NumCPU()
	jobs := make(chan int64, workers*2)
	results := make(chan pfscBlockWork, workers*2)

	var skip atomic.Bool
	skip.Store(initialSkip)

	var wg sync.WaitGroup
	var firstErr atomic.Value

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			blockBuf := make([]byte, pfscBlockSize)
			for idx := range jobs {
				if firstErr.Load() != nil {
					return
				}
				bw, err := readPFSCBlock(src, size, idx, blockBuf, skip.Load())
				if err != nil {
					firstErr.Store(err)
					return
				}
				results <- bw
			}
		}()
	}

	go func() {
		for i := int64(0); i < numBlocks; i++ {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	pending := make(map[int64]pfscBlockWork, workers*2)
	next := int64(0)
	compressMisses := 0

	for next < numBlocks {
		if err, _ := firstErr.Load().(error); err != nil {
			return err
		}
		for {
			bw, ready := pending[next]
			if !ready {
				break
			}
			if err := emit(next, bw.data, bw.onDiskSize); err != nil {
				return err
			}
			if bw.compressed {
				compressMisses = 0
			} else {
				compressMisses++
				if compressMisses >= 8 {
					skip.Store(true)
				}
			}
			delete(pending, next)
			next++
		}
		if next >= numBlocks {
			break
		}
		bw, ok := <-results
		if !ok {
			return fmt.Errorf("pfsc: missing block %d", next)
		}
		pending[bw.idx] = bw
	}
	return nil
}

func readPFSCBlock(src io.ReaderAt, size, idx int64, blockBuf []byte, skipCompress bool) (pfscBlockWork, error) {
	start := idx * pfscBlockSize
	end := start + pfscBlockSize
	if end > size {
		end = size
	}
	blockLen := int(end - start)
	if _, err := src.ReadAt(blockBuf[:blockLen], start); err != nil {
		return pfscBlockWork{}, fmt.Errorf("pfsc: read block %d: %w", idx, err)
	}

	compressed := compressPFSCBlockWithSkip(blockBuf[:blockLen], skipCompress)
	if compressed != nil && int64(len(compressed)) < pfscBlockSize {
		data := make([]byte, len(compressed))
		copy(data, compressed)
		return pfscBlockWork{idx: idx, data: data, onDiskSize: int64(len(data)), compressed: true}, nil
	}

	data := getPFSCBlockBuf()
	clear(data)
	copy(data, blockBuf[:blockLen])
	return pfscBlockWork{idx: idx, data: data, onDiskSize: pfscBlockSize, compressed: false}, nil
}

func putLE32(dst []byte, v int32) {
	binary.LittleEndian.PutUint32(dst, uint32(v))
}

func putLE64(dst []byte, v int64) {
	binary.LittleEndian.PutUint64(dst, uint64(v))
}
