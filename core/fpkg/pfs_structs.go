package fpkg

// This file implements PFS filesystem structures: header, inodes, dirents.
// Ported from LibOrbisPkg/PFS/PfsStructs.cs.

import (
	"encoding/binary"
	"io"
	"strings"
	"unicode"
)

// ---------------------------------------------------------------------------
// PFS mode flags
// ---------------------------------------------------------------------------

const (
	PfsModeSigned          uint16 = 0x1
	PfsModeIs64Bit         uint16 = 0x2
	PfsModeEncrypted       uint16 = 0x4
	PfsModeUnknownFlag     uint16 = 0x8
)

// ---------------------------------------------------------------------------
// Inode mode and flags
// ---------------------------------------------------------------------------

const (
	InodeModeORead    uint16 = 1
	InodeModeOWrite   uint16 = 2
	InodeModeOExec    uint16 = 4
	InodeModeGRead    uint16 = 8
	InodeModeGWrite   uint16 = 16
	InodeModeGExec    uint16 = 32
	InodeModeURead    uint16 = 64
	InodeModeUWrite   uint16 = 128
	InodeModeUExec    uint16 = 256
	InodeModeDir      uint16 = 16384
	InodeModeFile     uint16 = 32768

	InodeModeRXOnly   uint16 = InodeModeORead | InodeModeOExec | InodeModeGRead | InodeModeGExec | InodeModeURead | InodeModeUExec
)

const (
	InodeFlagCompressed uint32 = 0x1
	InodeFlagUnk2       uint32 = 0x4
	InodeFlagUnk3       uint32 = 0x8
	InodeFlagReadonly   uint32 = 0x10
	InodeFlagInternal   uint32 = 0x20000
)

// ---------------------------------------------------------------------------
// Dirent types
// ---------------------------------------------------------------------------

const (
	DirentTypeFile      int32 = 2
	DirentTypeDirectory int32 = 3
	DirentTypeDot       int32 = 4
	DirentTypeDotDot    int32 = 5
)

// ---------------------------------------------------------------------------
// PfsHeader — the PFS superblock
// ---------------------------------------------------------------------------

const pfsHeaderMagic int64 = 20130315

type pfsHeader struct {
	Version          int64
	Magic            int64
	Id               int64
	Fmode            uint8
	Clean            uint8
	ReadOnly         uint8
	Mode             uint16
	Unk1             uint16
	BlockSize        uint32
	NBackup          uint32
	NBlock           int64
	DinodeCount      int64
	Ndblock          int64
	DinodeBlockCount int64
	InodeBlockSig    dinodeS64
	UnknownIndex     int32
	Seed             []byte // 16 bytes, nil if no seed
}

func newPfsHeader(blockSize uint32, sign, encrypt bool, seed []byte) *pfsHeader {
	mode := PfsModeUnknownFlag
	if sign {
		mode |= PfsModeSigned
	}
	if encrypt {
		mode |= PfsModeEncrypted
	}
	return &pfsHeader{
		Version:   1,
		Magic:     pfsHeaderMagic,
		BlockSize: blockSize,
		ReadOnly:  1,
		Mode:      mode,
		NBlock:    1,
		InodeBlockSig: dinodeS64{
			Nlink: 1,
			Flags: InodeFlagReadonly,
			Size:  int64(blockSize),
			SizeCompressed: int64(blockSize),
			Blocks: 1,
		},
		UnknownIndex: 1,
		Seed:         seed,
	}
}

func (h *pfsHeader) writeTo(w io.WriteSeeker) {
	start, _ := w.Seek(0, io.SeekCurrent)
	writeLE(w, h.Version)
	writeLE(w, h.Magic)
	writeLE(w, h.Id)
	writeByte(w, h.Fmode)
	writeByte(w, h.Clean)
	writeByte(w, h.ReadOnly)
	writeByte(w, 0)   // Rsv: single byte padding, NOT uint16
	writeUint16LE(w, h.Mode)
	writeUint16LE(w, h.Unk1)
	writeUint32LE(w, h.BlockSize)
	writeUint32LE(w, h.NBackup)
	writeLE(w, h.NBlock)
	writeLE(w, h.DinodeCount)
	writeLE(w, h.Ndblock)
	writeLE(w, h.DinodeBlockCount)
	writeLE(w, int64(0)) // padding

	h.InodeBlockSig.writeTo(w)

	if h.Seed != nil {
		w.Seek(start+0x36C, io.SeekStart)
		writeLE(w, h.UnknownIndex)
		w.Write(h.Seed)
	} else {
		w.Seek(start+0x368, io.SeekStart)
		writeLE(w, int32(1))
	}
}

// ---------------------------------------------------------------------------
// DinodeS64 — signed 64-bit inode (used in header)
// ---------------------------------------------------------------------------

