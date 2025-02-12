package vscode

import "fmt"

// VersionTag is a unique identifier for a combination of
// extension/version/platform.
type VersionTag struct {
	UniqueID       UniqueID
	Version        string
	TargetPlatform string
	PreRelease     bool
}

func (vt VersionTag) String() string {
	return fmt.Sprintf("%s@%s:%s", vt.UniqueID, vt.Version, vt.TargetPlatform)
}
