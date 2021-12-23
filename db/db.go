package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/vscode"
	"golang.org/x/mod/semver"
)

var (
	ErrNotFound error = errors.New("query returned no results")
)

type DB struct {
	Root          string
	items         []vscode.Extension
	assetEndpoint string
}

type DBStats struct {
	ExtensionCount int
	VersionCount   int
}

func New(root, assetEndpoint string) (*DB, error) {
	db := &DB{
		Root:          root,
		items:         []vscode.Extension{},
		assetEndpoint: assetEndpoint,
	}
	err := db.load()

	if db.Empty() {
		log.Info().Msgf("could not find any extensions at %v", root)
	} else {
		stats := db.Stats()
		log.Info().Msgf("serving %v extensions with a total of %v versions", stats.ExtensionCount, stats.VersionCount)
	}

	return db, err
}

func (db *DB) Reload() error {
	ndb, err := New(db.Root, db.assetEndpoint)
	if err != nil {
		return err
	}
	db.items = ndb.items
	return nil
}

// func (db *DB) Get(uniqueID string) (vscode.Extension, bool) {
// 	for _, i := range db.items {
// 		if i.UniqueID() == uniqueID {
// 			log.Println(i)
// 			return i, true
// 		}
// 	}
// 	return vscode.Extension{}, false
// }

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
	stats := DBStats{}
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
			fmt.Println(err)
			os.Exit(1)
		}
		versions, err := listVersions(extensionRoot)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		for _, version := range versions {
			versionRoot := ext.AbsVersionDir(absRoot, version.Version)
			log.Debug().Str("path", versionRoot).Msg("loading version")
			version.Path = versionRoot
			version.AssetURI = db.assetEndpoint + ext.Publisher.Name + "/" + ext.Name + "/" + version.Version
			version.FallbackAssetURI = version.AssetURI
			version.Files = db.versionAssets(versionRoot)
			ext.Versions = append(ext.Versions, version)
		}
		exts = append(exts, ext)
	}
	db.items = exts
	db.sortVersions()
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

func listVersions(extensionRoot string) ([]vscode.Version, error) {
	log.Debug().Str("path", extensionRoot).Msg("list extension versions")
	matches, _ := fs.Glob(os.DirFS(extensionRoot), "*")
	versions := []vscode.Version{}
	for _, m := range matches {
		versionRoot := path.Join(extensionRoot, m)
		fi, err := os.Stat(versionRoot)
		if err != nil {
			return versions, err
		}
		if fi.IsDir() {
			log.Debug().Str("path", vscode.AbsVersionMetadataFile(versionRoot)).Str("file", m).Msg("found version candidate")
			v := vscode.Version{}
			b, err := ioutil.ReadFile(vscode.AbsVersionMetadataFile(versionRoot))
			if err != nil {
				return versions, err
			}
			if err := json.Unmarshal(b, &v); err != nil {
				return versions, err
			}
			versions = append(versions, v)
		} else {
			log.Debug().Str("file", m).Msg("not a directory, skipping")
		}
		// FIXME remove
		// if !strings.Contains(m, "metadata.json") {
		// 	debug.Printf("found metadata.json\n")
		// 	files = append(files, path.Join(extensionRoot, m))
		// } else {
		// 	debug.Printf("could not find metadata.json, probably not version directory, skipping\n")
		// }
	}
	return versions, nil
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
