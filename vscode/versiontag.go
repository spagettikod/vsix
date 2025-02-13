package vscode

import (
	"fmt"
	"net/http"
)

const (
	AssetURLPattern = "{publisher}/{name}/{version}/{platform}/{type}"
)

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

// Pattern will return the VersionTag as a URL path matching AssetURLPattern. Used when creating a URL compatible with VSIX serve for the given VersionTag.
func (vt VersionTag) Pattern(assetType AssetTypeKey) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", vt.UniqueID.Publisher, vt.UniqueID.Name, vt.Version, vt.TargetPlatform, string(assetType))
}

// ParseAssetURL parses the URL pattern in AssetURLPattern and return necessary objects needed to load the asset from database.
func ParseAssetURL(r *http.Request) (VersionTag, AssetTypeKey, error) {
	uid, ok := Parse(fmt.Sprintf("%s.%s", r.PathValue("publisher"), r.PathValue("name")))
	if !ok {
		return VersionTag{}, "", fmt.Errorf("invalid unique identifier")
	}
	version := r.PathValue("version")
	platform := r.PathValue("platform")
	assetType, err := StrToAssetType(r.PathValue("type"))
	if err != nil {
		return VersionTag{}, "", fmt.Errorf("invalid asset type: %w", err)
	}
	return VersionTag{UniqueID: uid, Version: version, TargetPlatform: platform}, assetType, nil
}
