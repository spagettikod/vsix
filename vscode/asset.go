package vscode

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
)

type AssetTypeKey string

type Asset struct {
	Type   AssetTypeKey `json:"assetType"`
	Source string       `json:"source"`
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

func (a Asset) Abs(p string) string {
	u, err := url.Parse(a.Source)
	if err != nil {
		return ""
	}
	return path.Join(p, u.Path)
}

func (a Asset) Download(p string) error {
	u, err := url.Parse(a.Source)
	if err != nil {
		return err
	}
	resp, err := http.Get(a.Source)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(path.Join(p, path.Dir(u.Path)), os.ModePerm); err != nil {
		return err
	}
	return ioutil.WriteFile(a.Abs(p), b, os.ModePerm)
}
