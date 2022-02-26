package db

import (
	"encoding/json"
	"errors"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/vscode"
	"golang.org/x/mod/semver"
)

const (
	modFilename string = ".modfile"
)

var (
	ErrNotFound error = errors.New("query returned no results")
)

type DB struct {
	root          string
	items         []vscode.Extension
	assetEndpoint string
	loadDuration  time.Duration
	loadedAt      time.Time
	modFile       string
	watcher       *fsnotify.Watcher
	dblog         zerolog.Logger
}

type DBStats struct {
	ExtensionCount int
	VersionCount   int
	// time it took to load the database from disk
	LoadDuration time.Duration
}

func Open(root string) (*DB, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	dblog := log.With().Str("component", "database").Str("db", absRoot).Logger()
	db := &DB{
		root:    absRoot,
		items:   []vscode.Extension{},
		modFile: path.Join(root, modFilename),
		dblog:   dblog,
	}
	if err := db.load(); err != nil {
		return nil, err
	}

	if db.Empty() {
		db.dblog.Info().Msgf("could not find any extensions at %v", root)
	} else {
		stats := db.Stats()
		db.dblog.Info().Msgf("database contains %v extensions with a total of %v versions", stats.ExtensionCount, stats.VersionCount)
	}

	return db, err
}

func (db *DB) SetAssetEndpoint(assetEndpoint string) {
	for _, e := range db.items {
		for _, v := range e.Versions {
			for i, f := range v.Files {
				v.Files[i].Source = assetEndpoint + f.Source
			}
		}
	}
}

// TODO remove later? only called from serve command
func (db *DB) Root() string {
	return db.root
}

