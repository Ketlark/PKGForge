package core

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// PKGInfo holds metadata extracted from a PS4/PS5 PKG header.
type PKGInfo struct {
	ContentID   string `json:"contentId"`
	TitleID     string `json:"titleId"`
	Region      string `json:"region"`
	ContentType string `json:"contentType"`
	DRMType     string `json:"drmType"`
	FileSize    int64  `json:"fileSize"`
	PKGSize     int64  `json:"pkgSize"`
	Valid       bool   `json:"valid"`
	Error       string `json:"error,omitempty"`
}

var regionMap = map[string]string{
	"EP": "Europe",
	"UP": "USA",
	"JP": "Japan",
	"HP": "Asia",
	"KP": "Korea",
	"IP": "India",
}

var contentTypeMap = map[uint32]string{
	0x01: "PS3 Game Data",
	0x04: "PS Vita Game Data",
	0x06: "PS Vita DLC",
	0x15: "PS4 (0x15)",
	0x1A: "PS4 Game",
	0x1B: "PS4 Game Patch",
	0x1C: "PS4 DLC",
	0x1D: "PS4 (0x1D)",
	0x1F: "PS4 (0x1F)",
}

var drmTypeMap = map[uint32]string{
	0x00: "None",
	0x01: "PS4",
	0x0F: "Free",
}

// InspectPKG reads the PKG header and extracts metadata.
func InspectPKG(path string) PKGInfo {
	info := PKGInfo{}

	stat, err := os.Stat(path)
	if err != nil {
		info.Error = fmt.Sprintf("cannot access file: %v", err)
		return info
	}
	info.FileSize = stat.Size()

	f, err := os.Open(path)
	if err != nil {
		info.Error = fmt.Sprintf("cannot open file: %v", err)
		return info
	}
	defer f.Close()

	header := make([]byte, 0x100)
	if _, err := io.ReadFull(f, header[:0x80]); err != nil {
		info.Error = "file too small to contain a PKG header"
		return info
	}
	// Read remaining bytes (optional, for fields at offset 0x80+)
	io.ReadFull(f, header[0x80:])

	if header[0] != 0x7F || header[1] != 0x43 || header[2] != 0x4E || header[3] != 0x54 {
		info.Error = fmt.Sprintf("invalid magic: 0x%X", header[:4])
		return info
	}
	info.Valid = true

	contentIDBytes := header[0x40:0x64]
	info.ContentID = strings.TrimRight(string(contentIDBytes), "\x00")

	if len(info.ContentID) >= 16 {
		info.TitleID = info.ContentID[7:16]
	}

	if len(info.ContentID) >= 2 {
		code := info.ContentID[:2]
		if region, ok := regionMap[code]; ok {
			info.Region = region
		} else {
			info.Region = code
		}
	}

	contentTypeRaw := binary.BigEndian.Uint32(header[0x74:0x78])
	if ct, ok := contentTypeMap[contentTypeRaw]; ok {
		info.ContentType = ct
	} else {
		info.ContentType = fmt.Sprintf("Unknown (0x%02X)", contentTypeRaw)
	}

	drmTypeRaw := binary.BigEndian.Uint32(header[0x70:0x74])
	if dt, ok := drmTypeMap[drmTypeRaw]; ok {
		info.DRMType = dt
	} else {
		info.DRMType = fmt.Sprintf("Unknown (0x%02X)", drmTypeRaw)
	}

	bodyOffset := binary.BigEndian.Uint64(header[0x20:0x28])
	bodySize := binary.BigEndian.Uint64(header[0x28:0x30])
	if bodyOffset > 0 && bodySize > 0 && bodyOffset+bodySize > bodyOffset {
		info.PKGSize = int64(bodyOffset + bodySize)
	}

	return info
}
