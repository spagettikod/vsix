package vscode

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	VersionTagVersionSeparator        = "@"
	VersionTagTargetPlatformSeparator = ":"
	AssetURLPattern                   = "{publisher}/{name}/{version}/{platform}/{type}"
)

var (
	ErrInvalidVersionTag = errors.New("invalid version tag")
)

// VersionTag is a unique identifier for a combination of
// extension/version/platform.
type VersionTag struct {
	UniqueID       UniqueID
	Version        string
	TargetPlatform string
	PreRelease     bool
}

var SortFuncVersionTag = func(v1, v2 VersionTag) int {
	if semver.Compare("v"+v1.Version, "v"+v2.Version) == 0 {
		return strings.Compare(v1.TargetPlatform, v2.TargetPlatform)
	}
	return semver.Compare("v"+v1.Version, "v"+v2.Version) * -1
}

// tagSplit splits the tag with the given separator returning the remainder and the
// split text. Example tagSplit("a@b:c") would return a@b and c.
func tagSplit(strTag, sep string) (string, string) {
	split := strings.Split(strTag, sep)
	if len(split) == 2 {
		return split[0], split[1]
	}
	return strTag, ""
}

// ParseVersionTag parse string into a VersionTag. Valid format <UID>[:VERSION[:TARGET_PLATFORM]].
func ParseVersionTag(strTag string) (VersionTag, error) {
	remainder, platform := tagSplit(strTag, VersionTagTargetPlatformSeparator)
	uidStr, version := tagSplit(remainder, VersionTagVersionSeparator)

	// fail if
	// * we have separators for version and target platform but the values are empty
	// * we have target platform separator but no version
	if (strings.Contains(strTag, VersionTagTargetPlatformSeparator) && platform == "") ||
		(strings.Contains(strTag, VersionTagVersionSeparator) && version == "") ||
		(strings.Contains(strTag, VersionTagTargetPlatformSeparator) && version == "") {
		return VersionTag{}, fmt.Errorf("%w: %s", ErrInvalidVersionTag, strTag)
	}

	uid, ok := Parse(uidStr)
	if !ok {
		return VersionTag{}, fmt.Errorf("%w: could not parse unique id: %s", ErrInvalidVersionTag, strTag)
	}
	return VersionTag{
		UniqueID:       uid,
		Version:        version,
		TargetPlatform: platform}, nil
}

func (vt VersionTag) Validate() bool {
	if vt.UniqueID.Valid() {
		return !(vt.TargetPlatform != "" && vt.Version == "")
	}
	return false
}

func (vt VersionTag) String() string {
	str := vt.UniqueID.String()
	if vt.Version != "" {
		str += VersionTagVersionSeparator + vt.Version
		if vt.TargetPlatform != "" {
			str += VersionTagTargetPlatformSeparator + vt.TargetPlatform
		}
	}
	return str
}

// Pattern will return the VersionTag as a URL path.
func (vt VersionTag) Pattern() string {
	return fmt.Sprintf("%s/%s/%s/%s", vt.UniqueID.Publisher, vt.UniqueID.Name, vt.Version, vt.TargetPlatform)
}

// PatternByAssetType will return the VersionTag as a URL path matching AssetURLPattern. Used when creating a URL compatible with VSIX serve for the given VersionTag.
func (vt VersionTag) PatternByAssetType(assetType AssetTypeKey) string {
	return fmt.Sprintf("%s/%s", vt.Pattern(), string(assetType))
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
