package fpkg

// This file implements the PFS image builder.
// Ported from LibOrbisPkg/PFS/PFSBuilder.cs.
//
// PFS (PlayStation File System) is the filesystem used inside PKG files.
// An fPKG has two PFS layers:
//   - Inner PFS (unsigned, unencrypted): contains the actual game files
//   - Outer PFS (signed, encrypted): wraps the inner PFS as pfs_image.dat
//
// The builder constructs a complete PFS image from a filesystem tree,
// handling inode allocation, flat path table, dirents, signing, and encryption.

import (
	"encoding/binary"
	"io"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Filesystem tree types
// ---------------------------------------------------------------------------

// fsNode is a node (file or directory) in the PFS filesystem tree.
type fsNode struct {
	name           string
	parent         *fsNode
	isDir          bool
	children       []*fsNode // for dirs
	data           []byte    // for files (nil for dirs)
	compressedSize int64     // for PFSC-wrapped files: uncompressed size
	ino            inode
}

func (n *fsNode) fullPath() string {
	if n.parent == nil {
		return ""
	}
	pp := n.parent.fullPath()
	if pp == "" {
		return n.name
	}
	return pp + "/" + n.name
}

func (n *fsNode) size() int64 {
	if n.isDir {
		var total int64
		for _, c := range n.children {
			total += int64(direntSize(c.name))
		}
		return total
	}
	return int64(len(n.data))
}

func (n *fsNode) allDirs() []*fsNode {
	var result []*fsNode
	for _, c := range n.children {
		if c.isDir {
			result = append(result, c)
			result = append(result, c.allDirs()...)
		}
	}
	return result
}

func (n *fsNode) allFiles() []*fsNode {
	var result []*fsNode
	for _, c := range n.children {
		if !c.isDir {
			result = append(result, c)
		}
	}
	for _, c := range n.children {
		if c.isDir {
			result = append(result, c.allFiles()...)
		}
	}
	return result
}

// direntSize calculates the on-disk size of a dirent entry with the given name.
func direntSize(name string) int {
	sz := len(name) + 17
	if sz%8 != 0 {
		sz += 8 - (sz % 8)
	}
	return sz
}

// ---------------------------------------------------------------------------
// Inode interface (signed or unsigned)
// ---------------------------------------------------------------------------

type inode interface {
	writeTo(w io.Writer)
	startBlock() int32
	setDirectBlock(idx int, block int32)
	setTime(t int64)
	getBlocks() uint32
	setBlocks(b uint32)
	getSize() int64
	setSize(s int64)
	getSizeCompressed() int64
	setSizeCompressed(s int64)
	getFlags() uint32
	setFlags(f uint32)
	getNumber() uint32
	setNumber(n uint32)
	getNlink() uint16
	setNlink(n uint16)
}

func (d *dinodeD32) getBlocks() uint32          { return d.Blocks }
func (d *dinodeD32) setBlocks(b uint32)          { d.Blocks = b }
func (d *dinodeD32) getSize() int64              { return d.Size }
func (d *dinodeD32) setSize(s int64)              { d.Size = s }
func (d *dinodeD32) getSizeCompressed() int64    { return d.SizeCompressed }
func (d *dinodeD32) setSizeCompressed(s int64)    { d.SizeCompressed = s }
func (d *dinodeD32) getFlags() uint32            { return d.Flags }
func (d *dinodeD32) setFlags(f uint32)            { d.Flags = f }
func (d *dinodeD32) getNumber() uint32           { return d.number }
func (d *dinodeD32) setNumber(n uint32)           { d.number = n }
func (d *dinodeD32) getNlink() uint16            { return d.Nlink }
func (d *dinodeD32) setNlink(n uint16)            { d.Nlink = n }

func (d *dinodeS32) getBlocks() uint32          { return d.Blocks }
func (d *dinodeS32) setBlocks(b uint32)          { d.Blocks = b }
func (d *dinodeS32) getSize() int64              { return d.Size }
func (d *dinodeS32) setSize(s int64)              { d.Size = s }
func (d *dinodeS32) getSizeCompressed() int64    { return d.SizeCompressed }
func (d *dinodeS32) setSizeCompressed(s int64)    { d.SizeCompressed = s }
func (d *dinodeS32) getFlags() uint32            { return d.Flags }
func (d *dinodeS32) setFlags(f uint32)            { d.Flags = f }
func (d *dinodeS32) getNumber() uint32           { return d.number }
func (d *dinodeS32) setNumber(n uint32)           { d.number = n }
func (d *dinodeS32) getNlink() uint16            { return d.Nlink }
func (d *dinodeS32) setNlink(n uint16)            { d.Nlink = n }

// ---------------------------------------------------------------------------
// PFS builder
// ---------------------------------------------------------------------------

// PfsProperties holds the configuration for building a PFS image.
type PfsProperties struct {
	Root      *fsNode // filesystem tree root
	FileTime  int64   // Unix timestamp
	BlockSize uint32  // typically 0x10000
	MinBlocks uint32  // minimum number of blocks
	Encrypt   bool
	Sign      bool
	EKPFS     []byte // encryption key (16 bytes)
	Seed      []byte // seed for key derivation (16 bytes)
}

type blockSigInfo struct {
	block    int64
	sigOff   int64
	sigSize  int
}

// BuildPFSImage constructs a complete PFS image and returns it as a byte slice.
func BuildPFSImage(props PfsProperties) ([]byte, error) {
	if props.BlockSize == 0 {
		props.BlockSize = 0x10000
	}
	if props.FileTime == 0 {
		props.FileTime = time.Now().Unix()
	}

	// Collect filesystem nodes
	allDirs := props.Root.allDirs()
	allFiles := props.Root.allFiles()

	// Sort dirs and files alphabetically by full path for deterministic output
	sort.Slice(allDirs, func(i, j int) bool {
		return allDirs[i].fullPath() < allDirs[j].fullPath()
	})
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].fullPath() < allFiles[j].fullPath() // Actually we sort by fullpath later
	})

	// Combine all nodes for the image (dirs first, then files)
	var allNodes []*fsNode
	allNodes = append(allNodes, allDirs...)
	allNodes = append(allNodes, allFiles...)
	sort.Slice(allNodes, func(i, j int) bool {
		return allNodes[i].fullPath() < allNodes[j].fullPath()
	})

	// Check for hash collisions
	paths := make([]string, len(allNodes))
	for i, n := range allNodes {
		paths[i] = n.fullPath()
	}
	collision := hasCollision(paths)

	// Create header
	seed := props.Seed
	if !props.Encrypt && !props.Sign {
		seed = nil
	}
	hdr := newPfsHeader(props.BlockSize, props.Sign, props.Encrypt, seed)

	// Create inodes
	var inodes []inode
	inodeNum := uint32(0)

	makeInode := func(mode uint16, blocks uint32, size int64, flags uint32, nlink uint16) inode {
		var ino inode
		if props.Sign {
			d := &dinodeS32{
				Mode:   mode,
				Blocks: blocks,
				Size:   size,
				SizeCompressed: size,
				Nlink:  nlink,
				Flags:  flags | InodeFlagUnk2 | InodeFlagUnk3,
			}
			d.setTime(props.FileTime)
			ino = d
		} else {
			d := &dinodeD32{
				Mode:   mode,
				Blocks: blocks,
				Size:   size,
				SizeCompressed: size,
				Nlink:  nlink,
				Flags:  flags,
			}
			d.setTime(props.FileTime)
			ino = d
		}
		return ino
	}

	// Super root
	superRootIno := makeInode(InodeModeDir|InodeModeRXOnly, 1, 65536, InodeFlagInternal|InodeFlagReadonly, 1)
	superRootIno.setNumber(inodeNum)
	inodeNum++
	inodes = append(inodes, superRootIno)

	// Flat path table inode
	fptIno := makeInode(InodeModeFile|InodeModeRXOnly, 1, 0, InodeFlagInternal|InodeFlagReadonly, 1)
	fptIno.setNumber(inodeNum)
	inodeNum++
	inodes = append(inodes, fptIno)

	// Collision resolver inode
	var crIno inode
	if collision {
		crIno = makeInode(InodeModeFile|InodeModeRXOnly, 1, 0, InodeFlagInternal|InodeFlagReadonly, 1)
		crIno.setNumber(inodeNum)
		inodeNum++
		inodes = append(inodes, crIno)
	}

	// Uroot (the actual root)
	urootIno := makeInode(InodeModeDir|InodeModeRXOnly, 1, 65536, InodeFlagReadonly, 3)
	urootIno.setNumber(inodeNum)
	inodeNum++
	inodes = append(inodes, urootIno)
	props.Root.name = "uroot"
	props.Root.ino = urootIno

	if props.Sign {
		superRootIno.setFlags(superRootIno.getFlags() & ^uint32(InodeFlagReadonly))
		fptIno.setFlags(fptIno.getFlags() & ^uint32(InodeFlagReadonly))
		urootIno.setFlags(urootIno.getFlags() & ^uint32(InodeFlagReadonly))
	}

	// Add directory inodes
	for _, dir := range allDirs {
		ino := makeInode(InodeModeDir|InodeModeRXOnly, 1, 65536, InodeFlagReadonly, 2)
		ino.setNumber(inodeNum)
		dir.ino = ino
		inodes = append(inodes, ino)
		inodeNum++
	}

	// Add file inodes
	for _, file := range allFiles {
		blocks := uint32(ceilDiv(int64(len(file.data)), int64(props.BlockSize)))
		if blocks == 0 {
			blocks = 1
		}
		flags := uint32(InodeFlagReadonly)
		if file.compressedSize > 0 {
			flags |= InodeFlagCompressed
		}
		ino := makeInode(InodeModeFile|InodeModeRXOnly, blocks, int64(len(file.data)), flags, 1)
		if props.Sign {
			ino.setFlags(ino.getFlags() & ^uint32(InodeFlagReadonly))
		}
		if file.compressedSize > 0 {
			ino.setSizeCompressed(file.compressedSize)
		}
		ino.setNumber(inodeNum)
		file.ino = ino
		inodes = append(inodes, ino)
		inodeNum++
	}

	// Build flat path table
	fptData := buildFlatPathTable(allNodes)

	// Calculate data block layout
	hdr.DinodeCount = int64(len(inodes))
	inodeSize := dinodeD32Size
	if props.Sign {
		inodeSize = dinodeS32Size
	}
	inodesPerBlock := int(props.BlockSize) / inodeSize
	hdr.DinodeBlockCount = int64(ceilDiv(int64(len(inodes)), int64(inodesPerBlock)))

	// Set up InodeBlockSig in header
	hdr.InodeBlockSig.Blocks = uint32(hdr.DinodeBlockCount)
	hdr.InodeBlockSig.Size = int64(hdr.DinodeBlockCount) * int64(props.BlockSize)
	hdr.InodeBlockSig.SizeCompressed = hdr.InodeBlockSig.Size
	hdr.InodeBlockSig.setTime(props.FileTime)

	hdr.Ndblock = int64(1) // header block

	// Block allocation — same logic for both signed and unsigned PFS.
	// The only difference is inode size (dinodeS32 vs dinodeD32), which
	// is already reflected in inodeSize above.
	{
		// Inode blocks
		hdr.InodeBlockSig.setDirectBlock(0, hdr.Ndblock)
		for i := 1; i < int(hdr.DinodeBlockCount); i++ {
			if i < 12 {
				hdr.InodeBlockSig.setDirectBlock(i, -1)
			}
			hdr.Ndblock++
		}
		hdr.Ndblock++

		// Super root dirents
		superRootIno.setDirectBlock(0, int32(hdr.Ndblock))
		hdr.Ndblock++

		// Flat path table
		fptIno.setDirectBlock(0, int32(hdr.Ndblock))
		fptIno.setSize(int64(len(fptData)))
		fptIno.setSizeCompressed(int64(len(fptData)))
		fptBlocks := uint32(ceilDiv(int64(len(fptData)), int64(props.BlockSize)))
		fptIno.setBlocks(fptBlocks)
		hdr.Ndblock += int64(fptBlocks)

		// Empty block (or collision resolver)
		if crIno == nil {
			hdr.Ndblock++
		} else {
			crIno.setDirectBlock(0, int32(hdr.Ndblock))
			hdr.Ndblock++
		}

		// Data blocks for uroot (root directory) and all child nodes
		// C# adds properties.root to allNodes before this loop
		urootNodes := []*fsNode{props.Root}
		urootNodes = append(urootNodes, allNodes...)

		for _, n := range urootNodes {
			sz := n.size()
			blocks := uint32(ceilDiv(sz, int64(props.BlockSize)))
			if blocks == 0 {
				blocks = 1
			}
			n.ino.setDirectBlock(0, int32(hdr.Ndblock))
			n.ino.setBlocks(blocks)
			if n.isDir {
				n.ino.setSize(ceilDiv(sz, int64(props.BlockSize)) * int64(props.BlockSize))
			} else {
			n.ino.setSize(sz)
			}
			if n.compressedSize > 0 {
				n.ino.setSizeCompressed(n.compressedSize)
			} else {
				n.ino.setSizeCompressed(n.ino.getSize())
			}
			for i := 1; int(i) < int(blocks) && i < 12; i++ {
				n.ino.setDirectBlock(i, -1)
			}
			hdr.Ndblock += int64(blocks)
		}
	}

	// Ensure minimum block count
	if hdr.Ndblock < int64(props.MinBlocks) {
		hdr.Ndblock = int64(props.MinBlocks)
	}

	// Allocate image buffer
	totalSize := hdr.Ndblock * int64(props.BlockSize)
	buf := make([]byte, totalSize)
	w := newBytesWriteSeeker(buf)

	// Write header
	hdr.writeTo(w)

	// Write inodes
	seekTo(w, int64(props.BlockSize))
	for _, ino := range inodes {
		ino.writeTo(w)
		cur, _ := w.Seek(0, io.SeekCurrent)
		mod := cur % int64(props.BlockSize)
		if mod > int64(props.BlockSize)-int64(inodeSize) {
			seekTo(w, cur+int64(props.BlockSize)-mod)
		}
	}

	// Write super root dirents
	superRootBlock := hdr.DinodeBlockCount + 1
	seekTo(w, int64(props.BlockSize)*superRootBlock)
	writeDirent(w, fptIno, "flat_path_table", DirentTypeFile)
	if crIno != nil {
		writeDirent(w, crIno, "collision_resolver", DirentTypeFile)
	}
	writeDirent(w, urootIno, "uroot", DirentTypeDirectory)

	// Write flat path table
	seekTo(w, int64(fptIno.startBlock())*int64(props.BlockSize))
	w.Write(fptData)

	// Write uroot directory dirents
	urootBlock := int64(props.Root.ino.startBlock()) * int64(props.BlockSize)
	seekTo(w, urootBlock)
	writeDirent(w, props.Root.ino, ".", DirentTypeDot)
	writeDirent(w, props.Root.ino, "..", DirentTypeDotDot)
	for _, child := range props.Root.children {
		typ := DirentTypeFile
		if child.isDir {
			typ = DirentTypeDirectory
		}
		writeDirent(w, child.ino, child.name, typ)
	}

	// Write directory dirents for child dirs
	for _, dir := range allDirs {
		blk := int64(dir.ino.startBlock()) * int64(props.BlockSize)
		seekTo(w, blk)
		// Write dot and dotdot
		writeDirentRaw(w, dir.ino, ".", DirentTypeDot)
		writeDirentRaw(w, dir.parent.ino, "..", DirentTypeDotDot)
		// Write children
		for _, child := range dir.children {
			typ := DirentTypeFile
			if child.isDir {
				typ = DirentTypeDirectory
			}
			writeDirentRaw(w, child.ino, child.name, typ)
		}
	}

	// Write file data
	for _, file := range allFiles {
		blk := int64(file.ino.startBlock()) * int64(props.BlockSize)
		seekTo(w, blk)
		w.Write(file.data)
	}

	// Sign if needed
	if props.Sign {
		signKey := PfsGenSignKey(props.EKPFS, hdr.Seed)
		// Sign header block
		sig := HmacSha256(signKey, buf[0:0x5A0])
		copy(buf[0x380:], sig)
		binary.LittleEndian.PutUint32(buf[0x380+32:], 0)

		// Sign inode blocks
		for i := int64(0); i < hdr.DinodeBlockCount; i++ {
			blkStart := int64(props.BlockSize) * (1 + i)
			sig := HmacSha256(signKey, buf[blkStart:blkStart+int64(props.BlockSize)])
			sigOff := 0xB8 + (36 * i)
			copy(buf[sigOff:], sig)
			binary.LittleEndian.PutUint32(buf[sigOff+32:], uint32(1+i))
		}
	}

	// Encrypt if needed
	if props.Encrypt {
		tweakKey, dataKey := PfsGenEncKey(props.EKPFS, hdr.Seed)
		// XTS encryption starts at sector 16 (= BlockSize / XtsSectorSize = 0x10000 / 0x1000)
		// Sectors 0-15 (first PFS block = header) remain plaintext.
		startSector := int(props.BlockSize) / 0x1000
		encrypted := AES128XTSEncrypt(buf, dataKey, tweakKey, 0x1000, startSector)
		buf = encrypted
	}

	return buf, nil
}

