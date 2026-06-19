package fpkg

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const pkgStreamThreshold = 32 << 20 // 32 MiB

type outerPFSSource struct {
	Data []byte
	Path string
	Size uint64
}

type pkgLayout struct {
	entries         []*pkgEntry
	bodyOffset      uint64
	bodySize        uint64
	pfsImageOffset  uint64
	pfsSize         uint64
	packageSize     uint64
	mainEntDataSize uint32
}

func outerPFSSize(src outerPFSSource) (uint64, error) {
	if src.Size > 0 {
		return src.Size, nil
	}
	if len(src.Data) > 0 {
		return uint64(len(src.Data)), nil
	}
	if src.Path != "" {
		info, err := os.Stat(src.Path)
		if err != nil {
			return 0, err
		}
		return uint64(info.Size()), nil
	}
	return 0, fmt.Errorf("fpkg: outer PFS source is empty")
}

func shouldStreamPKGAssembly(src outerPFSSource) (bool, error) {
	size, err := outerPFSSize(src)
	if err != nil {
		return false, err
	}
	return size >= pkgStreamThreshold || src.Path != "", nil
}

func buildPKGEntries(opts PKGOptions, sfo *ParamSfo, sfoData []byte, ekpfs []byte, pfsSize, innerPFSSize uint64) (*pkgLayout, error) {
	bodyOffset := uint64(0x2000)

	var entries []*pkgEntry
	entryKeys := buildEntryKeys(opts.ContentID, opts.Passcode)
	entries = append(entries, &pkgEntry{
		id: EntryIDEntryKeys, data: entryKeys, flags1: 0x60000000,
	})

	imageKey := RSA2048EncryptKey(FakeKeyset.Modulus, ekpfs)
	entries = append(entries, &pkgEntry{
		id: EntryIDImageKey, data: padTo(imageKey, 256), flags1: 0xE0000000, flags2: 3 << 12,
	})

	gdData := make([]byte, 0x180)
	binary.BigEndian.PutUint16(gdData[0:], 0xD256)
	binary.BigEndian.PutUint16(gdData[2:], 0x100)
	entries = append(entries, &pkgEntry{
		id: EntryIDGeneralDigests, data: gdData, flags1: 0x60000000,
	})
	entries = append(entries,
		&pkgEntry{id: EntryIDMetas, flags1: 0x60000000},
		&pkgEntry{id: EntryIDDigests, flags1: 0x40000000},
	)
	entryNames := buildEntryNames(entries)
	entries = append(entries, &pkgEntry{
		id: EntryIDEntryNames, data: entryNames, flags1: 0x40000000,
	})
	entries = append(entries,
		&pkgEntry{
			id: EntryIDPlayGoChunkDat, name: "playgo-chunk.dat",
			data: buildPlayGoChunkDat(opts.ContentID, 0, innerPFSSize),
		},
		&pkgEntry{id: EntryIDPlayGoChunkSha, name: "playgo-chunk.sha"},
		&pkgEntry{
			id: EntryIDPlayGoManifest, name: "playgo-manifest.xml",
			data: defaultPlayGoManifest(),
		},
	)

	licenseDat, err := buildLicenseDat(opts.ContentID)
	if err != nil {
		return nil, err
	}
	entries = append(entries,
		&pkgEntry{id: EntryIDLicenseDat, data: licenseDat, flags1: 0x80000000, flags2: 3 << 12},
		&pkgEntry{id: EntryIDLicenseInfo, data: buildLicenseInfo(opts.ContentID), flags1: 0x80000000, flags2: 2 << 12},
		&pkgEntry{id: EntryIDParamSfo, name: "param.sfo", data: sfoData},
		&pkgEntry{id: EntryIDPSReservedDat, data: make([]byte, 0x2000)},
	)

	for path, data := range opts.Files {
		if len(path) > 8 && path[:8] == "sce_sys/" {
			fileName := path[8:]
			if fileName == "param.sfo" || fileName == "keystone" {
				continue
			}
			if id, ok := entryNameToID[fileName]; ok {
				entries = append(entries, &pkgEntry{id: id, name: fileName, data: data})
			}
		}
	}

	nameTable := make([]byte, 1)
	nameOffsets := map[string]uint32{"": 0}
	for _, e := range entries {
		if e.name == "" {
			continue
		}
		if _, ok := nameOffsets[e.name]; ok {
			continue
		}
		nameOffsets[e.name] = uint32(len(nameTable))
		nameTable = append(nameTable, []byte(e.name)...)
		nameTable = append(nameTable, 0)
	}
	for _, e := range entries {
		if e.id == EntryIDEntryNames {
			e.data = nameTable
		}
	}

	updatePlayGoChunkShaSize(entries, bodyOffset, pfsSize)

	var metas []metaEntry
	dataOffset := uint32(bodyOffset)
	for _, e := range entries {
		size := uint32(len(e.data))
		if e.id == EntryIDMetas || e.id == EntryIDDigests {
			size = uint32(len(entries)) * 32
		}
		aligned := align64(uint64(size), 16)
		metas = append(metas, metaEntry{
			id: e.id, nameTableOffset: nameOffsets[e.name],
			flags1: e.flags1, flags2: e.flags2,
			dataOffset: dataOffset, dataSize: size,
		})
		e.metaOffset = dataOffset
		e.metaSize = size
		e.nameTableOffset = nameOffsets[e.name]
		e.dataOffset = dataOffset
		e.dataSize = size
		e.encrypted = (e.flags1 & 0x80000000) != 0
		dataOffset += uint32(aligned)
	}
	sortMetas(metas)

	metasData := make([]byte, len(entries)*32)
	for i, m := range metas {
		mw := newBytesWriteSeeker(metasData[i*32 : (i+1)*32])
		m.writeTo(mw)
	}
	for _, e := range entries {
		switch e.id {
		case EntryIDMetas:
			e.data = metasData
		case EntryIDDigests:
			e.data = make([]byte, len(entries)*32)
		}
	}

	bodySize := align64(uint64(dataOffset), 0x80000) - bodyOffset
	pfsImageOffset := bodyOffset + bodySize
	packageSize := pfsImageOffset + pfsSize
	if chunkDat := findEntry(entries, EntryIDPlayGoChunkDat); chunkDat != nil {
		chunkDat.data = buildPlayGoChunkDat(opts.ContentID, packageSize, innerPFSSize)
	}
	sfo.Set("PUBTOOLINFO", buildPubToolInfo(packageSize), 0x200)
	finalSFOData := sfo.Serialize()
	paramEntry := findEntry(entries, EntryIDParamSfo)
	if uint32(len(finalSFOData)) != paramEntry.dataSize {
		return nil, fmt.Errorf("fpkg: final param.sfo size changed from %d to %d", paramEntry.dataSize, len(finalSFOData))
	}
	paramEntry.data = finalSFOData

	return &pkgLayout{
		entries:         entries,
		bodyOffset:      bodyOffset,
		bodySize:        bodySize,
		pfsImageOffset:  pfsImageOffset,
		pfsSize:         pfsSize,
		packageSize:     packageSize,
		mainEntDataSize: calcMainEntryDataSize(entries),
	}, nil
}

