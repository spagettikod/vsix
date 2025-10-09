package storage

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spagettikod/vsix/vscode"
)

type Backend interface {
	listUniqueID() ([]vscode.UniqueID, error)
	DetectAssetContentType(vscode.VersionTag, vscode.AssetTypeKey) (string, error)
	ListVersionTags(vscode.UniqueID) ([]vscode.VersionTag, error)
	LoadAsset(vscode.VersionTag, vscode.AssetTypeKey) (io.ReadCloser, error)
	LoadExtensionMetadata(vscode.UniqueID) ([]byte, error)
	LoadVersionMetadata(vscode.VersionTag) ([]byte, error)
	Remove(vscode.VersionTag) error
	SaveAsset(vscode.VersionTag, vscode.AssetTypeKey, string, []byte) error
	SaveExtensionMetadata(vscode.Extension) error
	SaveVersionMetadata(vscode.UniqueID, vscode.Version) error
}

type BaseBackend struct {
	impl Backend
}

func (bb *BaseBackend) DetectAssetContentType(tag vscode.VersionTag, assetType vscode.AssetTypeKey) (string, error) {
	asset, err := bb.impl.LoadAsset(tag, assetType)
	if err != nil {
		return "", err
	}
	defer asset.Close()

	buffer := make([]byte, 512)
	_, err = asset.Read(buffer)
	if err != nil {
		return "", err
	}

	return http.DetectContentType(buffer), nil
}

type BackendType string

const (
	BackendTypeS3 BackendType = "s3"
	BackendTypeFS BackendType = "fs"

	extensionMetadataFilename = "_vsix_db_extension_metadata.json"
	versionMetadataFilename   = "_vsix_db_version_metadata.json"
)

var (
	ErrExtensionMetadataNotFound = errors.New("extension metadata missing")
	ErrVersionMetadataNotFound   = errors.New("version metadata missing")
	ErrNoVersions                = errors.New("extension has no versions")
	ErrMissingAsset              = errors.New("asset not found")
)

type ValidationError struct {
	Tag   vscode.VersionTag
	Error error
}

// ExtensionPath returns the asset path for a given ExtensionTag
func ExtensionPath(uid vscode.UniqueID) string {
	return filepath.Join(uid.Publisher, uid.Name)
}

// AssetPath returns the asset path for a given ExtensionTag. For example: redhat/java/1.23.3.
func VersionPath(tag vscode.VersionTag) string {
	return filepath.Join(ExtensionPath(tag.UniqueID), tag.Version)
}

// AssetPath returns the asset path for a given ExtensionTag. For example: redhat/java/1.23.3/darwin-arm64.
func AssetPath(tag vscode.VersionTag) string {
	return filepath.Join(ExtensionPath(tag.UniqueID), tag.Version, tag.TargetPlatform)
}

// UserDataDir returns the default root directory to use for user-specific
// configuration data. Users should create their own application-specific
// subdirectory within this one and use that.
//
// On Unix systems, it returns $XDG_DATA_HOME as specified by
// https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html if
// non-empty, else $HOME/.local/share.
// On Darwin, it returns $HOME/Library/Application Support.
// On Windows, it returns %AppData%.
// On Plan 9, it returns $home/lib.
//
// If the location cannot be determined (for example, $HOME is not defined) or
// the path in $XDG_CONFIG_HOME is relative, then it will return an error.
func UserDataDir() (string, error) {
	// copy of os.UserConfigDir from go 1.25.1
	var dir string

	switch runtime.GOOS {
	case "windows":
		dir = os.Getenv("AppData")
		if dir == "" {
			return "", errors.New("%AppData% is not defined")
		}

	case "darwin", "ios":
		dir = os.Getenv("HOME")
		if dir == "" {
			return "", errors.New("$HOME is not defined")
		}
		dir += "/Library/Application Support"

	case "plan9":
		dir = os.Getenv("home")
		if dir == "" {
			return "", errors.New("$home is not defined")
		}
		dir += "/lib"

	default: // Unix
		dir = os.Getenv("XDG_DATA_HOME")
		if dir == "" {
			dir = os.Getenv("HOME")
			if dir == "" {
				return "", errors.New("neither $XDG_DATA_HOME nor $HOME are defined")
			}
			dir += "/.local/share"
		} else if !filepath.IsAbs(dir) {
			return "", errors.New("path in $XDG_DATA_HOME is relative")
		}
	}

	return dir, nil
}
