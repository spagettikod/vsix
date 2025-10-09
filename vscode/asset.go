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

// URI return the correct asset source. The one supplied in the JSON response
// lacks platform information.
func (a Asset) URI(v Version) string {
	if v.TargetPlatform() == PlatformUniversal {
		return fmt.Sprintf("%s/%s", v.AssetURI, a.Type)
	}
	return fmt.Sprintf("%s/%s?targetPlatform=%s", v.AssetURI, a.Type, v.TargetPlatform())
}
