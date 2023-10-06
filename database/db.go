package database

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/afero"
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
	fs            afero.Fs
}

type DBStats struct {
	ExtensionCount int
	VersionCount   int
	// time it took to load the database from disk
	LoadDuration time.Duration
}

func open(fs afero.Fs, root string, autoreload bool) (*DB, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	dblog := log.With().Str("component", "database").Str("path", absRoot).Logger()
	db := &DB{
		root:    absRoot,
		items:   []vscode.Extension{},
		modFile: path.Join(root, modFilename),
		dblog:   dblog,
		fs:      fs,
	}
	if err := db.load(); err != nil {
		return nil, err
	}

	if autoreload {
		err := db.autoreload()
		if err != nil {
			return db, err
		}
	}

	if db.Empty() {
		db.dblog.Info().Msgf("could not find any extensions at %v", root)
	} else {
		stats := db.Stats()
		db.dblog.Info().Msgf("database contains %v extensions with a total of %v versions", stats.ExtensionCount, stats.VersionCount)
	}

	return db, err
}

func OpenMem() (*DB, error) {
	return open(afero.NewMemMapFs(), "/data", false)
}

func OpenFs(root string, autoreload bool) (*DB, error) {
	return open(afero.NewOsFs(), root, autoreload)
}

// TODO remove later? only called from serve command
func (db *DB) Root() string {
	return db.root
}

func (db *DB) autoreload() (err error) {
	db.dblog.Debug().
		Str("modfile", db.modFile).
		Msg("checking if modfile exists")
	_, err = db.fs.Stat(db.modFile)
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
	f, err := db.fs.Create(path.Join(db.root, modFilename))
	if err != nil {
		return err
	}
	return f.Close()
}

func (db *DB) saveExtensionMetadata(e vscode.Extension) error {
	// save extension metadata
	// re-run query to populate statistics, list versions query does not populate statistics, is there another way?
	eqr, err := marketplace.QueryLatestVersionByUniqueID(e.UniqueID()).Run()
	if err != nil {
		return err
	}
	newExt := eqr.Results[0].Extensions[0]
	newExt.Versions = []vscode.Version{}
	if err := db.fs.MkdirAll(ExtensionDir(db.root, newExt), os.ModePerm); err != nil {
		return err
	}
	return afero.WriteFile(db.fs, ExtensionMetaFile(db.root, newExt), []byte(newExt.String()), os.ModePerm)
}

func (db *DB) saveVersionMetadata(e vscode.Extension, v vscode.Version) error {
	if err := db.fs.MkdirAll(VersionDir(db.root, e, v), os.ModePerm); err != nil {
		return err
	}
	if err := afero.WriteFile(db.fs, VersionMetaFile(db.root, e, v), []byte(v.String()), os.ModePerm); err != nil {
		return err
	}
	return nil
}

// rollback removes the entire version directory, called when sync fails
func (db *DB) Rollback(e vscode.Extension, v vscode.Version) error {
	elog := db.dblog.With().Str("extension", e.UniqueID()).Str("extension_version", v.Version).Str("extension_version_id", v.ID()).Logger()
	versionDir := VersionDir(db.root, e, v)
	elog.Warn().Str("version_dir", versionDir).Msg("removing version directory due to rollback")
	return db.fs.RemoveAll(versionDir)
}

func (db *DB) SaveExtensionMetadata(e vscode.Extension) error {
	elog := db.dblog.With().Str("extension", e.UniqueID()).Logger()
	elog.Debug().Msg("saving extension metadata file")
	return db.saveExtensionMetadata(e)
}

func (db *DB) SaveVersionMetadata(e vscode.Extension, v vscode.Version) error {
	elog := db.dblog.With().Str("extension", e.UniqueID()).Str("extension_version", v.Version).Str("extension_version_id", v.ID()).Logger()

	elog.Debug().Msg("saving version metadata file")
	if err := db.saveVersionMetadata(e, v); err != nil {
		elog.Err(err).Msg("failed to save version metadata file")
		err = db.Rollback(e, v)
		if err != nil {
			elog.Err(err).Msg("rollback failed")
		}
		return err
	}

	return nil
}

