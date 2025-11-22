package vscode

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	PlatformUniversal string = "universal"
)

type Version struct {
	Version           string     `json:"version"`
	RawTargetPlatform string     `json:"targetPlatform,omitempty"`
	Flags             string     `json:"flags"`
	LastUpdated       time.Time  `json:"lastUpdated"`
	Files             []Asset    `json:"files"`
	Properties        []Property `json:"properties"`
	AssetURI          string     `json:"assetUri"`
	FallbackAssetURI  string     `json:"fallbackAssetUri"`
	Path              string     `json:"-"`
}

type Property struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var SortFuncVersion = func(v1, v2 Version) int {
	if semver.Compare("v"+v1.Version, "v"+v2.Version) == 0 {
		return strings.Compare(v1.TargetPlatform(), v2.TargetPlatform())
	}
	return semver.Compare("v"+v1.Version, "v"+v2.Version) * -1
}

func (v Version) GetProperty(key string) (Property, bool) {
	for _, p := range v.Properties {
		if p.Key == key {
			return p, true
		}
	}
	return Property{}, false
}

func (v Version) IsPreRelease() bool {
	if p, found := v.GetProperty("Microsoft.VisualStudio.Code.PreRelease"); found {
		return strings.ToLower(p.Value) == "true"
	}
	return false
}

func (v Version) String() string {
	b, err := json.MarshalIndent(v, "", "   ")
	if err != nil {
		return "! JSON UNMARSHAL FAILED !"
	}
	return string(b)
}

func (v Version) ToJSON() []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("! JSON UNMARSHAL FAILED !")
	}
	return b
}

func (v Version) TargetPlatform() string {
	if v.RawTargetPlatform == "" {
		return PlatformUniversal
	}
	return v.RawTargetPlatform
}

// Tag returns a complete version tag for this version.
func (v Version) Tag(uid UniqueID) VersionTag {
	return VersionTag{UniqueID: uid, Version: v.Version, PreRelease: v.IsPreRelease(), TargetPlatform: v.TargetPlatform()}
}

// Compare versions. Please not this only compare the version.Version field. In other words
// if the two Versions have different platofrms but the same version number it will still
// return true.
func (v Version) Compare(other Version) int {
	v1 := v.Version
	v2 := other.Version
	if strings.Index(v1, "v") != 0 {
		v1 = "v" + v1
	}
	if strings.Index(v2, "v") != 0 {
		v2 = "v" + v2
	}
	return semver.Compare(v1, v2)
}

type Versions []Version

// LatestVersion returns the latest version.
func (vs Versions) LatestVersion(preRelease bool) string {
	latest := Version{}
	for _, v := range vs {
		if latest.Version == "" || v.Compare(latest) > 0 {
			if (v.IsPreRelease() && preRelease) || (!v.IsPreRelease() && !preRelease) {
				latest = v
			}
		}
	}
	return latest.Version
}

// Latest returns a new Versions collection with all versions that match the latest version.
// When preRelase is true we also include the pre-release versions.
func (vs Versions) Latest(preRelease bool) Versions {
	newVs := Versions{}
	latest := vs.LatestVersion(false)
	latestPreRelease := vs.LatestVersion(true)
	for _, v := range vs {
		if v.Version == latest {
			newVs = append(newVs, v)
		}
		if preRelease && v.Version == latestPreRelease && v.IsPreRelease() {
			newVs = append(newVs, v)
		}
	}
	newVs.Sort()
	return newVs
}

func (vs Versions) ToJSON() []byte {
	b, err := json.Marshal(vs)
	if err != nil {
		return []byte("! JSON UNMARSHAL FAILED !")
	}
	return b
}

// Sort sorts the versions with the latest version as the first item.
func (vs Versions) Sort() {
	slices.SortFunc(vs, func(v1, v2 Version) int {
		return v1.Compare(v2) * -1
	})
}
