package vscode

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
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

// Abs returns the local file path by parsing the extension URL and adding the local path p.
// func (a Asset) Abs(p string) string {
// 	u, err := url.Parse(a.Source)
// 	if err != nil {
// 		return ""
// 	}

// 	// remote path has the format https://golang.gallerycdn.vsassets.io/extensions/golang/go/0.26.0/1623958451720/Microsoft.VisualStudio.Code.Manifest
// 	// we want to remove the extensions part for local storage. This makes it easier to find the extensions later when serving them.
// 	localPath := u.Path[len("/extensions"):]

// 	return path.Join(p, localPath)
// }

func (a Asset) Download(versionPath string) error {
	resp, err := http.Get(a.Source)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	filename := path.Join(versionPath, string(a.Type))
	return ioutil.WriteFile(filename, b, os.ModePerm)
}
