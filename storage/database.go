package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/afero"
	"golang.org/x/mod/semver"
)

const (
	modFilename               string = ".modfile"
	extensionMetadataFilename        = "_vsix_db_extension_metadata.json"
	versionMetadataFilename          = "_vsix_db_version_metadata.json"
)

var (
	ErrExtensionMetadataNotFound = errors.New("metadata file for the given extension was not found")
	ErrMissingAsset              = errors.New("version asset in metadata file was not found in the database")
)

type Database struct {
	root          string
	items         *sync.Map
	assetEndpoint string
	loadDuration  time.Duration
	loadedAt      time.Time
	modFile       string
	watcher       *fsnotify.Watcher
	dblog         *slog.Logger
	fs            afero.Fs
}

func OpenMem() (*Database, error) {
	return open(afero.NewMemMapFs(), "")
}

func OpenFs(root string) (*Database, error) {
	return open(afero.NewBasePathFs(afero.NewOsFs(), root), root)
}

func new(root string, fs afero.Fs) (*Database, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	dblog := slog.With("component", "database", "path", absRoot)
	return &Database{
		root:    absRoot,
		items:   &sync.Map{},
		modFile: path.Join(root, modFilename),
		dblog:   dblog,
		fs:      fs,
	}, nil
}

func open(fs afero.Fs, root string) (*Database, error) {
	db, err := new(root, fs)
	if err != nil {
		return nil, err
	}
	if err := db.index(); err != nil {
		return nil, err
	}

	return db, err
}

func (db Database) List() []vscode.Extension {
	exts := []vscode.Extension{}
	db.items.Range(func(key, value any) bool {
		exts = append(exts, value.(vscode.Extension))
		return true
	})
	return exts
}

func (db Database) ListUniqueIDs() []vscode.UniqueID {
	uids := []vscode.UniqueID{}
	db.items.Range(func(key, value any) bool {
		uids = append(uids, key.(vscode.UniqueID))
		return true
	})
	return uids
}

func (db Database) FindByUniqueID(uid vscode.UniqueID) (vscode.Extension, bool) {
	anyext, found := db.items.Load(uid)
	if !found {
		return vscode.Extension{}, false
	}
	return anyext.(vscode.Extension), found
}

func (db Database) FindByVersionTag(tag vscode.VersionTag) (vscode.Version, bool) {
	ext, found := db.FindByUniqueID(tag.UniqueID)
	if !found {
		return vscode.Version{}, false
	}
	if v, found := ext.VersionByTag(tag); found {
		return v, true
	}
	return vscode.Version{}, false
}

// FindByTargetPlatforms returns all extensions having versions for the given target platform(s).
func (db Database) FindByTargetPlatforms(targetPlatforms ...string) []vscode.Extension {
	exts := []vscode.Extension{}
	for _, ext := range db.List() {
		for _, v := range ext.Versions {
			if slices.Contains(targetPlatforms, v.TargetPlatform()) {
				exts = append(exts, ext)
				// exit version loop and continue with the next extension since we've found one match
				break
			}
		}
	}
	return exts
}

func (db Database) SaveExtensionMetadata(ext vscode.Extension) error {
	// clear version information, this is saved per version later on
	ext.Versions = []vscode.Version{}
	if err := db.fs.MkdirAll(db.extensionPath(ext.UniqueID()), 0755); err != nil {
		return err
	}
	fpath := filepath.Join(db.extensionPath(ext.UniqueID()), extensionMetadataFilename)
	return afero.WriteFile(db.fs, fpath, []byte(ext.String()), os.ModePerm)
}

func (db Database) SaveVersionMetadata(uid vscode.UniqueID, v vscode.Version) error {
	p := db.assetPath(v.Tag(uid))
	if err := db.fs.MkdirAll(p, 0755); err != nil {
		return err
	}
	fpath := filepath.Join(p, versionMetadataFilename)
	return afero.WriteFile(db.fs, fpath, []byte(v.String()), os.ModePerm)
}

func (db Database) SaveAsset(tag vscode.VersionTag, atype vscode.AssetTypeKey, r io.ReadCloser) error {
	p := db.assetPath(tag)
	if err := db.fs.MkdirAll(p, 0755); err != nil {
		return err
	}
	fpath := filepath.Join(p, string(atype))
	return afero.WriteReader(db.fs, fpath, r)
}

func (db Database) fsListUniqueID() ([]vscode.UniqueID, error) {
	uids := []vscode.UniqueID{}
	paths, err := afero.Glob(db.fs, filepath.Join("*/**"))
	if err != nil {
		return uids, err
	}
	for _, p := range paths {
		uid := vscode.UniqueID{Publisher: filepath.Dir(p), Name: filepath.Base(p)}
		uids = append(uids, uid)
	}
	return uids, nil
}