// ---------------------------------------------------------------------------
// Flat path table builder
// ---------------------------------------------------------------------------

func buildFlatPathTable(nodes []*fsNode) []byte {
	// Build hash -> value mapping
	type entry struct {
		hash  uint32
		value uint32
	}
	var entries []entry
	for _, n := range nodes {
		h := pfsHashFunction(n.fullPath())
		var val uint32
		if n.isDir {
			val = 0x20000000
		}
		entries = append(entries, entry{h, val})
	}

	// Sort by hash for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].hash < entries[j].hash
	})

	buf := make([]byte, len(entries)*8)
	for i, e := range entries {
		binary.LittleEndian.PutUint32(buf[i*8:], e.hash)
		binary.LittleEndian.PutUint32(buf[i*8+4:], e.value)
	}
	return buf
}

// ---------------------------------------------------------------------------
// Dirent writing
// ---------------------------------------------------------------------------

func writeDirent(w io.Writer, ino inode, name string, typ int32) {
	d := pfsDirent{
		InodeNumber: getInodeNumber(ino),
		Type:        typ,
		Name:        name,
	}
	d.writeTo(w)
}

func writeDirentRaw(w io.Writer, ino inode, name string, typ int32) {
	writeDirent(w, ino, name, typ)
}

func getInodeNumber(ino inode) uint32 {
	return ino.getNumber()
}

