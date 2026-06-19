package fpkg

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWrapPFSCToFileMatchesInMemory(t *testing.T) {
	src := bytes.Repeat([]byte{0x5A}, pfscBlockSize*3+123)
	inMemory := wrapPFSC(src)

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "inner.pfs")
	dstPath := filepath.Join(dir, "wrapped.pfsc")
	if err := os.WriteFile(srcPath, src, 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	written, err := wrapPFSCToFile(srcPath, dstPath, false)
	if err != nil {
		t.Fatalf("wrapPFSCToFile: %v", err)
	}
	onDisk, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read wrapped: %v", err)
	}
	if int64(len(onDisk)) != written {
		t.Fatalf("reported size %d != file size %d", written, len(onDisk))
	}
	if !bytes.Equal(inMemory, onDisk) {
		t.Fatalf("file-backed PFSC output differs from in-memory wrap")
	}
}

func TestParallelPFSCEncodeMatchesSequential(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Skip("requires at least 2 CPUs")
	}

	// Mix compressible zeros and incompressible random-ish bytes across many blocks.
	var src bytes.Buffer
	for i := 0; i < pfscParallelMinBlocks+4; i++ {
		if i%5 == 0 {
			src.Write(bytes.Repeat([]byte{0}, pfscBlockSize))
		} else {
			src.Write(bytes.Repeat([]byte{byte(i)}, pfscBlockSize))
		}
	}
	data := src.Bytes()

	plansSeq := make([]pfscBlockPlan, (int64(len(data))+pfscBlockSize-1)/pfscBlockSize)
	var bodySeq bytes.Buffer
	emitSeq := func(idx int64, block []byte, onDiskSize int64) error {
		plansSeq[idx].onDiskSize = onDiskSize
		_, err := bodySeq.Write(block)
		releasePFSCBlockData(block)
		return err
	}
	if err := sequentialPFSCEncode(bytes.NewReader(data), int64(len(data)), int64(len(plansSeq)), false, emitSeq); err != nil {
		t.Fatalf("sequential: %v", err)
	}

	plansPar := make([]pfscBlockPlan, len(plansSeq))
	var bodyPar bytes.Buffer
	emitPar := func(idx int64, block []byte, onDiskSize int64) error {
		plansPar[idx].onDiskSize = onDiskSize
		_, err := bodyPar.Write(block)
		releasePFSCBlockData(block)
		return err
	}
	if err := parallelPFSCEncode(bytes.NewReader(data), int64(len(data)), int64(len(plansPar)), false, emitPar); err != nil {
		t.Fatalf("parallel: %v", err)
	}

	if !bytes.Equal(bodySeq.Bytes(), bodyPar.Bytes()) {
		t.Fatalf("parallel body differs from sequential")
	}
	for i := range plansSeq {
		if plansSeq[i] != plansPar[i] {
			t.Fatalf("plan[%d]: sequential %+v != parallel %+v", i, plansSeq[i], plansPar[i])
		}
	}
}