func (db *DB) autoreload() (err error) {
	db.dblog.Debug().
		Str("modfile", db.modFile).
		Msg("checking if modfile exists")
	_, err = os.Stat(db.modFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			db.dblog.Debug().
				Str("modfile", db.modFile).
				Msg("modfile not found, creating one")
			err = db.Modified()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	db.dblog.Debug().
		Str("modfile", db.modFile).
		Msg("adding watcher to modfile")
	db.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	db.watcher.Add(db.modFile)
	go func() {
		for {
			<-db.watcher.Events
			db.dblog.Info().
				Str("modfile", db.modFile).
				Msg("database has been modified, reloading")
			err := db.Reload()
			if err != nil {
				db.dblog.Fatal().
					Err(err).
					Str("modfile", db.modFile).
					Msg("error while reloading database, exiting")
			}
		}
	}()
	go func() {
		err := <-db.watcher.Errors
		db.dblog.Error().
			Err(err).
			Str("modfile", db.modFile).
			Msg("error occured while monitoring .modfile, exiting")
	}()
	return nil
}

func (db *DB) Reload() error {
	db.items = []vscode.Extension{}
	err := db.load()
	if err != nil {
		return err
	}

	if db.Empty() {
		db.dblog.Info().Msgf("could not find any extensions")
	} else {
		stats := db.Stats()
		db.dblog.Info().Msgf("serving %v extensions with a total of %v versions", stats.ExtensionCount, stats.VersionCount)
	}

	return nil
}

// Modified notifies the database at path p that its content has been updated.
func (db *DB) Modified() error {
	f, err := os.Create(path.Join(db.root, modFilename))
	if err != nil {
		return err
	}
	return f.Close()
}

func (db *DB) saveExtensionMetadata(e vscode.Extension) error {
	// save extension metadata
	// re-run query to populate statistics, list versions query does not populate statistics, is there another way?
	eqr, err := vscode.RunQuery(vscode.LatestQueryJSON(e.UniqueID()))
	if err != nil {
		return err
	}
	newExt := eqr.Results[0].Extensions[0]
	newExt.Versions = []vscode.Version{}
	if err := os.MkdirAll(ExtensionDir(db.root, newExt), os.ModePerm); err != nil {
		return err
	}
	return ioutil.WriteFile(ExtensionMetaFile(db.root, newExt), []byte(newExt.String()), os.ModePerm)
}

func (db *DB) saveVersionMetadata(e vscode.Extension, v vscode.Version) error {
	if err := os.MkdirAll(VersionDir(db.root, e, v), os.ModePerm); err != nil {
		return err
	}
	if err := ioutil.WriteFile(VersionMetaFile(db.root, e, v), []byte(v.String()), os.ModePerm); err != nil {
		return err
	}
	return nil
}

// rollback removes the entire version directory, called when sync fails
func (db *DB) rollback(e vscode.Extension, v vscode.Version) error {
	elog := db.dblog.With().Str("extension", e.UniqueID()).Str("extension_version", v.Version).Str("extension_version_id", v.ID()).Logger()
	versionDir := VersionDir(db.root, e, v)
	elog.Warn().Str("version_dir", versionDir).Msg("removing version directory due to rollback")
	return os.RemoveAll(versionDir)
}

func (db *DB) SaveExtension(e vscode.Extension) error {
	elog := db.dblog.With().Str("extension", e.UniqueID()).Logger()
	elog.Debug().Msg("saving extension metadata file")
	return db.saveExtensionMetadata(e)
}

func (db *DB) SaveVersion(e vscode.Extension, v vscode.Version) error {
	elog := db.dblog.With().Str("extension", e.UniqueID()).Str("extension_version", v.Version).Str("extension_version_id", v.ID()).Logger()

	elog.Debug().Msg("saving version metadata file")
	if err := db.saveVersionMetadata(e, v); err != nil {
		elog.Err(err).Msg("failed to save version metadata file")
		err = db.rollback(e, v)
		if err != nil {
			elog.Err(err).Msg("rollback failed")
		}
		return err
	}

	for _, a := range v.Files {
		filename := AssetFile(db.root, e, v, a)
		elog.Info().Str("source", a.Source).Str("destination", filename).Msg("downloading")
		if err := a.Download(filename); err != nil {
			elog.Err(err).Str("source", a.Source).Str("destination", filename).Msg("download failed")
			err = db.rollback(e, v)
			if err != nil {
				elog.Err(err).Msg("rollback failed")
			}
			return err
		}
	}

	return nil
}

// VersionExists returns true if the given extension and version can be found. It
// returns false if the extension can not be found.
func (db *DB) VersionExists(uniqueID string, version vscode.Version) bool {
	exts := db.FindByUniqueID(false, uniqueID)
	// if the extensions could not be found the version does not exist
	if len(exts) == 0 {
		return false
	}
	for _, v := range exts[0].Versions {
		if v.Equals(version) {
			return true
		}
	}
	return false
}

// FindByUniqueID returns an array of extensions matching a list of uniqueID's. If keepLatestVersion is true only the latest
// version is keep of all available version for returned extensions. When false all versions for an extension are included.
func (db *DB) FindByUniqueID(keepLatestVersion bool, uniqueIDs ...string) []vscode.Extension {
	queryMap := map[string]bool{}

	for _, id := range uniqueIDs {
		queryMap[id] = true
	}

	result := []vscode.Extension{}
	for _, i := range db.items {
		if queryMap[i.UniqueID()] {
			if keepLatestVersion {
				i = i.KeepVersions(i.LatestVersion())
			}
			result = append(result, i)
		}
	}
	return result
}

func (db *DB) FindByExtensionID(keepLatestVersion bool, ids ...string) []vscode.Extension {
	queryMap := map[string]bool{}

	for _, id := range ids {
		queryMap[id] = true
	}

	result := []vscode.Extension{}
	for _, i := range db.items {
		if queryMap[i.ID] {
			if keepLatestVersion {
				i = i.KeepVersions(i.LatestVersion())
			}
			result = append(result, i)
		}
	}
	return result
}

func multiContains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func (db *DB) Search(keepLatestVersion bool, text ...string) []vscode.Extension {
	result := []vscode.Extension{}
	for _, i := range db.items {
		if multiContains(i.Name, text...) || multiContains(i.DisplayName, text...) || multiContains(i.Publisher.Name, text...) || multiContains(i.ShortDescription, text...) {
			if keepLatestVersion {
				i = i.KeepVersions(i.LatestVersion())
			}
			result = append(result, i)
		}
	}
	return result
}

// String dumps the entire database as a JSON string.
func (db *DB) String() string {
	b, err := json.MarshalIndent(db.items, "", "   ")
	if err != nil {
		return "! JSON UNMARSHAL FAILED !"
	}
	return string(b)
}

// Empty returns true if the database has no entries, otherwise false.
func (db *DB) Empty() bool {
	return len(db.items) == 0
}

// List return all entries in the database.
func (db *DB) List() []vscode.Extension {
	return db.items
}

// Stats return some statistics about the database.
func (db *DB) Stats() DBStats {
	stats := DBStats{
		LoadDuration: db.loadDuration,
	}
	stats.ExtensionCount = len(db.items)
	for _, i := range db.items {
		stats.VersionCount += len(i.Versions)
	}
	return stats
}

func (db *DB) sortVersions() {
	for _, item := range db.items {
		sort.Slice(item.Versions, func(i, j int) bool {
			return semver.Compare("v"+item.Versions[i].Version, "v"+item.Versions[j].Version) > 0
		})
	}
}

func (db *DB) load() error {
	start := time.Now()
	db.dblog.Debug().Msg("loading database")
	exts := []vscode.Extension{}
	for _, extensionRoot := range db.listExtensions() {
		db.dblog.Debug().Str("path", extensionRoot).Msg("loading extension")
		ext, err := db.loadExtension(extensionRoot)
		if err != nil {
			db.dblog.Error().Err(err).Str("path", extensionRoot).Msg("error while loading extension, skipping")
			continue
		}
		versions := db.listVersions(ext)
		for _, version := range versions {
			versionRoot := VersionDir(db.root, ext, version)
			db.dblog.Debug().Str("path", versionRoot).Msg("loading version")
			version.Path = versionRoot
			version.AssetURI = db.assetEndpoint + VersionDir("", ext, version)
			version.FallbackAssetURI = version.AssetURI
			version.Files = db.versionAssets(versionRoot)
			ext.Versions = append(ext.Versions, version)
		}
		exts = append(exts, ext)
	}
	db.items = exts
	db.sortVersions()

	db.loadDuration = time.Since(start)
	db.loadedAt = time.Now()
	db.dblog.Debug().Msgf("loading database took %.3fs", db.loadDuration.Seconds())
	return nil
}

// listExtensions returns a list of paths to extension in directory root. Valid extensions have a file named metadata.json at <root>/extensions/*/*/metadata.json.
func (db *DB) listExtensions() []string {
	db.dblog.Debug().Msg("searching for extensions")
	matches, _ := fs.Glob(os.DirFS(db.root), "*/*")
	files := []string{}
	for _, m := range matches {
		p := path.Join(db.root, m)
		db.dblog.Debug().Str("path", p).Msg("found extension candidate")
		files = append(files, p)
	}
	return files
}

func (db *DB) listVersions(ext vscode.Extension) []vscode.Version {
	db.dblog.Debug().Str("path", ext.Path).Msg("list extension versions")
	matches, _ := fs.Glob(os.DirFS(ext.Path), "*/*")
	versions := []vscode.Version{}
	for _, m := range matches {
		versionRoot := path.Join(ext.Path, m)
		fi, err := os.Stat(versionRoot)
		if err != nil {
			db.dblog.Error().Err(err).Str("path", ext.Path).Msg("error while loading version, skipping")
			continue
		}
		if fi.IsDir() {
			metafile := path.Join(versionRoot, versionMetadataFileName)
			db.dblog.Debug().Str("path", metafile).Str("file", m).Msg("found version candidate")
			v := vscode.Version{}
			b, err := ioutil.ReadFile(metafile)
			if err != nil {
				db.dblog.Error().Err(err).Str("path", ext.Path).Msg("error while loading version, skipping")
				continue
			}
			if err := json.Unmarshal(b, &v); err != nil {
				db.dblog.Error().Err(err).Str("path", ext.Path).Msg("error while loading version, skipping")
				continue
			}
			versions = append(versions, v)
		} else {
			db.dblog.Debug().Str("file", m).Msg("not a directory, skipping")
		}
	}
	return versions
}

func (db *DB) loadExtension(extensionRoot string) (vscode.Extension, error) {
	metaFile := path.Join(extensionRoot, extensionMetadataFileName)
	db.dblog.Debug().Str("path", metaFile).Msg("loading metadata")
	b, err := ioutil.ReadFile(metaFile)
	if err != nil {
		return vscode.Extension{}, err
	}
	ext := vscode.Extension{}
	err = json.Unmarshal(b, &ext)
	if err != nil {
		return vscode.Extension{}, err
	}
	ext.Path = extensionRoot
	return ext, err
}

func (db *DB) listAssets(versionRoot string) []string {
	db.dblog.Debug().Str("path", versionRoot).Msg("looking for version assets")
	matches, _ := fs.Glob(os.DirFS(versionRoot), "*")
	files := []string{}
	for _, m := range matches {
		if m == "_vsix_db_version_metadata.json" {
			continue
		}
		db.dblog.Debug().Str("path", versionRoot).Str("asset", m).Msg("found asset")
		files = append(files, path.Join(versionRoot, m))
	}
	return files
}

func (db *DB) versionAssets(versionRoot string) []vscode.Asset {
	assets := []vscode.Asset{}
	for _, a := range db.listAssets(versionRoot) {
		pathelements := strings.Split(filepath.Dir(a), "/")
		db.dblog.Debug().
			Str("path", versionRoot).
			Str("asset", a).
			Str("asset_type", string(vscode.AssetTypeKey(filepath.Base(a)))).Msg("adding asset to extension")
		asset := vscode.Asset{
			Type:   vscode.AssetTypeKey(filepath.Base(a)),
			Source: db.assetEndpoint + path.Join(pathelements[len(pathelements)-4], pathelements[len(pathelements)-3], pathelements[len(pathelements)-2], pathelements[len(pathelements)-1], filepath.Base(a)),
			Path:   a,
		}
		assets = append(assets, asset)
	}
	return assets
}
