package vscode

import (
	"fmt"
)

type AssetTypeKey string

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

type Asset struct {
	Type   AssetTypeKey `json:"assetType"`
	Source string       `json:"source"`
	Path   string       `json:"-"`
}