const dinodeS64Size = 0x310

type dinodeS64 struct {
	Mode           uint16
	Nlink          uint16
	Flags          uint32
	Size           int64
	SizeCompressed int64
	Time1Sec       int64
	Time2Sec       int64
	Time3Sec       int64
	Time4Sec       int64
	Time1Nsec      uint32
	Time2Nsec      uint32
	Time3Nsec      uint32
	Time4Nsec      uint32
	Uid            uint32
	Gid            uint32
	Unk1           uint64
	Unk2           uint64
	Blocks         uint32
	db             [12]blockSig64
	ib             [5]blockSig64
}

type blockSig64 struct {
	sig   [32]byte
	block int64
}

func (d *dinodeS64) setTime(t int64) {
	d.Time1Sec = t
	d.Time2Sec = t
	d.Time3Sec = t
	d.Time4Sec = t
}

func (d *dinodeS64) setDirectBlock(idx int, block int64) {
	d.db[idx].block = block
}

func (d *dinodeS64) writeTo(w io.Writer) {
	writeUint16LE(w, d.Mode)
	writeUint16LE(w, d.Nlink)
	writeUint32LE(w, d.Flags)
	writeLE(w, d.Size)
	writeLE(w, d.SizeCompressed)
	writeLE(w, d.Time1Sec)
	writeLE(w, d.Time2Sec)
	writeLE(w, d.Time3Sec)
	writeLE(w, d.Time4Sec)
	writeUint32LE(w, d.Time1Nsec)
	writeUint32LE(w, d.Time2Nsec)
	writeUint32LE(w, d.Time3Nsec)
	writeUint32LE(w, d.Time4Nsec)
	writeUint32LE(w, d.Uid)
	writeUint32LE(w, d.Gid)
	writeLE(w, d.Unk1)
	writeLE(w, d.Unk2)
	writeUint32LE(w, d.Blocks)
	writeLE(w, int32(0)) // padding for 64-bit alignment
	for i := 0; i < 12; i++ {
		w.Write(d.db[i].sig[:])
		writeLE(w, d.db[i].block)
	}
	for i := 0; i < 5; i++ {
		w.Write(d.ib[i].sig[:])
		writeLE(w, d.ib[i].block)
	}
}

// ---------------------------------------------------------------------------
// DinodeD32 — unsigned 32-bit inode
// ---------------------------------------------------------------------------

const dinodeD32Size = 0xA8

type dinodeD32 struct {
	Mode           uint16
	Nlink          uint16
	Flags          uint32
	Size           int64
	SizeCompressed int64
	Time1Sec       int64
	Time2Sec       int64
	Time3Sec       int64
	Time4Sec       int64
	Time1Nsec      uint32
	Time2Nsec      uint32
	Time3Nsec      uint32
	Time4Nsec      uint32
	Uid            uint32
	Gid            uint32
	Unk1           uint64
	Unk2           uint64
	Blocks         uint32
	db             [12]int32
	ib             [5]int32
	number         uint32 // not written to disk, used for dirent references
}

func (d *dinodeD32) startBlock() int32 { return d.db[0] }

func (d *dinodeD32) setDirectBlock(idx int, block int32) {
	d.db[idx] = block
}

func (d *dinodeD32) setTime(t int64) {
	d.Time1Sec = t
	d.Time2Sec = t
	d.Time3Sec = t
	d.Time4Sec = t
}

func (d *dinodeD32) writeTo(w io.Writer) {
	writeUint16LE(w, d.Mode)
	writeUint16LE(w, d.Nlink)
	writeUint32LE(w, d.Flags)
	writeLE(w, d.Size)
	writeLE(w, d.SizeCompressed)
	writeLE(w, d.Time1Sec)
	writeLE(w, d.Time2Sec)
	writeLE(w, d.Time3Sec)
	writeLE(w, d.Time4Sec)
	writeUint32LE(w, d.Time1Nsec)
	writeUint32LE(w, d.Time2Nsec)
	writeUint32LE(w, d.Time3Nsec)
	writeUint32LE(w, d.Time4Nsec)
	writeUint32LE(w, d.Uid)
	writeUint32LE(w, d.Gid)
	writeLE(w, d.Unk1)
	writeLE(w, d.Unk2)
	writeUint32LE(w, d.Blocks)
	for i := 0; i < 12; i++ {
		writeLE(w, d.db[i])
	}
	for i := 0; i < 5; i++ {
		writeLE(w, d.ib[i])
	}
}

// ---------------------------------------------------------------------------
// DinodeS32 — signed 32-bit inode
// ---------------------------------------------------------------------------

const dinodeS32Size = 0x2C8

