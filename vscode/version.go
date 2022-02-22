package vscode

import (
	"encoding/json"
	"path"
	"time"
)

type Version struct {
	Version          string     `json:"version"`
	TargetPlatform   string     `json:"targetPlatform"`
	Flags            string     `json:"flags"`
	LastUpdated      time.Time  `json:"lastUpdated"`
	Files            []Asset    `json:"files"`
	Properties       []Property `json:"properties"`
	AssetURI         string     `json:"assetUri"`
	FallbackAssetURI string     `json:"fallbackAssetUri"`
	Path             string     `json:"-"`
}

type Property struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ID returns the number in the Asset path returned from the Marketplace. Each version
// has a asset URI, for example https://ms-vscode.gallerycdn.vsassets.io/extensions/ms-vscode/cpptools/1.9.0/1644541363277,
// this method return the unique number last in the path.
func (v Version) ID() string {
	return path.Base(v.AssetURI)
}

func (v Version) Equals(comp Version) bool {
	return (v.Version == comp.Version) && (v.ID() == comp.ID())
}

func (v Version) String() string {
	b, err := json.MarshalIndent(v, "", "   ")
	if err != nil {
		return "! JSON UNMARSHAL FAILED !"
	}
	return string(b)
}
