package core

// SplitFormat identifies the naming convention for split PKG files.
type SplitFormat int

const (
	SplitPkgpart   SplitFormat = iota // Game_001.pkgpart
	SplitPkgUnderN                    // Game.pkg_0
	SplitPkgDotNNN                    // Game.pkg.001
)

var bufferOptions = []struct {
	Label string
	Bytes int
}{
	{"1 MB", 1 << 20},
	{"4 MB", 4 << 20},
	{"16 MB", 16 << 20},
	{"64 MB", 64 << 20},
	{"128 MB", 128 << 20},
	{"256 MB", 256 << 20},
}

// BufferLabels returns the display labels for available buffer sizes.
func BufferLabels() []string {
	out := make([]string, len(bufferOptions))
	for i, o := range bufferOptions {
		out[i] = o.Label
	}
	return out
}

// BufferBytes returns the byte count matching a buffer label.
func BufferBytes(label string) int {
	for _, o := range bufferOptions {
		if o.Label == label {
			return o.Bytes
		}
	}
	return 64 << 20
}

var chunkOptions = []struct {
	Label string
	Bytes int64
}{
	{"2 GB", 2 << 30},
	{"4 GB", 4 << 30},
	{"8 GB", 8 << 30},
	{"15 GB", 15 << 30},
	{"30 GB", 30 << 30},
}

// ChunkLabels returns the display labels for available chunk sizes.
func ChunkLabels() []string {
	out := make([]string, len(chunkOptions))
	for i, o := range chunkOptions {
		out[i] = o.Label
	}
	return out
}

// ChunkBytes returns the byte count matching a chunk label.
func ChunkBytes(label string) int64 {
	for _, o := range chunkOptions {
		if o.Label == label {
			return o.Bytes
		}
	}
	return 4 << 30
}

var splitFormatOptions = []struct {
	Label  string
	Format SplitFormat
}{
	{"_NNN.pkgpart", SplitPkgpart},
	{".pkg_N", SplitPkgUnderN},
	{".pkg.NNN", SplitPkgDotNNN},
}

// SplitFormatLabels returns the display labels for available split formats.
func SplitFormatLabels() []string {
	out := make([]string, len(splitFormatOptions))
	for i, o := range splitFormatOptions {
		out[i] = o.Label
	}
	return out
}

// SplitFormatByLabel returns the SplitFormat matching a label.
func SplitFormatByLabel(label string) SplitFormat {
	for _, o := range splitFormatOptions {
		if o.Label == label {
			return o.Format
		}
	}
	return SplitPkgpart
}
