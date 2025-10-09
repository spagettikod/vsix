package storage

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/afero"
)

const (
	FSBackendDir = "extensions"
)

type FSBackend struct {
	BaseBackend
	root string
	fs   afero.Fs
}

func NewFSBackend(root string) (Backend, error) {
	fs := afero.NewBasePathFs(afero.NewOsFs(), root)
	b := &FSBackend{
		root: root,
		fs:   fs,
	}
	b.BaseBackend = BaseBackend{impl: b}

	if err := os.MkdirAll(root, 0750); err != nil {
		if !os.IsExist(err) {
			log.Fatalln(err)
		}
	}

	return b, nil
}

func (b FSBackend) ListVersionTags(uid vscode.UniqueID) ([]vscode.VersionTag, error) {
	uidfs := afero.NewBasePathFs(b.fs, ExtensionPath(uid))
	matches, err := afero.Glob(uidfs, filepath.Join("*", "*"))
	if err != nil {
		return nil, err
	}
	vts := []vscode.VersionTag{}
	for _, m := range matches {
		split := strings.Split(m, string(os.PathSeparator))
		if len(split) != 2 {
			return nil, fmt.Errorf("error parsing version tag, could not split path: %s", m)
		}
		vt := vscode.VersionTag{
			UniqueID:       uid,
			Version:        split[0],
			TargetPlatform: split[1],
		}
		vts = append(vts, vt)
		slog.Debug("found version", "stringValue", vt.String(), "path", filepath.Join(b.root, ExtensionPath(uid), m))
	}
	return vts, nil
}

func (b FSBackend) LoadExtensionMetadata(uid vscode.UniqueID) ([]byte, error) {
	metaFile := filepath.Join(ExtensionPath(uid), extensionMetadataFilename)
	f, err := b.fs.Open(metaFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (b FSBackend) LoadVersionMetadata(tag vscode.VersionTag) ([]byte, error) {
	metaFile := filepath.Join(AssetPath(tag), versionMetadataFilename)
	f, err := b.fs.Open(metaFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (b FSBackend) SaveExtensionMetadata(ext vscode.Extension) error {
	ext.Versions = []vscode.Version{}
	if err := b.fs.MkdirAll(ExtensionPath(ext.UniqueID()), 0755); err != nil {
		return err
	}
	fpath := filepath.Join(ExtensionPath(ext.UniqueID()), extensionMetadataFilename)
	return afero.WriteFile(b.fs, fpath, []byte(ext.String()), os.ModePerm)
}

func (b FSBackend) SaveVersionMetadata(uid vscode.UniqueID, v vscode.Version) error {
	p := AssetPath(v.Tag(uid))
	if err := b.fs.MkdirAll(p, 0755); err != nil {
		return err
	}
	fpath := filepath.Join(p, versionMetadataFilename)
	return afero.WriteFile(b.fs, fpath, []byte(v.String()), os.ModePerm)
}

func (b FSBackend) SaveAsset(tag vscode.VersionTag, atype vscode.AssetTypeKey, contentType string, data []byte) error {
	p := AssetPath(tag)
	if err := b.fs.MkdirAll(p, 0755); err != nil {
		return err
	}
	fpath := filepath.Join(p, string(atype))
	return afero.WriteFile(b.fs, fpath, data, os.ModePerm)
}

func (b FSBackend) isDirectoryEmpty(path string) (bool, error) {
	// Open the directory
	dir, err := b.fs.Open(path)
	if err != nil {
		return false, err
	}
	defer dir.Close()

	// Read the directory contents
	entries, err := dir.Readdir(-1)
	if err != nil {
		return false, err
	}

	// Check if the directory is empty
	return len(entries) == 0, nil
}

func (b FSBackend) Remove(tag vscode.VersionTag) error {
	p := AssetPath(tag)
	if tag.HasVersion() {
		if !tag.HasTargetPlatform() {
			p = filepath.Dir(p)
		}
	}

	slog.Debug("removing", "path", filepath.Join(b.root, p), "tag", tag.String())
	if err := b.fs.RemoveAll(p); err != nil {
		return err
	}

	// remove the version path if there are no more platform versions
	empty, err := b.isDirectoryEmpty(VersionPath(tag))
	if err != nil {
		return err
	}
	if empty {
		slog.Debug("no platforms left, removing version folder", "path", filepath.Join(b.root, VersionPath(tag)), "tag", tag.String())
		if err := b.fs.RemoveAll(VersionPath(tag)); err != nil {
			return err
		}
	} else {
		slog.Debug("there are still platforms left, remove finished")
		return nil
	}

	// remove extension folder if there are no versions left
	tags, err := b.ListVersionTags(tag.UniqueID)
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		extensionDir := ExtensionPath(tag.UniqueID)
		slog.Debug("no version left, removing extension folder", "path", filepath.Join(b.root, extensionDir), "tag", tag.String())
		if err := b.fs.RemoveAll(extensionDir); err != nil {
			return err
		}
	} else {
		slog.Debug("there are still versions left, remove finished")
		return nil
	}

	// remove publisher folder if there are no more extensions for this publisher
	empty, err = b.isDirectoryEmpty(tag.UniqueID.Publisher)
	if err != nil {
		return err
	}
	if empty {
		slog.Debug("no extensions left, removing publisher folder", "path", filepath.Join(b.root, tag.UniqueID.Publisher), "tag", tag.String())
		if err := b.fs.RemoveAll(tag.UniqueID.Publisher); err != nil {
			return err
		}
	} else {
		slog.Debug("there are still extensions left, remove finished")
	}
	return nil
}

func (b FSBackend) LoadAsset(tag vscode.VersionTag, atype vscode.AssetTypeKey) (io.ReadCloser, error) {
	return b.fs.Open(filepath.Join(AssetPath(tag), string(atype)))
}

func (b FSBackend) listUniqueID() ([]vscode.UniqueID, error) {
	matches, err := afero.Glob(b.fs, filepath.Join("*", "*"))
	if err != nil {
		return nil, err
	}
	uids := []vscode.UniqueID{}
	for _, m := range matches {
		m = strings.Replace(m, string(os.PathSeparator), ".", 1)
		uid, ok := vscode.Parse(m)
		if !ok {
			return nil, fmt.Errorf("could not parse as unique identifier: %s", m)
		}
		slog.Debug("extension found", "publisher", uid.Publisher, "name", uid.Name, "path", filepath.Join(b.root, m))
		uids = append(uids, uid)
	}
	return uids, nil
}