func writePKGBodyEntries(w io.WriteSeeker, entries []*pkgEntry, opts PKGOptions) error {
	for _, e := range entries {
		seekTo(w, int64(e.dataOffset))
		if e.encrypted {
			if err := writeEncryptedEntry(w, e, opts.ContentID, opts.Passcode); err != nil {
				return err
			}
			continue
		}
		if _, err := w.Write(e.data); err != nil {
			return err
		}
	}
	return nil
}

// assemblePKGToFile writes a complete PKG without holding the full image in memory.
func assemblePKGToFile(outputPath string, opts PKGOptions, sfo *ParamSfo, sfoData []byte, outer outerPFSSource, ekpfs []byte, innerPFSSize uint64) error {
	pfsSize, err := outerPFSSize(outer)
	if err != nil {
		return err
	}

	layout, err := buildPKGEntries(opts, sfo, sfoData, ekpfs, pfsSize, innerPFSSize)
	if err != nil {
		return err
	}

	pkgFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("fpkg: create pkg: %w", err)
	}
	defer pkgFile.Close()

	if err := pkgFile.Truncate(int64(layout.packageSize)); err != nil {
		return fmt.Errorf("fpkg: truncate pkg: %w", err)
	}

	writePKGHeader(pkgFile, opts, len(layout.entries), layout.mainEntDataSize, layout.bodyOffset, layout.bodySize, layout.pfsImageOffset, layout.pfsSize, layout.packageSize)
	if err := writePKGBodyEntries(pkgFile, layout.entries, opts); err != nil {
		return err
	}

	var pfsDigests pfsDigestResult
	switch {
	case outer.Path != "":
		pfsDigests, err = copyOuterPFSToPKG(pkgFile, int64(layout.pfsImageOffset), outer.Path, int64(layout.pfsSize))
	case len(outer.Data) > 0:
		if _, err := pkgFile.Seek(int64(layout.pfsImageOffset), io.SeekStart); err != nil {
			return err
		}
		if _, err := pkgFile.Write(outer.Data); err != nil {
			return err
		}
		pfsDigests = digestPFSSlice(outer.Data)
	default:
		return fmt.Errorf("fpkg: outer PFS source is empty")
	}
	if err != nil {
		return err
	}

	if err := writePlayGoChunkSHAAt(pkgFile, layout.entries, layout.pfsImageOffset, layout.packageSize); err != nil {
		return err
	}
	if err := calculateDigestsAt(pkgFile, int64(layout.packageSize), layout.entries, opts.ContentID, sfo, layout.bodyOffset, layout.bodySize, layout.pfsImageOffset, layout.pfsSize, pfsDigests); err != nil {
		return err
	}

	writePKGHeader(pkgFile, opts, len(layout.entries), layout.mainEntDataSize, layout.bodyOffset, layout.bodySize, layout.pfsImageOffset, layout.pfsSize, layout.packageSize)
	if err := finalizePKGHeaderAt(pkgFile); err != nil {
		return err
	}
	return pkgFile.Close()
}