// ---------------------------------------------------------------------------
// Helper types
// ---------------------------------------------------------------------------

type bytesWriteSeeker struct {
	data []byte
	pos  int
}

func newBytesWriteSeeker(data []byte) *bytesWriteSeeker {
	return &bytesWriteSeeker{data: data}
}

func (w *bytesWriteSeeker) Write(p []byte) (int, error) {
	n := copy(w.data[w.pos:], p)
	w.pos += n
	return n, nil
}

func (w *bytesWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		w.pos = int(offset)
	case io.SeekCurrent:
		w.pos += int(offset)
	case io.SeekEnd:
		w.pos = len(w.data) + int(offset)
	}
	return int64(w.pos), nil
}

func ceilDiv(a, b int64) int64 {
	return (a + b - 1) / b
}

// ---------------------------------------------------------------------------
// NewDir creates a new directory fsNode.
// ---------------------------------------------------------------------------

func NewDir(name string) *fsNode {
	return &fsNode{name: name, isDir: true}
}

// NewFile creates a new file fsNode with the given data.
func NewFile(name string, data []byte) *fsNode {
	return &fsNode{name: name, data: data}
}

// AddChild adds a child node to a directory.
func (n *fsNode) AddChild(child *fsNode) {
	child.parent = n
	n.children = append(n.children, child)
}

