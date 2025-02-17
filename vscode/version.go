package vscode

import (
	"encoding/json"
	"strings"
	"time"
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

func (v Version) TargetPlatform() string {
	if v.RawTargetPlatform == "" {
		return PlatformUniversal
	}
	return v.RawTargetPlatform
}

func (v Version) Tag(uid UniqueID) VersionTag {
	return VersionTag{UniqueID: uid, Version: v.Version, PreRelease: v.IsPreRelease(), TargetPlatform: v.TargetPlatform()}
}