type dinodeS32 struct {
	Mode           uint16
	Nlink          uint16
	Flags          uint32
	Size           int64
	SizeCompressed int64
	Time1Sec       int64
	Time2Sec       int64
	Time3Sec       int64
	Time4Sec       int64
	Time1Nsec      uint32
	Time2Nsec      uint32
	Time3Nsec      uint32
	Time4Nsec      uint32
	Uid            uint32
	Gid            uint32
	Unk1           uint64
	Unk2           uint64
	Blocks         uint32
	db             [12]blockSig
	ib             [5]blockSig
	number         uint32 // not written to disk, used for dirent references
}

type blockSig struct {
	sig   [32]byte
	block int32
}

func (d *dinodeS32) startBlock() int32 { return d.db[0].block }

func (d *dinodeS32) setDirectBlock(idx int, block int32) {
	d.db[idx].block = block
}

func (d *dinodeS32) setTime(t int64) {
	d.Time1Sec = t
	d.Time2Sec = t
	d.Time3Sec = t
	d.Time4Sec = t
}

func (d *dinodeS32) writeTo(w io.Writer) {
	writeUint16LE(w, d.Mode)
	writeUint16LE(w, d.Nlink)
	writeUint32LE(w, d.Flags)
	writeLE(w, d.Size)
	writeLE(w, d.SizeCompressed)
	writeLE(w, d.Time1Sec)
	writeLE(w, d.Time2Sec)
	writeLE(w, d.Time3Sec)
	writeLE(w, d.Time4Sec)
	writeUint32LE(w, d.Time1Nsec)
	writeUint32LE(w, d.Time2Nsec)
	writeUint32LE(w, d.Time3Nsec)
	writeUint32LE(w, d.Time4Nsec)
	writeUint32LE(w, d.Uid)
	writeUint32LE(w, d.Gid)
	writeLE(w, d.Unk1)
	writeLE(w, d.Unk2)
	writeUint32LE(w, d.Blocks)
	for i := 0; i < 12; i++ {
		w.Write(d.db[i].sig[:])
		writeLE(w, d.db[i].block)
	}
	for i := 0; i < 5; i++ {
		w.Write(d.ib[i].sig[:])
		writeLE(w, d.ib[i].block)
	}
}

// ---------------------------------------------------------------------------
// PfsDirent
// ---------------------------------------------------------------------------

const pfsDirentMaxSize = 280

type pfsDirent struct {
	InodeNumber uint32
	Type        int32
	Name        string
}

func (d *pfsDirent) entSize() int {
	sz := len(d.Name) + 17
	if sz%8 != 0 {
		sz += 8 - (sz % 8)
	}
	return sz
}

func (d *pfsDirent) writeTo(w io.Writer) {
	nameLen := len(d.Name)
	entSz := d.entSize()
	writeUint32LE(w, d.InodeNumber)
	writeLE(w, d.Type)
	writeLE(w, int32(nameLen))
	writeLE(w, int32(entSz))
	w.Write([]byte(d.Name))
	// Pad remaining
	remaining := entSz - 16 - nameLen
	if remaining > 0 {
		padding := make([]byte, remaining)
		w.Write(padding)
	}
}

// ---------------------------------------------------------------------------
// Flat path table
// ---------------------------------------------------------------------------

func pfsHashFunction(name string) uint32 {
	var hash uint32
	for _, c := range name {
		hash = uint32(unicode.ToUpper(c)) + 31*hash
	}
	return hash
}

// ---------------------------------------------------------------------------
// Binary write helpers
// ---------------------------------------------------------------------------

func writeByte(w io.Writer, v uint8) {
	w.Write([]byte{v})
}

func writeUint16LE(w io.Writer, v uint16) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	w.Write(buf[:])
}

func writeUint32LE(w io.Writer, v uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	w.Write(buf[:])
}

func writeLE(w io.Writer, v interface{}) {
	switch v := v.(type) {
	case int32:
		writeUint32LE(w, uint32(v))
	case uint32:
		writeUint32LE(w, v)
	case int64:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(v))
		w.Write(buf[:])
	case uint64:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], v)
		w.Write(buf[:])
	case uint16:
		writeUint16LE(w, v)
	case int16:
		writeUint16LE(w, uint16(v))
	}
}

// seekTo sets position on a WriteSeeker. Panics on error.
func seekTo(w io.WriteSeeker, offset int64) {
	_, err := w.Seek(offset, io.SeekStart)
	if err != nil {
		panic("fpkg: seek: " + err.Error())
	}
}

// hasCollision checks if any two nodes have the same path hash.
func hasCollision(paths []string) bool {
	seen := make(map[uint32]bool)
	for _, p := range paths {
		h := pfsHashFunction(p)
		if seen[h] {
			return true
		}
		seen[h] = true
	}
	return false
}

// normalizePath returns the full path within the image (lowercase, forward slashes).
func normalizePath(parts ...string) string {
	return strings.Join(parts, "/")
}
