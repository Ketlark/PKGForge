package fpkg

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
)

const pkgChunkSize = 0x10000

// pfsDigestResult holds PFS hashes computed while streaming the outer image.
type pfsDigestResult struct {
	SignedDigest []byte // first 64 KiB of the PFS image
	FullDigest   []byte // entire PFS image
}

// digestPFSSlice hashes an in-memory outer PFS image.
func digestPFSSlice(outerPFS []byte) pfsDigestResult {
	var res pfsDigestResult
	if len(outerPFS) == 0 {
		return res
	}
	signedLen := pkgChunkSize
	if len(outerPFS) < signedLen {
		signedLen = len(outerPFS)
	}
	res.SignedDigest = Sha256(outerPFS[:signedLen])
	res.FullDigest = Sha256(outerPFS)
	return res
}

// copyOuterPFSToPKG streams an outer PFS file into the PKG at pfsImageOffset.
func copyOuterPFSToPKG(pkgFile *os.File, pfsImageOffset int64, outerPath string, pfsSize int64) (pfsDigestResult, error) {
	src, err := os.Open(outerPath)
	if err != nil {
		return pfsDigestResult{}, fmt.Errorf("fpkg: open outer pfs: %w", err)
	}
	defer src.Close()

	if _, err := pkgFile.Seek(pfsImageOffset, io.SeekStart); err != nil {
		return pfsDigestResult{}, err
	}

	full := sha256.New()
	signed := sha256.New()
	signedWritten := 0

	buf := make([]byte, 1<<20)
	remaining := pfsSize
	for remaining > 0 {
		toRead := int64(len(buf))
		if toRead > remaining {
			toRead = remaining
		}
		n, err := io.ReadFull(src, buf[:toRead])
		if err != nil {
			return pfsDigestResult{}, fmt.Errorf("fpkg: read outer pfs: %w", err)
		}
		if _, err := pkgFile.Write(buf[:n]); err != nil {
			return pfsDigestResult{}, fmt.Errorf("fpkg: write pfs to pkg: %w", err)
		}
		if signedWritten < pkgChunkSize {
			need := pkgChunkSize - signedWritten
			if need > n {
				need = n
			}
			if need > 0 {
				signed.Write(buf[:need])
				signedWritten += need
			}
		}
		full.Write(buf[:n])
		remaining -= int64(n)
	}

	return pfsDigestResult{
		SignedDigest: signed.Sum(nil),
		FullDigest:   full.Sum(nil),
	}, nil
}

func writePlayGoChunkSha(pkg []byte, entries []*pkgEntry, pfsImageOffset, packageSize uint64) error {
	data, err := computePlayGoChunkSHA(bytesReaderAt{pkg}, int64(len(pkg)), pfsImageOffset, packageSize)
	if err != nil {
		return err
	}
	chunkSha := findEntry(entries, EntryIDPlayGoChunkSha)
	if chunkSha == nil {
		return nil
	}
	chunkSha.data = data
	copy(pkg[chunkSha.dataOffset:chunkSha.dataOffset+chunkSha.dataSize], data)
	return nil
}

func writePlayGoChunkSHAAt(pkgFile *os.File, entries []*pkgEntry, pfsImageOffset, packageSize uint64) error {
	data, err := computePlayGoChunkSHA(pkgFile, int64(packageSize), pfsImageOffset, packageSize)
	if err != nil {
		return err
	}
	chunkSha := findEntry(entries, EntryIDPlayGoChunkSha)
	if chunkSha == nil {
		return nil
	}
	chunkSha.data = data
	_, err = pkgFile.WriteAt(data, int64(chunkSha.dataOffset))
	return err
}

func computePlayGoChunkSHA(pkg io.ReaderAt, pkgLen int64, pfsImageOffset, packageSize uint64) ([]byte, error) {
	data := make([]byte, (packageSize/pkgChunkSize)*4)

	startChunk := pfsImageOffset / pkgChunkSize
	totalChunks := packageSize / pkgChunkSize
	if totalChunks <= startChunk {
		return data, nil
	}

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	if int(totalChunks-startChunk) < workers {
		workers = int(totalChunks - startChunk)
	}

	type job struct {
		index uint64
	}
	jobs := make(chan job, workers*2)
	var wg sync.WaitGroup
	var jobErr error
	var errMu sync.Mutex

	worker := func() {
		defer wg.Done()
		chunkBuf := make([]byte, pkgChunkSize)
		for j := range jobs {
			if j.index >= totalChunks {
				continue
			}
			offset := j.index * pkgChunkSize
			if offset >= uint64(pkgLen) {
				errMu.Lock()
				if jobErr == nil {
					jobErr = fmt.Errorf("fpkg: PlayGo chunk %d exceeds package size", j.index)
				}
				errMu.Unlock()
				return
			}
			chunkLen := int(pkgChunkSize)
			if int64(offset)+int64(chunkLen) > pkgLen {
				chunkLen = int(pkgLen - int64(offset))
			}
			n, err := pkg.ReadAt(chunkBuf[:chunkLen], int64(offset))
			if err != nil {
				errMu.Lock()
				if jobErr == nil {
					jobErr = fmt.Errorf("fpkg: read playgo chunk %d: %w", j.index, err)
				}
				errMu.Unlock()
				return
			}
			hash := Sha256(chunkBuf[:n])
			outOffset := j.index * 4
			if outOffset+4 > uint64(len(data)) {
				errMu.Lock()
				if jobErr == nil {
					jobErr = fmt.Errorf("fpkg: PlayGo SHA table too small for chunk %d", j.index)
				}
				errMu.Unlock()
				return
			}
			copy(data[outOffset:outOffset+4], hash[:4])
		}
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker()
	}
	for chunk := startChunk; chunk < totalChunks; chunk++ {
		jobs <- job{index: chunk}
	}
	close(jobs)
	wg.Wait()
	if jobErr != nil {
		return nil, jobErr
	}
	return data, nil
}

