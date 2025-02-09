package storage

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/afero"
)

const (
	modFilename               string = ".modfile"
	extensionMetadataFilename        = "_vsix_db_extension_metadata.json"
	versionMetadataFilename          = "_vsix_db_version_metadata.json"
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
	return open(afero.NewMemMapFs(), "/data", false)
}

func OpenFs(root string, autoreload bool) (*Database, error) {
	return open(afero.NewOsFs(), root, autoreload)
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

func open(fs afero.Fs, root string, autoreload bool) (*Database, error) {
	db, err := new(root, fs)
	if err != nil {
		return nil, err
	}
	if err := db.load(); err != nil {
		return nil, err
	}

	// FIXME autoreload
	// if autoreload {
	// 	err := db.autoreload()
	// 	if err != nil {
	// 		return db, err
	// 	}
	// }

	// FIXME
	// if db.Empty() {
	// 	db.dblog.Info().Msgf("could not find any extensions at %v", root)
	// } else {
	// 	stats := db.Stats()
	// 	db.dblog.Info().Msgf("database contains %v extensions with a total of %v versions", stats.ExtensionCount, stats.VersionCount)
	// }

	return db, err
}

func (db *Database) load() error {
	// start := time.Now()
	// db.dblog.Debug().Msg("loading database")
	// exts := []vscode.Extension{}
	// for _, extensionRoot := range db.listExtensions() {
	// 	db.dblog.Debug().Str("path", extensionRoot).Msg("loading extension")
	// 	ext, err := db.loadExtension(extensionRoot)
	// 	if err != nil {
	// 		db.dblog.Error().Err(err).Str("path", extensionRoot).Msg("error while loading extension, skipping")
	// 		continue
	// 	}
	// 	versions, _ := db.listVersions(ext)
	// 	if len(versions) == 0 {
	// 		// db.dblog.Info().Str("path", extensionRoot).Msg("extension does not have any versions, skipping")
	// 		db.dblog.Info().Str("path", extensionRoot).Msg("extension does not have any versions")
	// 		// continue
	// 	}
	// 	for _, version := range versions {
	// 		versionRoot := VersionDir(db.root, ext, version)
	// 		db.dblog.Debug().Str("path", versionRoot).Msg("loading version")
	// 		version.Path = versionRoot
	// 		version.AssetURI = db.assetEndpoint + VersionDir("", ext, version)
	// 		version.FallbackAssetURI = version.AssetURI
	// 		version.Files = db.versionAssets(versionRoot)
	// 		ext.Versions = append(ext.Versions, version)
	// 	}
	// 	exts = append(exts, ext)
	// }
	// db.items = exts
	// db.sortVersions()

	// db.loadDuration = time.Since(start)
	// db.loadedAt = time.Now()
	// db.dblog.Debug().Msgf("loading database took %.3fs", db.loadDuration.Seconds())
	return nil
}

// extensionPath returns the asset path for a given ExtensionTag
func (db Database) extensionPath(uid vscode.UniqueID) string {
	return filepath.Join(db.root, uid.Publisher, uid.Name)
}

// assetPath returns the asset path for a given ExtensionTag
func (db Database) assetPath(tag vscode.ExtensionTag) string {
	return filepath.Join(db.extensionPath(tag.UniqueID), tag.Version, tag.TargetPlatform)
}

func (db Database) SaveExtensionMetadata(ext vscode.Extension) error {
	uid, ok := vscode.Parse(ext.UniqueID())
	if !ok {
		return fmt.Errorf("extension unique id %s is not valid", ext.ID)
	}
	// clear version information, this is saved per version later on
	ext.Versions = []vscode.Version{}
	if err := db.fs.MkdirAll(db.extensionPath(uid), 0755); err != nil {
		return err
	}
	fpath := filepath.Join(db.extensionPath(uid), extensionMetadataFilename)
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

func (db Database) SaveAsset(tag vscode.ExtensionTag, atype vscode.AssetTypeKey, r io.ReadCloser) error {
	p := db.assetPath(tag)
	if err := db.fs.MkdirAll(p, 0755); err != nil {
		return err
	}
	fpath := filepath.Join(p, string(atype))
	return afero.WriteReader(db.fs, fpath, r)
}