func calculateDigestsAt(pkg io.ReaderAt, pkgLen int64, entries []*pkgEntry, contentID string, sfo *ParamSfo, bodyOffset, bodySize, pfsImageOffset, pfsSize uint64, pfsDigests pfsDigestResult) error {
	w, ok := pkg.(io.WriterAt)
	if !ok {
		return fmt.Errorf("fpkg: pkg reader is not writable")
	}

	sortedEntries := sortedEntriesByID(entries)

	if generalEntry := findEntry(entries, EntryIDGeneralDigests); generalEntry != nil {
		paramEntry := findEntry(entries, EntryIDParamSfo)
		paramData, err := readPKGRange(pkg, int64(paramEntry.dataOffset), int64(paramEntry.dataSize))
		if err != nil {
			return err
		}
		headerPrefix, err := readPKGRange(pkg, 0, 0x480)
		if err != nil {
			return err
		}
		generalData := buildGeneralDigests(headerPrefix, contentID, sfo, paramData, pfsDigests.FullDigest)
		if err := writePKGRange(pkg, int64(generalEntry.dataOffset), generalData); err != nil {
			return err
		}
		generalEntry.data = generalData
	}

	digestEntry := findEntry(entries, EntryIDDigests)
	if digestEntry != nil {
		digestData := make([]byte, len(sortedEntries)*pkgHashSize)
		for i := 1; i < len(sortedEntries); i++ {
			e := sortedEntries[i]
			data, err := readPKGRange(pkg, int64(e.dataOffset), int64(e.dataSize))
			if err != nil {
				return err
			}
			copy(digestData[i*pkgHashSize:(i+1)*pkgHashSize], Sha256(data))
		}
		if err := writePKGRange(pkg, int64(digestEntry.dataOffset), digestData); err != nil {
			return err
		}
		digestEntry.data = digestData
		if _, err := w.WriteAt(Sha256(digestData), 0x140); err != nil {
			return err
		}
	}

	bodyData, err := readPKGRange(pkg, int64(bodyOffset), int64(bodySize))
	if err != nil {
		return err
	}
	if _, err := w.WriteAt(Sha256(bodyData), 0x160); err != nil {
		return err
	}

	if scEntries1, err := concatEntryDataAt(pkg, entries, []uint32{
		EntryIDEntryKeys, EntryIDImageKey, EntryIDGeneralDigests, EntryIDMetas, EntryIDDigests,
	}, false); err != nil {
		return err
	} else if len(scEntries1) > 0 {
		if _, err := w.WriteAt(Sha256(scEntries1), 0x100); err != nil {
			return err
		}
	}
	if scEntries2, err := concatEntryDataAt(pkg, entries, []uint32{
		EntryIDEntryKeys, EntryIDImageKey, EntryIDGeneralDigests, EntryIDMetas,
	}, true); err != nil {
		return err
	} else if len(scEntries2) > 0 {
		if _, err := w.WriteAt(Sha256(scEntries2), 0x120); err != nil {
			return err
		}
	}

	if _, err := w.WriteAt(pfsDigests.FullDigest, 0x440); err != nil {
		return err
	}
	_, err = w.WriteAt(pfsDigests.SignedDigest, 0x460)
	return err
}

func readPKGRange(pkg io.ReaderAt, offset, size int64) ([]byte, error) {
	if size == 0 {
		return nil, nil
	}
	buf := make([]byte, size)
	if _, err := pkg.ReadAt(buf, offset); err != nil {
		return nil, err
	}
	return buf, nil
}

func writePKGRange(pkg io.ReaderAt, offset int64, data []byte) error {
	w, ok := pkg.(io.WriterAt)
	if !ok {
		return fmt.Errorf("fpkg: pkg reader is not writable")
	}
	_, err := w.WriteAt(data, offset)
	return err
}

func concatEntryDataAt(pkg io.ReaderAt, entries []*pkgEntry, ids []uint32, sc2MetasSize bool) ([]byte, error) {
	var out []byte
	for _, id := range ids {
		e := findEntry(entries, id)
		if e == nil {
			return nil, nil
		}
		size := e.dataSize
		if sc2MetasSize && id == EntryIDMetas {
			size = 6 * 0x20
		}
		data, err := readPKGRange(pkg, int64(e.dataOffset), int64(size))
		if err != nil {
			return nil, err
		}
		out = append(out, data...)
	}
	return out, nil
}
