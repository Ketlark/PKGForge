package fpkg

// This file implements GP4 XML manifest generation for PS4 PKG building.
// Adapted from /tmp/create-gp4/ with cleaner API.

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GP4 XML structure types.

type gp4Project struct {
	XMLName  xml.Name   `xml:"psproject"`
	XmlnsXsd string     `xml:"xmlns:xsd,attr"`
	XmlnsXsi string     `xml:"xmlns:xsi,attr"`
	Fmt      string     `xml:"fmt,attr"`
	Version  string     `xml:"version,attr"`
	Volume   gp4Volume  `xml:"volume"`
	Files    gp4Files   `xml:"files"`
	RootDir  gp4Rootdir `xml:"rootdir"`
}

type gp4Volume struct {
	Type      string       `xml:"volume_type"`
	ID        string       `xml:"volume_id,omitempty"`
	Timestamp string       `xml:"volume_ts"`
	Package   gp4Pkg       `xml:"package"`
	ChunkInfo gp4ChunkInfo `xml:"chunk_info"`
}

type gp4Pkg struct {
	ContentID   string `xml:"content_id,attr"`
	Passcode    string `xml:"passcode,attr"`
	StorageType string `xml:"storage_type,attr"`
	AppType     string `xml:"app_type,attr"`
	CDate       string `xml:"c_date,attr,omitempty"`
}

type gp4ChunkInfo struct {
	ChunkCount    int          `xml:"chunk_count,attr"`
	ScenarioCount int          `xml:"scenario_count,attr"`
	Chunks        gp4Chunks    `xml:"chunks"`
	Scenarios     gp4Scenarios `xml:"scenarios"`
}

type gp4Chunks struct {
	Chunk []gp4Chunk `xml:"chunk"`
}

type gp4Chunk struct {
	ID      int    `xml:"id,attr"`
	LayerNo int    `xml:"layer_no,attr"`
	Label   string `xml:"label,attr"`
}

type gp4Scenarios struct {
	DefaultID int           `xml:"default_id,attr"`
	Scenario  []gp4Scenario `xml:"scenario"`
}

type gp4Scenario struct {
	ID                int    `xml:"id,attr"`
	Type              string `xml:"type,attr"`
	InitialChunkCount int    `xml:"initial_chunk_count,attr"`
	Label             string `xml:"label,attr"`
	ChunkIDs          string `xml:",chardata"`
}

type gp4Files struct {
	ImgNo int       `xml:"img_no,attr"`
	File  []gp4File `xml:"file"`
}

type gp4File struct {
	TargPath string `xml:"targ_path,attr"`
	OrigPath string `xml:"orig_path,attr"`
}

type gp4Rootdir struct {
	Dir []gp4Dir `xml:"dir"`
}

type gp4Dir struct {
	TargName string   `xml:"targ_name,attr"`
	Dirs     []gp4Dir `xml:"dir"`
}

// GP4Options configures the GP4 manifest generation.
type GP4Options struct {
	ContentID    string   // e.g. "UP9000-SCUS94400_00-0000000000000001"
	Files        []string // relative file paths within the project
	SourceDir    string   // base directory to prepend as orig_path (optional)
	Passcode     string
	VolumeID     string
	OmitVolumeID bool
	Timestamp    string
	PackageDate  string
}

// GenerateGP4 creates a GP4 XML manifest and returns it as a byte slice.
func GenerateGP4(opts GP4Options) ([]byte, error) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	if opts.Timestamp != "" {
		timestamp = opts.Timestamp
	}
	passcode := opts.Passcode
	if passcode == "" {
		passcode = "00000000000000000000000000000000"
	}
	volumeID := opts.VolumeID
	if volumeID == "" && !opts.OmitVolumeID {
		volumeID = "PS4VOLUME"
	}

	project := gp4Project{
		XmlnsXsd: "http://www.w3.org/2001/XMLSchema",
		XmlnsXsi: "http://www.w3.org/2001/XMLSchema-instance",
		Fmt:      "gp4",
		Version:  "1000",
		Volume: gp4Volume{
			Type:      "pkg_ps4_app",
			ID:        volumeID,
			Timestamp: timestamp,
			Package: gp4Pkg{
				ContentID:   opts.ContentID,
				Passcode:    passcode,
				StorageType: "digital50",
				AppType:     "full",
				CDate:       opts.PackageDate,
			},
			ChunkInfo: gp4ChunkInfo{
				ChunkCount:    1,
				ScenarioCount: 1,
				Chunks: gp4Chunks{
					Chunk: []gp4Chunk{
						{ID: 0, LayerNo: 0, Label: "Chunk #0"},
					},
				},
				Scenarios: gp4Scenarios{
					DefaultID: 0,
					Scenario: []gp4Scenario{
						{ID: 0, Type: "sp", InitialChunkCount: 1, Label: "Scenario #0", ChunkIDs: "0"},
					},
				},
			},
		},
		Files: gp4Files{
			ImgNo: 0,
			File:  make([]gp4File, 0, len(opts.Files)),
		},
	}

	// Add files
	for _, f := range opts.Files {
		origPath := f
		if opts.SourceDir != "" {
			origPath = filepath.Join(opts.SourceDir, f)
		}
		project.Files.File = append(project.Files.File, gp4File{
			TargPath: filepath.ToSlash(f),
			OrigPath: filepath.ToSlash(origPath),
		})
	}

	// Build directory tree
	project.RootDir = buildDirTree(opts.Files)

	output, err := xml.MarshalIndent(project, "", "\t")
	if err != nil {
		return nil, fmt.Errorf("fpkg: GP4 marshal: %w", err)
	}

	return append([]byte(xml.Header+"\n"), output...), nil
}

// WriteGP4 generates and writes a GP4 file to disk.
func WriteGP4(path string, opts GP4Options) error {
	data, err := GenerateGP4(opts)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// buildDirTree constructs the nested directory tree from file paths.
func buildDirTree(files []string) gp4Rootdir {
	dirMap := make(map[string]*gp4Dir)
	var rootDirs []gp4Dir

	// Collect unique directory paths, sorted by depth
	var dirs []string
	seen := make(map[string]bool)
	for _, f := range files {
		d := filepath.ToSlash(filepath.Dir(f))
		for d != "." && d != "" {
			if !seen[d] {
				dirs = append(dirs, d)
				seen[d] = true
			}
			d = filepath.ToSlash(filepath.Dir(d))
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i] < dirs[j]
	})

	// Build tree
	for _, d := range dirs {
		parts := strings.Split(d, "/")
		parentKey := strings.Join(parts[:len(parts)-1], "/")
		name := parts[len(parts)-1]

		dir := gp4Dir{TargName: name}
		dirMap[d] = &dir

		if len(parts) == 1 {
			// Top-level directory
			rootDirs = append(rootDirs, dir)
		} else if parent, ok := dirMap[parentKey]; ok {
			parent.Dirs = append(parent.Dirs, dir)
		}
	}

	return gp4Rootdir{Dir: rootDirs}
}

// CollectFiles walks a directory and returns relative file paths.
func CollectFiles(root string) ([]string, error) {
	var files []string
	root = filepath.ToSlash(root)
	if !strings.HasSuffix(root, "/") {
		root += "/"
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel := strings.TrimPrefix(filepath.ToSlash(path), root)
			if rel != "" {
				files = append(files, rel)
			}
		}
		return nil
	})

	return files, err
}