// fsListVersionTags lists all (partial) version tags for a unique id. Please not the tag
// is partial since the tag is created from the storage path and not the version metadata
// which leads to the pre-release flag is not be used.
func (db Database) fsListVersionTags(uid vscode.UniqueID) ([]vscode.VersionTag, error) {
	tags := []vscode.VersionTag{}
	paths, err := afero.Glob(db.fs, filepath.Join(db.extensionPath(uid), "*/**"))
	if err != nil {
		return tags, err
	}
	for _, p := range paths {
		vpath, err := filepath.Rel(db.extensionPath(uid), p)
		if err != nil {
			return tags, err
		}
		t := vscode.VersionTag{
			UniqueID:       uid,
			Version:        filepath.Dir(vpath),
			TargetPlatform: filepath.Base(vpath),
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (db Database) index() error {
	uids, err := db.fsListUniqueID()
	if err != nil {
		return err
	}

	const maxGoroutines = 10 // Limit to 3 concurrent goroutines
	sem := make(chan struct{}, maxGoroutines)
	wg := sync.WaitGroup{}
	var indexingError error
	for _, uid := range uids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{} // Block if the semaphore is full
			defer func() { <-sem }()

			ext, err := db.loadExtensionMetadata(uid)
			if err != nil {
				indexingError = fmt.Errorf("indexing failed for extension %s: %w", uid.String(), err)
			}
			if err := db.loadAllVersionMetadata(&ext); err != nil {
				indexingError = fmt.Errorf("indexing failed for extension %s: %w", uid.String(), err)
			}
			db.items.Store(uid, ext)
		}()
	}
	wg.Wait()
	if indexingError != nil {
		return indexingError
	}
	return nil
}

// loadAllVersionMetadata populates the extension with the metadata from all versions
// in storage.
func (db Database) loadAllVersionMetadata(ext *vscode.Extension) error {
	tags, err := db.fsListVersionTags(ext.UniqueID())
	if err != nil {
		return err
	}
	ext.Versions = []vscode.Version{}
	for _, tag := range tags {
		vmeta, err := db.loadVersionMetadata(tag)
		if err != nil {
			return err
		}
		ext.Versions = append(ext.Versions, vmeta)
	}
	slices.SortFunc(ext.Versions, func(v1, v2 vscode.Version) int {
		return semver.Compare("v"+v1.Version, "v"+v2.Version) * -1
	})
	return nil
}

func (db Database) loadVersionMetadata(tag vscode.VersionTag) (vscode.Version, error) {
	vmeta := vscode.Version{}
	bites, err := afero.ReadFile(db.fs, filepath.Join(db.assetPath(tag), versionMetadataFilename))
	if err != nil {
		return vscode.Version{}, err
	}
	return vmeta, json.Unmarshal(bites, &vmeta)
}

// ValidateVersion check if all assets for the given version exist in the database.
func (db Database) ValidateVersion(tag vscode.VersionTag) error {
	vmeta, err := db.loadVersionMetadata(tag)
	if err != nil {
		return err
	}
	for _, asset := range vmeta.Files {
		found, err := afero.Exists(db.fs, filepath.Join(db.assetPath(tag), string(asset.Type)))
		if err != nil {
			return err
		}
		if !found {
			return ErrMissingAsset
		}
	}
	return nil
}

func (db Database) loadExtensionMetadata(uid vscode.UniqueID) (vscode.Extension, error) {
	ext := vscode.Extension{}
	s, err := afero.Glob(db.fs, filepath.Join(db.extensionPath(uid), extensionMetadataFilename))
	if err != nil {
		return vscode.Extension{}, err
	}
	if len(s) != 1 {
		if len(s) == 0 {
			return vscode.Extension{}, ErrExtensionMetadataNotFound
		}
		return vscode.Extension{}, errors.New("multiple metadata files found, this should not happen")
	}

	bites, err := afero.ReadFile(db.fs, s[0])
	if err != nil {
		return vscode.Extension{}, err
	}

	return ext, json.Unmarshal(bites, &ext)
}

// extensionPath returns the asset path for a given ExtensionTag
func (db Database) extensionPath(uid vscode.UniqueID) string {
	return filepath.Join(uid.Publisher, uid.Name)
}

// assetPath returns the asset path for a given ExtensionTag
func (db Database) assetPath(tag vscode.VersionTag) string {
	return filepath.Join(db.extensionPath(tag.UniqueID), tag.Version, tag.TargetPlatform)
}
