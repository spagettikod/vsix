package vscode

// VersionTag is a unique identifier for a combination of
// extension/version/platform.
type VersionTag struct {
	UniqueID       UniqueID
	Version        string
	TargetPlatform string
	PreRelease     bool
}