// bytesReaderAt adapts a byte slice for io.ReaderAt.
type bytesReaderAt struct {
	b []byte
}

func (r bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r.b)) {
		return 0, io.EOF
	}
	n := copy(p, r.b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// spillToTempFile writes data to a temp file and returns path + cleanup.
func spillToTempFile(prefix string, data []byte) (string, func(), error) {
	tmp, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", nil, err
	}
	path := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(path)
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(path)
		return "", nil, err
	}
	return path, func() { os.Remove(path) }, nil
}

func defaultPlayGoManifest() []byte {
	manifest := "<?xml version=\"1.0\" encoding=\"utf-8\" standalone=\"yes\"?>\r\n" +
		"<psproject fmt=\"playgo-manifest\" version=\"0990\">\r\n" +
		"  <volume>\r\n" +
		"    <chunk_info chunk_count=\"1\" scenario_count=\"1\">\r\n" +
		"      <scenarios default_id=\"0\">\r\n" +
		"        <scenario id=\"0\" type=\"sp\" initial_chunk_count=\"1\" label=\"Scenario #0\">0</scenario>\r\n" +
		"      </scenarios>\r\n" +
		"    </chunk_info>\r\n" +
		"  </volume>\r\n" +
		"</psproject>\r\n"
	return append([]byte{0xEF, 0xBB, 0xBF}, []byte(manifest)...)
}

func buildPlayGoChunkDat(contentID string, packageSize, innerPFSSize uint64) []byte {
	buf := make([]byte, 416)
	binary.LittleEndian.PutUint32(buf[0x00:], 0x6f676c70) // "plgo"
	binary.LittleEndian.PutUint16(buf[0x08:], 1)          // image_count
	binary.LittleEndian.PutUint16(buf[0x0A:], 1)          // chunk_count
	binary.LittleEndian.PutUint16(buf[0x0C:], 1)          // mchunk_count
	binary.LittleEndian.PutUint16(buf[0x0E:], 1)          // scenario_count
	binary.LittleEndian.PutUint32(buf[0x10:], uint32(len(buf)))
	binary.LittleEndian.PutUint16(buf[0x16:], 1) // attrib
	for i := 0x20; i < 0x40; i++ {
		buf[i] = 0xFF
	}
	copy(buf[0x40:], []byte(contentID))

	table := []struct {
		offset uint32
		size   uint32
	}{
		{0x100, 0x20},
		{0x120, 0x02},
		{0x130, 0x09},
		{0x140, 0x10},
		{0x160, 0x20},
		{0x180, 0x02},
		{0x190, 0x0C},
		{0x150, 0x10},
	}
	for i, item := range table {
		base := 0xC0 + i*8
		binary.LittleEndian.PutUint32(buf[base:], item.offset)
		binary.LittleEndian.PutUint32(buf[base+4:], item.size)
	}

	buf[0x100] = 0x80
	buf[0x102] = 0x03
	binary.LittleEndian.PutUint16(buf[0x10E:], 1)
	binary.LittleEndian.PutUint64(buf[0x110:], 0xFFFFFFFFFFFFFFFF)
	copy(buf[0x130:], []byte("Chunk #0"))

	binary.LittleEndian.PutUint64(buf[0x140:], 0)
	binary.LittleEndian.PutUint64(buf[0x148:], packageSize)
	binary.LittleEndian.PutUint64(buf[0x150:], 0)
	binary.LittleEndian.PutUint64(buf[0x158:], innerPFSSize)

	buf[0x160] = 1
	binary.LittleEndian.PutUint16(buf[0x174:], 1)
	binary.LittleEndian.PutUint16(buf[0x176:], 1)
	copy(buf[0x190:], []byte("Scenario #0"))
	return buf
}

func updatePlayGoChunkShaSize(entries []*pkgEntry, bodyOffset, pfsSize uint64) {
	chunkSha := findEntry(entries, EntryIDPlayGoChunkSha)
	if chunkSha == nil {
		return
	}

	size := uint32(0)
	for {
		chunkSha.data = make([]byte, size)
		packageSize := estimatePackageSizeFromEntries(entries, bodyOffset, pfsSize)
		nextSize := uint32(packageSize/0x10000) * 4
		if nextSize == size {
			return
		}
		size = nextSize
	}
}

func estimatePackageSizeFromEntries(entries []*pkgEntry, bodyOffset, pfsSize uint64) uint64 {
	dataOffset := bodyOffset
	for _, e := range entries {
		size := entryDataSizeForLayout(e, len(entries))
		dataOffset += align64(uint64(size), 16)
	}
	bodySize := align64(dataOffset, 0x80000) - bodyOffset
	return bodyOffset + bodySize + pfsSize
}

func entryDataSizeForLayout(e *pkgEntry, entryCount int) uint32 {
	switch e.id {
	case EntryIDMetas, EntryIDDigests:
		return uint32(entryCount) * 32
	default:
		return uint32(len(e.data))
	}
}

func finalizePKGHeaderAt(pkgFile *os.File) error {
	header := make([]byte, 0x1100)
	if _, err := pkgFile.ReadAt(header, 0); err != nil {
		return err
	}
	headerDigest := Sha256(header[0:0xFE0])
	copy(header[0xFE0:], headerDigest)
	headerSHA256 := Sha256(header[0:0x1000])
	copy(header[0x1000:], RSA2048EncryptKey(PkgPublicKeys[3], headerSHA256))
	_, err := pkgFile.WriteAt(header, 0)
	return err
}