func (db *DB) SaveAssetFile(e vscode.Extension, v vscode.Version, a vscode.Asset, b []byte) error {
	elog := db.dblog.With().Str("extension", e.UniqueID()).Str("extension_version", v.Version).Str("extension_version_id", v.ID()).Logger()
	filename := AssetFile(db.root, e, v, a)
	elog.Info().Str("source", a.Source).Str("destination", filename).Msg("downloading")
	f, err := db.fs.Create(filename)
	if err != nil {
		elog.Err(err).Str("source", a.Source).Str("destination", filename).Msg("could not create file")
		err = db.Rollback(e, v)
		if err != nil {
			elog.Err(err).Msg("rollback failed")
		}
		return err
	}
	defer f.Close()
	if _, err := f.Write(b); err != nil {
		elog.Err(err).Str("source", a.Source).Str("destination", filename).Msg("could not save file")
		err = db.Rollback(e, v)
		if err != nil {
			elog.Err(err).Msg("rollback failed")
		}
		return err
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

// GetVersion returns true and the found version if the given extension and version can be found. It
// returns false if the extension can not be found.
func (db *DB) GetVersion(uniqueID string, version vscode.Version) (vscode.Version, bool) {
	exts := db.FindByUniqueID(false, uniqueID)
	// if the extensions could not be found the version does not exist
	if len(exts) == 0 {
		return vscode.Version{}, false
	}
	for _, v := range exts[0].Versions {
		if v.ID() == version.ID() {
			return v, true
		}
	}
	return vscode.Version{}, false
}

// FindByUniqueID returns an array of extensions matching a list of uniqueID's. If keepLatestVersion is true only the latest
// version is keep of all available version for returned extensions. When false all versions for an extension are included.
// This function ignores case.
func (db *DB) FindByUniqueID(keepLatestVersion bool, uniqueIDs ...string) []vscode.Extension {
	queryMap := map[string]bool{}

	for _, id := range uniqueIDs {
		queryMap[strings.ToLower(id)] = true
	}

	result := []vscode.Extension{}
	for _, i := range db.items {
		if queryMap[strings.ToLower(i.UniqueID())] {
			if keepLatestVersion {
				i = i.KeepVersions(i.LatestVersion(true))
			}
			result = append(result, i.Copy())
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
				i = i.KeepVersions(i.LatestVersion(true))
			}
			result = append(result, i.Copy())
		}
	}
	return result
}

func multiContains(s string, substrs ...string) bool {
	s = strings.ToLower(s)
	for _, substr := range substrs {
		substr = strings.ToLower(substr)
		splits := strings.Split(substr, " ")
		for _, split := range splits {
			if strings.Contains(s, split) {
				return true
			}
		}
	}
	return false
}

func (db *DB) Search(keepLatestVersion bool, text ...string) []vscode.Extension {
	result := []vscode.Extension{}
	for _, i := range db.items {
		if multiContains(i.Name, text...) || multiContains(i.DisplayName, text...) || multiContains(i.Publisher.Name, text...) || multiContains(i.ShortDescription, text...) {
			if keepLatestVersion {
				i = i.KeepVersions(i.LatestVersion(true))
			}
			result = append(result, i.Copy())
		}
	}
	return result
}

// Run will execute all aspects of a marketplace.Query against the database. This includes
// querying, sorting, limiting and paging.
func (db *DB) Run(q marketplace.Query) (vscode.Results, error) {
	res := vscode.NewResults()

	extensions := []vscode.Extension{}

	if !q.IsValid() {
		return res, marketplace.ErrInvalidQuery
	}

	if q.IsEmptyQuery() {
		// empty queries sorted by number of installs equates to a @popular query
		extensions = append(extensions, db.List(q.Flags.Is(marketplace.FlagIncludeLatestVersionOnly))...)
	} else {
		uniqueIDs := q.CriteriaValues(marketplace.FilterTypeExtensionName)
		if len(uniqueIDs) > 0 {
			db.dblog.Debug().Msgf("found array of extension names in query: %v", uniqueIDs)
			extensions = append(extensions, db.FindByUniqueID(q.Flags.Is(marketplace.FlagIncludeLatestVersionOnly), uniqueIDs...)...)
			db.dblog.Debug().Msgf("extension name database query found %v extension", len(extensions))
		}

		searchValues := q.CriteriaValues(marketplace.FilterTypeSearchText)
		if len(searchValues) > 0 {
			db.dblog.Debug().Msgf("found text searches in query: %v", searchValues)
			extensions = append(extensions, db.Search(q.Flags.Is(marketplace.FlagIncludeLatestVersionOnly), searchValues...)...)
			db.dblog.Debug().Msgf("free text database query found %v extension", len(extensions))
		}

		extIDs := q.CriteriaValues(marketplace.FilterTypeExtensionID)
		if len(extIDs) > 0 {
			db.dblog.Debug().Msgf("found array of extension identifiers in query: %v", extIDs)
			extensions = append(extensions, db.FindByExtensionID(q.Flags.Is(marketplace.FlagIncludeLatestVersionOnly), extIDs...)...)
			db.dblog.Debug().Msgf("extension identifier database query found %v extension", len(extensions))
		}
	}

	// set total count to all extensions found, before some might be removed if paginated
	res.SetTotalCount(len(extensions))

	// sort the result
	switch q.SortBy() {
	case marketplace.ByInstallCount:
		sort.Sort(vscode.ByPopularity(extensions))
	}

	// paginate
	begin, end := pageBoundaries(len(extensions), q.Filters[0].PageSize, q.Filters[0].PageNumber)

	// add sorted and paginated extensions to the result
	res.AddExtensions(extensions[begin:end])

	return res, nil
}

// pageBoundaries return the begin and end index for a given page size and page. Indices
// can be used when slicing arrays/slices.
func pageBoundaries(totalCount, pageSize, pageNumber int) (begin, end int) {
	if pageNumber < 1 {
		pageNumber = 1
	}
	begin = ((pageNumber - 1) * pageSize)
	end = begin + pageSize
	if end > totalCount {
		end = totalCount
	}
	return
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
func (db *DB) List(keepLatestVersion bool) []vscode.Extension {
	result := []vscode.Extension{}
	for _, e := range db.items {
		if keepLatestVersion {
			e = e.KeepVersions(e.LatestVersion(true))
		}
		result = append(result, e.Copy())
	}
	return result
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
		if len(versions) == 0 {
			db.dblog.Error().Str("path", extensionRoot).Msg("extension does not have any versions, skipping")
			continue
		}
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
	matches, _ := afero.Glob(afero.NewBasePathFs(db.fs, db.root), "*/*")
	// matches, _ := fs.Glob(os.DirFS(db.root), "*/*")
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
	matches, _ := afero.Glob(afero.NewBasePathFs(db.fs, ext.Path), "*/*")
	// matches, _ := fs.Glob(os.DirFS(ext.Path), "*/*")
	versions := []vscode.Version{}
	for _, m := range matches {
		versionRoot := path.Join(ext.Path, m)
		fi, err := db.fs.Stat(versionRoot)
		if err != nil {
			db.dblog.Error().Err(err).Str("path", ext.Path).Msg("error while loading version, skipping")
			continue
		}
		if fi.IsDir() {
			metafile := path.Join(versionRoot, versionMetadataFileName)
			db.dblog.Debug().Str("path", metafile).Str("file", m).Msg("found version candidate")
			v := vscode.Version{}
			b, err := afero.ReadFile(db.fs, metafile)
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
	b, err := afero.ReadFile(db.fs, metaFile)
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

func (db *DB) DeleteVersion(e vscode.Extension, v vscode.Version) error {
	db.dblog.Info().Str("extension", e.UniqueID()).Str("version", v.Version).Msg("removing version")
	return os.RemoveAll(path.Dir(v.Path))
}
