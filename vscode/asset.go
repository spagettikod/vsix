package vscode

import (
	"fmt"
	"io"
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

func (a Asset) Download(destFilename string) error {
	resp, err := http.Get(a.Source)
	if err != nil {
		return err
	}
	f, err := os.Create(destFilename)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
