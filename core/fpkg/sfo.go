package fpkg

// This file implements the param.sfo binary format writer.
// Ported from LibOrbisPkg/SFO/ParamSfo.cs.
//
// The SFO (System File Object) format is used by PlayStation systems to store
// metadata about content (games, DLC, saves). It contains a header, key table,
// entry table, and data table, all in a flat binary layout.

import (
	"encoding/binary"
	"sort"
)

// SFO entry types.
const (
	SfoTypeUtf8Special uint16 = 0x004
	SfoTypeUtf8        uint16 = 0x204
	SfoTypeInteger     uint16 = 0x404
)

// SfoValue represents a single entry in the param.sfo file.
type SfoValue struct {
	Name      string
	Type      uint16
	MaxLength int

	// For Utf8/Utf8Special entries
	Text string

	// For Integer entries
	IntValue int32
}

// Length returns the actual data length of the value.
func (v *SfoValue) Length() int {
	switch v.Type {
	case SfoTypeInteger:
		return 4
	case SfoTypeUtf8:
		return len(v.Text) + 1 // null-terminated
	case SfoTypeUtf8Special:
		return len(v.Text)
	default:
		return 0
	}
}

// ToBytes serializes the value to bytes.
func (v *SfoValue) ToBytes() []byte {
	switch v.Type {
	case SfoTypeInteger:
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(v.IntValue))
		return buf
	case SfoTypeUtf8:
		return append([]byte(v.Text), 0)
	case SfoTypeUtf8Special:
		return []byte(v.Text)
	default:
		return nil
	}
}

// ParamSfo represents a param.sfo file.
type ParamSfo struct {
	Values []*SfoValue
}

// NewParamSfo creates an empty ParamSfo.
func NewParamSfo() *ParamSfo {
	return &ParamSfo{}
}

// Set sets or updates a UTF-8 string entry.
func (s *ParamSfo) Set(key string, value string, maxLen int) {
	s.set(key, SfoTypeUtf8, value, maxLen)
}

// SetInt sets or updates an integer entry.
func (s *ParamSfo) SetInt(key string, value int32) {
	for _, v := range s.Values {
		if v.Name == key {
			v.Type = SfoTypeInteger
			v.IntValue = value
			v.MaxLength = 4
			return
		}
	}
	s.Values = append(s.Values, &SfoValue{
		Name:      key,
		Type:      SfoTypeInteger,
		IntValue:  value,
		MaxLength: 4,
	})
	sort.Slice(s.Values, func(i, j int) bool {
		return s.Values[i].Name < s.Values[j].Name
	})
}

func (s *ParamSfo) set(key string, typ uint16, value string, maxLen int) {
	for _, v := range s.Values {
		if v.Name == key {
			v.Type = typ
			v.Text = value
			v.MaxLength = maxLen
			return
		}
	}
	s.Values = append(s.Values, &SfoValue{
		Name:      key,
		Type:      typ,
		Text:      value,
		MaxLength: maxLen,
	})
	sort.Slice(s.Values, func(i, j int) bool {
		return s.Values[i].Name < s.Values[j].Name
	})
}

