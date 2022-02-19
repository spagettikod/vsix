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
	Root          string
	items         []vscode.Extension
	assetEndpoint string
	loadDuration  time.Duration
	loadedAt      time.Time
	modFile       string
	watcher       *fsnotify.Watcher
}

type DBStats struct {
	ExtensionCount int
	VersionCount   int
	// time it took to load the database from disk
	LoadDuration time.Duration
}

func New(root, assetEndpoint string) (*DB, error) {
	db := &DB{
		Root:          root,
		items:         []vscode.Extension{},
		assetEndpoint: assetEndpoint,
		modFile:       path.Join(root, modFilename),
	}
	err := db.load()
	if err != nil {
		return nil, err
	}

	err = db.autoreload()
	if err != nil {
		return nil, err
	}

	if db.Empty() {
		log.Info().Msgf("could not find any extensions at %v", root)
	} else {
		stats := db.Stats()
		log.Info().Msgf("serving %v extensions with a total of %v versions", stats.ExtensionCount, stats.VersionCount)
	}

	return db, err
}

func (db *DB) autoreload() (err error) {
	log.Debug().
		Str("path", db.Root).
		Str("modfile", db.modFile).
		Msg("checking if modfile exists")
	_, err = os.Stat(db.modFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Debug().
				Str("path", db.Root).
				Str("modfile", db.modFile).
				Msg("modfile not found, creating one")
			err = Modified(db.Root)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	log.Debug().
		Str("path", db.Root).
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
			log.Info().
				Str("path", db.Root).
				Str("modfile", db.modFile).
				Msg("database has been modified, reloading")
			err := db.Reload()
			if err != nil {
				log.Fatal().
					Err(err).
					Str("path", db.Root).
					Str("modfile", db.modFile).
					Msg("error while reloading database, exiting")
			}
		}
	}()
	go func() {
		err := <-db.watcher.Errors
		log.Error().
			Err(err).
			Str("path", db.Root).
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
		log.Info().Str("path", db.Root).Msgf("could not find any extensions")
	} else {
		stats := db.Stats()
		log.Info().Msgf("serving %v extensions with a total of %v versions", stats.ExtensionCount, stats.VersionCount)
	}

	return nil
}

// Modified notifies the database at path p that its content has been updated.
func Modified(p string) error {
	f, err := os.Create(path.Join(p, modFilename))
	if err != nil {
		return err
	}
	return f.Close()
}

// Inconsistent returns true if the database in memory is inconsistent with files on disk.
// Currently it only checks if version folders have been modified since the last database load.
func (db *DB) Inconsistent() (bool, error) {
	log.Debug().Str("path", db.modFile).Msg("checking modfile to see if database has been updated")
	fi, err := os.Stat(db.modFile)
	if err != nil {
		return false, err
	}
	log.Debug().Str("path", db.modFile).Msgf("is modfile updated? %v", db.loadedAt.Before(fi.ModTime()))
	return db.loadedAt.Before(fi.ModTime()), nil
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
	absRoot, err := filepath.Abs(db.Root)
	if err != nil {
		return err
	}
	log.Debug().Str("path", absRoot).Msg("loading database with extensions")
	exts := []vscode.Extension{}
	for _, extensionRoot := range listExtensions(absRoot) {
		log.Debug().Str("path", extensionRoot).Msg("loading extension")
		ext, err := loadExtension(extensionRoot)
		if err != nil {
			log.Error().Err(err).Str("path", extensionRoot).Msg("error while loading extension, skipping")
			continue
		}
		versions := listVersions(extensionRoot)
		for _, version := range versions {
			versionRoot := ext.AbsVersionDir(absRoot, version.Version)
			log.Debug().Str("path", versionRoot).Msg("loading version")
			version.Path = versionRoot
			version.AssetURI = db.assetEndpoint + ext.AbsVersionDir("", version.Version)
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
	log.Debug().Msgf("loading database took %.3fs", db.loadDuration.Seconds())
	return nil
}

// listExtensions returns a list of paths to extension in directory root. Valid extensions have a file named metadata.json at <root>/extensions/*/*/metadata.json.
func listExtensions(root string) []string {
	log.Debug().Str("path", root).Msg("searching for extension")
	matches, _ := fs.Glob(os.DirFS(root), "*/*")
	files := []string{}
	for _, m := range matches {
		log.Debug().Str("path", m).Msg("found extension candidate")
		files = append(files, path.Join(root, m))
	}
	return files
}

func listVersions(extensionRoot string) []vscode.Version {
	log.Debug().Str("path", extensionRoot).Msg("list extension versions")
	matches, _ := fs.Glob(os.DirFS(extensionRoot), "*")
	versions := []vscode.Version{}
	for _, m := range matches {
		versionRoot := path.Join(extensionRoot, m)
		fi, err := os.Stat(versionRoot)
		if err != nil {
			log.Error().Err(err).Str("path", extensionRoot).Msg("error while loading version, skipping")
			continue
		}
		if fi.IsDir() {
			log.Debug().Str("path", vscode.AbsVersionMetadataFile(versionRoot)).Str("file", m).Msg("found version candidate")
			v := vscode.Version{}
			b, err := ioutil.ReadFile(vscode.AbsVersionMetadataFile(versionRoot))
			if err != nil {
				log.Error().Err(err).Str("path", extensionRoot).Msg("error while loading version, skipping")
				continue
			}
			if err := json.Unmarshal(b, &v); err != nil {
				log.Error().Err(err).Str("path", extensionRoot).Msg("error while loading version, skipping")
				continue
			}
			versions = append(versions, v)
		} else {
			log.Debug().Str("file", m).Msg("not a directory, skipping")
		}
	}
	return versions
}

func loadExtension(extensionRoot string) (vscode.Extension, error) {
	metaFile := path.Join(extensionRoot, vscode.ExtensionMetadataFileName)
	log.Debug().Str("path", metaFile).Msg("loading metadata")
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

func listAssets(versionRoot string) []string {
	log.Debug().Str("path", versionRoot).Msg("looking for version assets")
	matches, _ := fs.Glob(os.DirFS(versionRoot), "*")
	files := []string{}
	for _, m := range matches {
		if m == "_vsix_db_version_metadata.json" {
			continue
		}
		log.Debug().Str("path", versionRoot).Str("asset", m).Msg("found asset")
		files = append(files, path.Join(versionRoot, m))
	}
	return files
}

func (db *DB) versionAssets(versionRoot string) []vscode.Asset {
	assets := []vscode.Asset{}
	for _, a := range listAssets(versionRoot) {
		pathelements := strings.Split(filepath.Dir(a), "/")
		log.Debug().Str("path", versionRoot).Str("asset", a).Msg("adding asset to extension")
		asset := vscode.Asset{
			Type:   vscode.AssetTypeKey(filepath.Base(a)),
			Source: db.assetEndpoint + path.Join(pathelements[len(pathelements)-3], pathelements[len(pathelements)-2], pathelements[len(pathelements)-1], filepath.Base(a)),
			Path:   a,
		}
		assets = append(assets, asset)
	}
	return assets
}
