package vscode

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"time"
)

const (
	versionMetadataFileName = "_vsix_db_version_metadata.json"
)

type Version struct {
	AssetURI         string     `json:"assetUri"`
	FallbackAssetURI string     `json:"fallbackAssetUri"`
	Files            []Asset    `json:"files"`
	Flags            string     `json:"flags"`
	LastUpdated      time.Time  `json:"lastUpdated"`
	Properties       []Property `json:"properties"`
	Version          string     `json:"version"`
	Path             string     `json:"-"`
}

type Property struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func AbsVersionMetadataFile(versionRoot string) string {
	return path.Join(versionRoot, versionMetadataFileName)
}

func (v Version) SaveMetadata(versionRoot string) error {
	j, err := json.MarshalIndent(v, "", "   ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(AbsVersionMetadataFile(versionRoot), j, os.ModePerm)
}
