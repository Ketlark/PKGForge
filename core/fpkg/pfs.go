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
	"fmt"
	"io"
	"os"
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
	sourcePath     string    // optional on-disk source (streamed, not loaded into RAM)
	compressedSize int64     // for PFSC-wrapped files: uncompressed size
	ino            inode
}

func (n *fsNode) fullPath() string {
	return n.fullPathWithSuffix("")
}

func (n *fsNode) fullPathWithSuffix(suffix string) string {
	if n.parent == nil {
		return suffix
	}
	// LibOrbis hashes absolute PFS paths in flat_path_table, including the
	// leading slash for files directly under uroot.
	return n.parent.fullPathWithSuffix("/" + n.name + suffix)
}

func (n *fsNode) size() int64 {
	if n.isDir {
		var total int64
		for _, c := range n.children {
			total += int64(direntSize(c.name))
		}
		return total
	}
	if n.sourcePath != "" {
		info, err := os.Stat(n.sourcePath)
		if err != nil {
			return 0
		}
		return info.Size()
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

func dirNlink(dir *fsNode) uint16 {
	nlink := uint16(2)
	for _, child := range dir.children {
		if child.isDir {
			nlink++
		}
	}
	return nlink
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

func (d *dinodeD32) getBlocks() uint32         { return d.Blocks }
func (d *dinodeD32) setBlocks(b uint32)        { d.Blocks = b }
func (d *dinodeD32) getSize() int64            { return d.Size }
func (d *dinodeD32) setSize(s int64)           { d.Size = s }
func (d *dinodeD32) getSizeCompressed() int64  { return d.SizeCompressed }
func (d *dinodeD32) setSizeCompressed(s int64) { d.SizeCompressed = s }
func (d *dinodeD32) getFlags() uint32          { return d.Flags }
func (d *dinodeD32) setFlags(f uint32)         { d.Flags = f }
func (d *dinodeD32) getNumber() uint32         { return d.number }
func (d *dinodeD32) setNumber(n uint32)        { d.number = n }
func (d *dinodeD32) getNlink() uint16          { return d.Nlink }
func (d *dinodeD32) setNlink(n uint16)         { d.Nlink = n }

func (d *dinodeS32) getBlocks() uint32         { return d.Blocks }
func (d *dinodeS32) setBlocks(b uint32)        { d.Blocks = b }
func (d *dinodeS32) getSize() int64            { return d.Size }
func (d *dinodeS32) setSize(s int64)           { d.Size = s }
func (d *dinodeS32) getSizeCompressed() int64  { return d.SizeCompressed }
func (d *dinodeS32) setSizeCompressed(s int64) { d.SizeCompressed = s }
func (d *dinodeS32) getFlags() uint32          { return d.Flags }
func (d *dinodeS32) setFlags(f uint32)         { d.Flags = f }
func (d *dinodeS32) getNumber() uint32         { return d.number }
func (d *dinodeS32) setNumber(n uint32)        { d.number = n }
func (d *dinodeS32) getNlink() uint16          { return d.Nlink }
func (d *dinodeS32) setNlink(n uint16)         { d.Nlink = n }

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
	block  int64
	sigOff int64
	size   int64
}

// BuildPFSImage constructs a complete PFS image and returns it as a byte slice.
func BuildPFSImage(props PfsProperties) ([]byte, error) {
	return buildPFSImage(props, nil)
}

// BuildPFSToFile writes an unsigned, unencrypted PFS image directly to disk.
// This avoids holding the full image in memory — used for large PS2 disc payloads.
func BuildPFSToFile(props PfsProperties, path string) (int64, error) {
	if props.Encrypt || props.Sign {
		return 0, fmt.Errorf("pfs: BuildPFSToFile only supports unsigned inner images")
	}
	return buildPFSToFile(props, path)
}

// BuildSignedEncryptedPFSToFile writes a signed, XTS-encrypted outer PFS image
// directly to disk without holding the full image in memory.
func BuildSignedEncryptedPFSToFile(props PfsProperties, path string) (int64, error) {
	if !props.Encrypt || !props.Sign {
		return 0, fmt.Errorf("pfs: BuildSignedEncryptedPFSToFile requires signed encrypted images")
	}
	return buildPFSToFile(props, path)
}

func buildPFSToFile(props PfsProperties, path string) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err := buildPFSImage(props, f); err != nil {
		return 0, err
	}
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func buildPFSImage(props PfsProperties, destFile *os.File) ([]byte, error) {
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
				Mode:           mode,
				Blocks:         blocks,
				Size:           size,
				SizeCompressed: size,
				Nlink:          nlink,
				Flags:          flags | InodeFlagUnk2 | InodeFlagUnk3,
			}
			d.setTime(props.FileTime)
			ino = d
		} else {
			d := &dinodeD32{
				Mode:           mode,
				Blocks:         blocks,
				Size:           size,
				SizeCompressed: size,
				Nlink:          nlink,
				Flags:          flags,
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
	urootIno := makeInode(InodeModeDir|InodeModeRXOnly, 1, 65536, InodeFlagReadonly, dirNlink(props.Root))
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
		ino := makeInode(InodeModeDir|InodeModeRXOnly, 1, 65536, InodeFlagReadonly, dirNlink(dir))
		ino.setNumber(inodeNum)
		dir.ino = ino
		inodes = append(inodes, ino)
		inodeNum++
	}

	// Add file inodes
	for _, file := range allFiles {
		blocks := uint32(ceilDiv(file.size(), int64(props.BlockSize)))
		if blocks == 0 {
			blocks = 1
		}
		flags := uint32(InodeFlagReadonly)
		if file.compressedSize > 0 {
			flags |= InodeFlagCompressed
		}
		ino := makeInode(InodeModeFile|InodeModeRXOnly, blocks, file.size(), flags, 1)
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

	var dataSigs, finalSigs []blockSigInfo
	emptyBlock := int64(-1)
	layoutNodes := append([]*fsNode{props.Root}, allNodes...)

	hdr.Ndblock = 1 // header block
	if props.Sign {
		hdr.InodeBlockSig.Flags = 0
		finalSigs = append(finalSigs, blockSigInfo{block: 0, sigOff: 0x380, size: 0x5A0})

		for i := int64(0); i < hdr.DinodeBlockCount; i++ {
			hdr.InodeBlockSig.setDirectBlock(int(i), 1+i)
			finalSigs = append(finalSigs, blockSigInfo{block: 1 + i, sigOff: 0xB8 + 36*i, size: int64(props.BlockSize)})
		}
		hdr.Ndblock += hdr.DinodeBlockCount

		superRootIno.setDirectBlock(0, int32(hdr.DinodeBlockCount+1))
		finalSigs = append(finalSigs, blockSigInfo{block: int64(superRootIno.startBlock()), sigOff: inodeSigOffset(props.BlockSize, superRootIno, 0), size: int64(props.BlockSize)})
		hdr.Ndblock += int64(superRootIno.getBlocks())

		fptIno.setDirectBlock(0, int32(int64(superRootIno.startBlock())+1))
		fptIno.setSize(int64(len(fptData)))
		fptIno.setSizeCompressed(int64(len(fptData)))
		fptBlocks := uint32(ceilDiv(int64(len(fptData)), int64(props.BlockSize)))
		if fptBlocks == 0 {
			fptBlocks = 1
		}
		fptIno.setBlocks(fptBlocks)
		finalSigs = append(finalSigs, blockSigInfo{block: int64(fptIno.startBlock()), sigOff: inodeSigOffset(props.BlockSize, fptIno, 0), size: int64(props.BlockSize)})
		for i := uint32(1); i < fptBlocks && i < 12; i++ {
			fptIno.setDirectBlock(int(i), int32(hdr.Ndblock))
			finalSigs = append(finalSigs, blockSigInfo{block: hdr.Ndblock, sigOff: inodeSigOffset(props.BlockSize, fptIno, int(i)), size: int64(props.BlockSize)})
			hdr.Ndblock++
		}

		// LibOrbis advances past the flat path table, then leaves one zero
		// block in the outer PFS that is intentionally not XTS-encrypted.
		hdr.Ndblock++
		emptyBlock = hdr.Ndblock
		hdr.Ndblock++

		ibStartBlock := hdr.Ndblock
		for _, n := range layoutNodes {
			hdr.Ndblock += calculateIndirectBlocks(n.size(), int64(props.BlockSize))
		}

		sigsPerBlock := int64(props.BlockSize) / 36
		for _, n := range layoutNodes {
			sz := n.size()
			blocks := ceilDiv(sz, int64(props.BlockSize))
			if blocks == 0 {
				blocks = 1
			}
			n.ino.setDirectBlock(0, int32(hdr.Ndblock))
			n.ino.setBlocks(uint32(blocks))
			if n.isDir {
				n.ino.setSize(ceilDiv(sz, int64(props.BlockSize)) * int64(props.BlockSize))
			} else {
				n.ino.setSize(sz)
			}
			if n.compressedSize > 0 {
				n.ino.setSizeCompressed(n.compressedSize)
			} else if n.ino.getSizeCompressed() == 0 {
				n.ino.setSizeCompressed(n.ino.getSize())
			}

			for i := int64(0); blocks-i > 0 && i < 12; i++ {
				dataSigs = append(dataSigs, blockSigInfo{block: hdr.Ndblock, sigOff: inodeSigOffset(props.BlockSize, n.ino, int(i)), size: int64(props.BlockSize)})
				hdr.Ndblock++
			}
			if blocks > 12 {
				finalSigs = append(finalSigs, blockSigInfo{block: ibStartBlock, sigOff: inodeSigOffset(props.BlockSize, n.ino, 12), size: int64(props.BlockSize)})
				for i, pointerOffset := int64(12), int64(0); blocks-i > 0 && i < 12+sigsPerBlock; i, pointerOffset = i+1, pointerOffset+36 {
					dataSigs = append(dataSigs, blockSigInfo{block: hdr.Ndblock, sigOff: ibStartBlock*int64(props.BlockSize) + pointerOffset, size: int64(props.BlockSize)})
					hdr.Ndblock++
				}
				ibStartBlock++
			}
			if blocks > 12+sigsPerBlock {
				blockSigsDone := int64(12 + sigsPerBlock)
				finalSigs = append(finalSigs, blockSigInfo{block: ibStartBlock, sigOff: inodeSigOffset(props.BlockSize, n.ino, 13), size: int64(props.BlockSize)})
				ib1Block := ibStartBlock
				for i := int64(0); i < sigsPerBlock && blockSigsDone < blocks; i++ {
					ibStartBlock++
					finalSigs = append(finalSigs, blockSigInfo{block: ibStartBlock, sigOff: ib1Block*int64(props.BlockSize) + i*36, size: int64(props.BlockSize)})
					for j := int64(0); j < sigsPerBlock && blockSigsDone < blocks; j++ {
						dataSigs = append(dataSigs, blockSigInfo{block: hdr.Ndblock, sigOff: ibStartBlock*int64(props.BlockSize) + j*36, size: int64(props.BlockSize)})
						hdr.Ndblock++
						blockSigsDone++
					}
				}
			}
		}
	} else {
		hdr.InodeBlockSig.setDirectBlock(0, hdr.Ndblock)
		for i := int64(1); i < hdr.DinodeBlockCount; i++ {
			if i < 12 {
				hdr.InodeBlockSig.setDirectBlock(int(i), -1)
			}
			hdr.Ndblock++
		}
		hdr.Ndblock++

		superRootIno.setDirectBlock(0, int32(hdr.Ndblock))
		hdr.Ndblock += int64(superRootIno.getBlocks())

		fptIno.setDirectBlock(0, int32(hdr.Ndblock))
		fptIno.setSize(int64(len(fptData)))
		fptIno.setSizeCompressed(int64(len(fptData)))
		fptBlocks := uint32(ceilDiv(int64(len(fptData)), int64(props.BlockSize)))
		if fptBlocks == 0 {
			fptBlocks = 1
		}
		fptIno.setBlocks(fptBlocks)
		hdr.Ndblock += int64(fptBlocks)

		if crIno == nil {
			hdr.Ndblock++
		} else {
			crIno.setDirectBlock(0, int32(hdr.Ndblock))
			hdr.Ndblock++
		}

		for _, n := range layoutNodes {
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
			for i := uint32(1); i < blocks && i < 12; i++ {
				n.ino.setDirectBlock(int(i), -1)
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
	var buf []byte
	var w io.WriteSeeker
	if destFile != nil {
		if err := destFile.Truncate(totalSize); err != nil {
			return nil, fmt.Errorf("pfs: truncate output: %w", err)
		}
		w = destFile
	} else {
		buf = make([]byte, totalSize)
		w = newBytesWriteSeeker(buf)
	}

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

	// Write file data before directory entries so directory blocks are the
	// final authority for traversal tools that read the image directly.
	for _, file := range allFiles {
		blk := int64(file.ino.startBlock()) * int64(props.BlockSize)
		seekTo(w, blk)
		if err := writeFileNodeContent(w, file); err != nil {
			return nil, fmt.Errorf("pfs: write %s: %w", file.fullPath(), err)
		}
	}

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

	// Sign if needed
	if props.Sign {
		// The PS4 kernel uses raw EKPFS for signing key derivation regardless
		// of pfs_flags. Confirmed by klog analysis.
		signKey := PfsGenSignKey(props.EKPFS, hdr.Seed)
		if err := signPFSImage(destFile, buf, props.BlockSize, signKey, dataSigs, finalSigs); err != nil {
			return nil, err
		}
	}

	// Encrypt if needed
	if props.Encrypt {
		// The PS4 kernel uses raw EKPFS for XTS key derivation regardless of
		// pfs_flags bit 61. Confirmed by klog analysis: a PKG built with raw
		// EKPFS + pfs_flags=0xA0 mounts and executes eboot.bin successfully.
		tweakKey, dataKey := PfsGenEncKey(props.EKPFS, hdr.Seed)
		// XTS encryption starts at sector 16 (= BlockSize / XtsSectorSize = 0x10000 / 0x1000)
		// Sectors 0-15 (first PFS block = header) remain plaintext.
		startSector := int(props.BlockSize) / 0x1000
		sectorsPerBlock := int(props.BlockSize) / 0x1000
		var skipBlocks map[int64]bool
		if emptyBlock >= 0 {
			skipBlocks = map[int64]bool{emptyBlock: true}
		}
		if destFile != nil {
			if err := AES128XTSEncryptSkipBlocksInPlace(destFile, totalSize, dataKey, tweakKey, 0x1000, startSector, sectorsPerBlock, skipBlocks); err != nil {
				return nil, err
			}
		} else {
			encrypted := AES128XTSEncryptSkipBlocks(buf, dataKey, tweakKey, 0x1000, startSector, sectorsPerBlock, skipBlocks)
			buf = encrypted
		}
	}

	return buf, nil
}

func signPFSImage(destFile *os.File, buf []byte, blockSize uint32, signKey []byte, dataSigs, finalSigs []blockSigInfo) error {
	if destFile == nil && buf == nil {
		return fmt.Errorf("pfs: signed image requires memory buffer or output file")
	}
	signBlock := func(sigInfo blockSigInfo) error {
		start := sigInfo.block * int64(blockSize)
		var blockData []byte
		if buf != nil {
			end := start + sigInfo.size
			blockData = buf[start:end]
		} else {
			blockData = make([]byte, sigInfo.size)
			if _, err := destFile.ReadAt(blockData, start); err != nil {
				return err
			}
		}
		sig := HmacSha256(signKey, blockData)
		sigBuf := make([]byte, 36)
		copy(sigBuf, sig)
		binary.LittleEndian.PutUint32(sigBuf[32:], uint32(sigInfo.block))
		if buf != nil {
			copy(buf[sigInfo.sigOff:], sigBuf)
			return nil
		}
		_, err := destFile.WriteAt(sigBuf, sigInfo.sigOff)
		return err
	}

	for _, sigInfo := range dataSigs {
		if err := signBlock(sigInfo); err != nil {
			return err
		}
	}
	for i := len(finalSigs) - 1; i >= 0; i-- {
		if err := signBlock(finalSigs[i]); err != nil {
			return err
		}
	}
	return nil
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
		val := n.ino.getNumber()
		if n.isDir {
			val |= 0x20000000
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

func (w *bytesWriteSeeker) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 || int(off) > len(w.data) {
		return 0, fmt.Errorf("pfs: writeat out of range")
	}
	end := int(off) + len(p)
	if end > len(w.data) {
		return 0, fmt.Errorf("pfs: writeat out of range")
	}
	return copy(w.data[off:end], p), nil
}

func ceilDiv(a, b int64) int64 {
	return (a + b - 1) / b
}

func calculateIndirectBlocks(size, blockSize int64) int64 {
	sigsPerBlock := blockSize / 36
	blocks := ceilDiv(size, blockSize)
	var indirectBlocks int64
	if blocks > 12 {
		blocks -= 12
		indirectBlocks++
	}
	if blocks > sigsPerBlock {
		blocks -= sigsPerBlock
		indirectBlocks += 1 + ceilDiv(blocks, sigsPerBlock)
	}
	return indirectBlocks
}

func inodeSigOffset(blockSize uint32, ino inode, db int) int64 {
	return int64(blockSize) + int64(dinodeS32Size)*int64(ino.getNumber()) + 0x64 + int64(36*db)
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

// BuildFSTree creates a filesystem tree from in-memory files and optional on-disk sources.
// fileSources entries are streamed into the PFS instead of being loaded into RAM.
func BuildFSTree(files map[string][]byte, fileSources map[string]string) *fsNode {
	root := &fsNode{isDir: true, name: ""}

	var paths []string
	seen := make(map[string]bool, len(files)+len(fileSources))
	for p := range files {
		paths = append(paths, p)
		seen[p] = true
	}
	for p := range fileSources {
		if !seen[p] {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)

	for _, path := range paths {
		parts := strings.Split(path, "/")
		current := root

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

		file := &fsNode{
			name:   parts[len(parts)-1],
			parent: current,
		}
		if src, ok := fileSources[path]; ok {
			file.sourcePath = src
		} else {
			file.data = files[path]
		}
		current.children = append(current.children, file)
	}

	return root
}

func writeFileNodeContent(w io.Writer, file *fsNode) error {
	if file.sourcePath != "" {
		f, err := os.Open(file.sourcePath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	}
	_, err := w.Write(file.data)
	return err
}

// ---------------------------------------------------------------------------
// BuildPFS creates a complete PFS image from a file map (inner PFS).
// This is a convenience function that handles the boilerplate.
// ---------------------------------------------------------------------------

func BuildPFS(files map[string][]byte, fileSources map[string]string, blockSize uint32, minBlocks uint32) ([]byte, error) {
	root := BuildFSTree(files, fileSources)
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

func BuildPFSToFileFromMaps(files map[string][]byte, fileSources map[string]string, blockSize uint32, minBlocks uint32, path string) (int64, error) {
	root := BuildFSTree(files, fileSources)
	props := PfsProperties{
		Root:      root,
		BlockSize: blockSize,
		MinBlocks: minBlocks,
		Encrypt:   false,
		Sign:      false,
		FileTime:  time.Now().Unix(),
	}
	return BuildPFSToFile(props, path)
}

// ---------------------------------------------------------------------------
// Verify interface satisfaction
// ---------------------------------------------------------------------------

var _ io.WriteSeeker = (*bytesWriteSeeker)(nil)
