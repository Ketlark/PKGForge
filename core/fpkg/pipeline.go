package fpkg

import (
	"fmt"
	"os"
)

// ImagePipelineOptions configures inner/outer PFS construction.
type ImagePipelineOptions struct {
	ContentID        string
	Passcode         string
	OnProgress       ProgressReporter
	SkipPFSCCompress bool // when true, store PFSC blocks uncompressed (faster builds, larger PKG)
}

// ImagePipelineResult holds the signed outer PFS and inner logical size.
type ImagePipelineResult struct {
	OuterPFS         []byte
	OuterPFSPath     string
	OuterPFSCleanup  func()
	InnerLogicalSize int64
	EKPFS            []byte
}

// BuildPackagedImages runs inner PFS → PFSC wrap → signed outer PFS.
func BuildPackagedImages(project VirtualFS, runtimeFiles map[string][]byte, opts ImagePipelineOptions) (ImagePipelineResult, error) {
	var result ImagePipelineResult
	var err error

	if opts.OnProgress != nil {
		opts.OnProgress(15, "Building inner filesystem")
	}

	files := project.MergeMem(runtimeFiles)
	innerPFS, innerPFSPath, innerPFSSize, innerCleanup, err := buildInnerPFS(files, project.Disk)
	if err != nil {
		return result, err
	}
	if innerCleanup != nil {
		defer innerCleanup()
	}
	result.InnerLogicalSize = innerPFSSize

	if opts.OnProgress != nil {
		opts.OnProgress(35, "Wrapping filesystem")
	}
	pfscWrapped, pfscPath, pfscCleanup, err := wrapInnerPFS(innerPFS, innerPFSPath, opts.SkipPFSCCompress)
	innerPFS = nil
	if err != nil {
		return result, fmt.Errorf("pfkg: pfsc wrap: %w", err)
	}
	if pfscCleanup != nil {
		defer pfscCleanup()
	}

	result.EKPFS = ComputeKeys(opts.ContentID, opts.Passcode, 1)

	var outerRoot *fsNode
	if pfscPath != "" {
		outerRoot = BuildFSTree(nil, map[string]string{"pfs_image.dat": pfscPath})
	} else {
		outerRoot = BuildFSTree(map[string][]byte{"pfs_image.dat": pfscWrapped}, nil)
	}
	for _, child := range outerRoot.children {
		if child.name == "pfs_image.dat" {
			child.compressedSize = innerPFSSize
		}
	}

	outerProps := PfsProperties{
		Root:      outerRoot,
		BlockSize: 0x10000,
		Encrypt:   true,
		Sign:      true,
		EKPFS:     result.EKPFS,
		Seed:      make([]byte, 16),
		MinBlocks: 0,
	}
	if opts.OnProgress != nil {
		opts.OnProgress(55, "Building encrypted filesystem")
	}
	tmp, err := os.CreateTemp("", "pkg-forge-outer-*.pfs")
	if err != nil {
		return result, fmt.Errorf("fpkg: outer PFS temp: %w", err)
	}
	outerPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		os.Remove(outerPath)
		return result, err
	}
	if _, err := BuildSignedEncryptedPFSToFile(outerProps, outerPath); err != nil {
		os.Remove(outerPath)
		return result, fmt.Errorf("fpkg: outer PFS: %w", err)
	}
	result.OuterPFSPath = outerPath
	result.OuterPFSCleanup = func() { os.Remove(outerPath) }
	return result, nil
}

func buildInnerPFS(files map[string][]byte, fileSources map[string]string) (data []byte, filePath string, logicalSize int64, cleanup func(), err error) {
	if len(fileSources) == 0 {
		data, err = BuildPFS(files, nil, 0x10000, 0x55)
		if err != nil {
			return nil, "", 0, nil, err
		}
		return data, "", int64(len(data)), nil, nil
	}

	tmp, err := os.CreateTemp("", "pkg-forge-inner-*.pfs")
	if err != nil {
		return nil, "", 0, nil, err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return nil, "", 0, nil, err
	}

	size, err := BuildPFSToFileFromMaps(files, fileSources, 0x10000, 0x55, tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return nil, "", 0, nil, err
	}
	return nil, tmpPath, size, func() { os.Remove(tmpPath) }, nil
}

func wrapInnerPFS(innerData []byte, innerPath string, skipCompress bool) (data []byte, filePath string, cleanup func(), err error) {
	if innerPath != "" {
		tmp, err := os.CreateTemp("", "pkg-forge-pfsc-*.dat")
		if err != nil {
			return nil, "", nil, err
		}
		tmpPath := tmp.Name()
		if err := tmp.Close(); err != nil {
			os.Remove(tmpPath)
			return nil, "", nil, err
		}
		if _, err := wrapPFSCToFile(innerPath, tmpPath, skipCompress); err != nil {
			os.Remove(tmpPath)
			return nil, "", nil, err
		}
		return nil, tmpPath, func() { os.Remove(tmpPath) }, nil
	}
	return wrapPFSC(innerData), "", nil, nil
}