// BuildFSTree creates a filesystem tree from a list of file paths and their data.
// files is a map of relative path -> file content.
func BuildFSTree(files map[string][]byte) *fsNode {
	root := &fsNode{isDir: true, name: ""}

	// Sort paths for deterministic ordering
	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		parts := strings.Split(path, "/")
		current := root

		// Navigate/create intermediate directories
		for i := 0; i < len(parts)-1; i++ {
			dirName := parts[i]
			found := false
			for _, child := range current.children {
				if child.isDir && child.name == dirName {
					current = child
					found = true
					break
				}
			}
			if !found {
				newDir := &fsNode{name: dirName, isDir: true, parent: current}
				current.children = append(current.children, newDir)
				current = newDir
			}
		}

		// Add the file
		file := &fsNode{
			name:   parts[len(parts)-1],
			data:   files[path],
			parent: current,
		}
		current.children = append(current.children, file)
	}

	return root
}

// ---------------------------------------------------------------------------
// BuildPFS creates a complete PFS image from a file map (inner PFS).
// This is a convenience function that handles the boilerplate.
// ---------------------------------------------------------------------------

func BuildPFS(files map[string][]byte, blockSize uint32, minBlocks uint32) ([]byte, error) {
	root := BuildFSTree(files)
	props := PfsProperties{
		Root:      root,
		BlockSize: blockSize,
		MinBlocks: minBlocks,
		Encrypt:   false,
		Sign:      false,
		FileTime:  time.Now().Unix(),
	}
	return BuildPFSImage(props)
}

// ---------------------------------------------------------------------------
// Verify interface satisfaction
// ---------------------------------------------------------------------------

var _ io.WriteSeeker = (*bytesWriteSeeker)(nil)
