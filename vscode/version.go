package vscode

import (
	"encoding/json"
	"time"
)

type Version struct {
	Version          string     `json:"version"`
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

func (v Version) String() string {
	b, err := json.MarshalIndent(v, "", "   ")
	if err != nil {
		return "! JSON UNMARSHAL FAILED !"
	}
	return string(b)
}
