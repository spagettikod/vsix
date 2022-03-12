package database

import (
	"path"

	"github.com/spagettikod/vsix/vscode"
)

const (
	extensionMetadataFileName string = "_vsix_db_extension_metadata.json"
	versionMetadataFileName   string = "_vsix_db_version_metadata.json"
)

// ExtensionDir return the path to the extension in the database store where
// root is the root path to the database.
// For example: /var/mydb/golang/go
func ExtensionDir(root string, e vscode.Extension) string {
	return path.Join(root, e.Publisher.Name, e.Name)
}

// VersionPath return the path to the extension and given version in the database store
// where root is the root path to the database.
// For example: /var/mydb/golang/go/0.29.0
func VersionDir(root string, e vscode.Extension, v vscode.Version) string {
	return path.Join(ExtensionDir(root, e), v.Version, v.ID())
}

// ExtensionMetaFile return the path to the metadata.json file for this extension.
func ExtensionMetaFile(root string, e vscode.Extension) string {
	return path.Join(ExtensionDir(root, e), extensionMetadataFileName)
}

// VersionMetaFile returns the file path to the metadata file for a given version.
func VersionMetaFile(root string, e vscode.Extension, v vscode.Version) string {
	return path.Join(VersionDir(root, e, v), versionMetadataFileName)
}

// AssetFile returns the file name, including dir, for a asset.
func AssetFile(root string, e vscode.Extension, v vscode.Version, a vscode.Asset) string {
	return path.Join(VersionDir(root, e, v), string(a.Type))
}
