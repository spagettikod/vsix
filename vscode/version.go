package vscode

import (
	"encoding/json"
	"errors"
	"path"
	"strings"
	"time"
)

const (
	PlatformUniversal string = "universal"
)

var (
	ErrVersionValidation = errors.New("version failed validation")
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

func (v Version) GetProperty(key string) (Property, bool) {
	for _, p := range v.Properties {
		if p.Key == key {
			return p, true
		}
	}
	return Property{}, false
}

// ID returns the number in the Asset path returned from the Marketplace. Each version
// has a asset URI, for example https://ms-vscode.gallerycdn.vsassets.io/extensions/ms-vscode/cpptools/1.9.0/1644541363277,
// this method return the unique number last in the path.
func (v Version) ID() string {
	return path.Base(v.AssetURI)
}

func (v Version) IsPreRelease() bool {
	if p, found := v.GetProperty("Microsoft.VisualStudio.Code.PreRelease"); found {
		return strings.ToLower(p.Value) == "true"
	}
	return false
}

func (v Version) Equals(comp Version) bool {
	return (v.Version == comp.Version) && (v.ID() == comp.ID())
}

func (v Version) Copy() Version {
	v2 := v
	v2.Files = append([]Asset{}, v.Files...)
	v2.Properties = append([]Property{}, v.Properties...)
	return v2
}

func (v Version) String() string {
	b, err := json.MarshalIndent(v, "", "   ")
	if err != nil {
		return "! JSON UNMARSHAL FAILED !"
	}
	return string(b)
}

func (v Version) TargetPlatform() string {
	if v.RawTargetPlatform == "" {
		return PlatformUniversal
	}
	return v.RawTargetPlatform
}

func (v Version) Tag(uid UniqueID) VersionTag {
	return VersionTag{UniqueID: uid, Version: v.Version, PreRelease: v.IsPreRelease(), TargetPlatform: v.TargetPlatform()}
}