// Serialize writes the complete param.sfo to a byte slice.
func (s *ParamSfo) Serialize() []byte {
	// Sort values alphabetically by name
	sort.Slice(s.Values, func(i, j int) bool {
		return s.Values[i].Name < s.Values[j].Name
	})

	// Calculate layout
	headerSize := 0x14                              // 20 bytes header
	entryTableSize := len(s.Values) * 0x10          // 16 bytes per entry
	keyTableOffset := headerSize + entryTableSize

	// Key table: each key is name + null byte
	keyTableSize := 0
	for _, v := range s.Values {
		keyTableSize += len(v.Name) + 1
	}

	// Data table starts after key table, aligned to 4 bytes
	dataTableOffset := keyTableOffset + keyTableSize
	if dataTableOffset%4 != 0 {
		dataTableOffset += 4 - (dataTableOffset % 4)
	}

	// Total file size
	dataTableSize := 0
	for _, v := range s.Values {
		dataTableSize += v.MaxLength
	}
	totalSize := dataTableOffset + dataTableSize

	// Allocate buffer
	buf := make([]byte, totalSize)

	// Write header
	binary.BigEndian.PutUint32(buf[0x00:0x04], 0x00505346) // "\x00PSF" magic
	binary.LittleEndian.PutUint32(buf[0x04:0x08], 0x101)   // version
	binary.LittleEndian.PutUint32(buf[0x08:0x0C], uint32(keyTableOffset))
	binary.LittleEndian.PutUint32(buf[0x0C:0x10], uint32(dataTableOffset))
	binary.LittleEndian.PutUint32(buf[0x10:0x14], uint32(len(s.Values)))

	// Write entries, keys, and data
	keyOffset := 0
	dataOffset := 0
	for i, v := range s.Values {
		entryOff := 0x14 + i*0x10

		// Entry: keyOffset(2) + type(2) + length(4) + maxLength(4) + dataOffset(4)
		binary.LittleEndian.PutUint16(buf[entryOff:entryOff+2], uint16(keyOffset))
		binary.LittleEndian.PutUint16(buf[entryOff+2:entryOff+4], v.Type)
		binary.LittleEndian.PutUint32(buf[entryOff+4:entryOff+8], uint32(v.Length()))
		binary.LittleEndian.PutUint32(buf[entryOff+8:entryOff+12], uint32(v.MaxLength))
		binary.LittleEndian.PutUint32(buf[entryOff+12:entryOff+16], uint32(dataOffset))

		// Write key name in key table
		nameOff := keyTableOffset + keyOffset
		copy(buf[nameOff:], v.Name)
		buf[nameOff+len(v.Name)] = 0 // null terminator

		// Write data in data table
		dataOff := dataTableOffset + dataOffset
		valBytes := v.ToBytes()
		copy(buf[dataOff:], valBytes)

		keyOffset += len(v.Name) + 1
		dataOffset += v.MaxLength
	}

	return buf
}

// NewPS1ParamSfo creates a param.sfo for PS1 fPKG (category "gd").
func NewPS1ParamSfo(title, titleID, contentID string) *ParamSfo {
	sfo := NewParamSfo()
	sfo.SetInt("APP_TYPE", 4)
	sfo.Set("APP_VER", "01.00", 8)
	sfo.SetInt("ATTRIBUTE", 0)
	sfo.Set("CATEGORY", "gd", 4)
	sfo.Set("CONTENT_ID", contentID, 48)
	sfo.SetInt("DOWNLOAD_DATA_SIZE", 0)
	sfo.Set("FORMAT", "obs", 4)
	sfo.SetInt("PARENTAL_LEVEL", 1)
	sfo.Set("TITLE", title, 128)
	sfo.Set("TITLE_ID", titleID, 12)
	sfo.SetInt("SYSTEM_VER", 0)
	sfo.Set("VERSION", "01.00", 8)
	return sfo
}

// NewPS2ParamSfo creates a param.sfo for PS2 fPKG (category "gd").
func NewPS2ParamSfo(title, titleID, contentID string) *ParamSfo {
	sfo := NewParamSfo()
	sfo.SetInt("APP_TYPE", 4)
	sfo.Set("APP_VER", "01.00", 8)
	sfo.SetInt("ATTRIBUTE", 0)
	sfo.Set("CATEGORY", "gd", 4)
	sfo.Set("CONTENT_ID", contentID, 48)
	sfo.SetInt("DOWNLOAD_DATA_SIZE", 0)
	sfo.Set("FORMAT", "obs", 4)
	sfo.SetInt("PARENTAL_LEVEL", 1)
	sfo.Set("TITLE", title, 128)
	sfo.Set("TITLE_ID", titleID, 12)
	sfo.SetInt("SYSTEM_VER", 0)
	sfo.Set("VERSION", "01.00", 8)
	return sfo
}
