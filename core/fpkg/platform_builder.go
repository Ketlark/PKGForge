package fpkg

// DiscMetadata is platform-agnostic title information extracted from a disc image.
type DiscMetadata struct {
	GameID string
	Title  string
	Region string
}

// DiscProjectBuilder assembles the virtual file tree for a disc-based emulator fPKG.
//
// PS1 and PS2 implement this pattern via BuildPS1Project and BuildPS2Project.
// The interface documents the intended extension point; callers use the concrete
// build functions today because their option types differ.
type DiscProjectBuilder interface {
	BuildProject() (VirtualFS, DiscMetadata, error)
}
