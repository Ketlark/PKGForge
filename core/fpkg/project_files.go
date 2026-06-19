package fpkg

// VirtualFS is the file tree packed into the inner PFS image.
//
// Small files live in memory (Mem). Large payloads — typically PS2 disc images —
// are referenced on disk (Disk) and streamed during PFS construction.
type VirtualFS struct {
	Mem  map[string][]byte
	Disk map[string]string
}

// NewVirtualFS returns an empty virtual filesystem.
func NewVirtualFS() VirtualFS {
	return VirtualFS{
		Mem:  make(map[string][]byte),
		Disk: make(map[string]string),
	}
}

// VirtualFSFromMaps adapts legacy separate maps into a VirtualFS.
func VirtualFSFromMaps(mem map[string][]byte, disk map[string]string) VirtualFS {
	v := VirtualFS{Mem: mem, Disk: disk}
	if v.Mem == nil {
		v.Mem = make(map[string][]byte)
	}
	if v.Disk == nil {
		v.Disk = make(map[string]string)
	}
	return v
}

// PutData stores an in-memory file at a virtual path.
func (v *VirtualFS) PutData(path string, data []byte) {
	if v.Mem == nil {
		v.Mem = make(map[string][]byte)
	}
	v.Mem[path] = data
}

// PutFile registers a host file to stream at a virtual path.
func (v *VirtualFS) PutFile(virtualPath, hostPath string) {
	if v.Disk == nil {
		v.Disk = make(map[string]string)
	}
	v.Disk[virtualPath] = hostPath
}

// UsesDiskStaging reports whether any file is streamed from disk.
func (v VirtualFS) UsesDiskStaging() bool {
	return len(v.Disk) > 0
}

// MergeMem returns a copy of in-memory files merged with extra runtime entries.
func (v VirtualFS) MergeMem(extra map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(v.Mem)+len(extra))
	for k, val := range v.Mem {
		out[k] = val
	}
	for k, val := range extra {
		out[k] = val
	}
	return out
}
