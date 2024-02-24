package vscode

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type AssetTypeKey string

type Asset struct {
	Type   AssetTypeKey `json:"assetType"`
	Source string       `json:"source"`
	Path   string       `json:"-"`
}

const (
	Manifest         AssetTypeKey = "Microsoft.VisualStudio.Code.Manifest"
	ContentChangelog AssetTypeKey = "Microsoft.VisualStudio.Services.Content.Changelog"
	ContentDetails   AssetTypeKey = "Microsoft.VisualStudio.Services.Content.Details"
	ContentLicense   AssetTypeKey = "Microsoft.VisualStudio.Services.Content.License"
	IconsDefault     AssetTypeKey = "Microsoft.VisualStudio.Services.Icons.Default"
	IconsSmall       AssetTypeKey = "Microsoft.VisualStudio.Services.Icons.Small"
	VSIXManifest     AssetTypeKey = "Microsoft.VisualStudio.Services.VsixManifest"
	VSIXPackage      AssetTypeKey = "Microsoft.VisualStudio.Services.VSIXPackage"
	VSIXSignature    AssetTypeKey = "Microsoft.VisualStudio.Services.VsixSignature"
)

func StrToAssetType(assetType string) (AssetTypeKey, error) {
	switch assetType {
	case string(Manifest):
		return Manifest, nil
	case string(ContentChangelog):
		return ContentChangelog, nil
	case string(ContentDetails):
		return ContentDetails, nil
	case string(ContentLicense):
		return ContentLicense, nil
	case string(IconsDefault):
		return IconsDefault, nil
	case string(IconsSmall):
		return IconsSmall, nil
	case string(VSIXManifest):
		return VSIXManifest, nil
	case string(VSIXPackage):
		return VSIXPackage, nil
	case string(VSIXSignature):
		return VSIXSignature, nil
	default:
		return "nil", fmt.Errorf("unknown asset type: %v", assetType)
	}
}

func NewAsset(assetType AssetTypeKey, source string) Asset {
	return Asset{Type: assetType, Source: source}
}

func (a Asset) Is(t AssetTypeKey) bool {
	return a.Type == t
}

func (a Asset) Download() ([]byte, error) {
	resp, err := http.Get(a.Source)
	if err != nil {
		return []byte{}, err
	}
	return ioutil.ReadAll(resp.Body)
}

// Validate return true if the asset is valid. It checks if
// the file exist and that it's not a directory.
func (a Asset) Validate() (bool, error) {
	fi, err := os.Stat(a.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("not exists", a.Path)
			return false, nil
		}
		return false, err
	}
	if fi.IsDir() {
		fmt.Println("isdir")
		return false, nil
	}
	return true, nil
}
