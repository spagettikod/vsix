package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/afero"
	"golang.org/x/mod/semver"
)

const (
	extensionMetadataFilename = "_vsix_db_extension_metadata.json"
	versionMetadataFilename   = "_vsix_db_version_metadata.json"
)

var (
	ErrExtensionMetadataNotFound = errors.New("extension metadata missing")
	ErrVersionMetadataNotFound   = errors.New("version metadata missing")
	ErrMissingAsset              = errors.New("asset not found")
	ErrNotFound                  = errors.New("object not found")
)

type ValidationError struct {
	Tag   vscode.VersionTag
	Error error
}

type Database struct {
	items *sync.Map
	fs    afero.Fs
}

func Open(root string) (*Database, []ValidationError, error) {
	osFs, err := newOsFs(root)
	if err != nil {
		return nil, []ValidationError{}, err
	}
	return open(osFs)
}

func New(root string) (*Database, error) {
	osFs, err := newOsFs(root)
	if err != nil {
		return nil, err
	}
	return new(osFs)
}

func newOsFs(root string) (afero.Fs, error) {
	if exists, err := afero.DirExists(afero.NewOsFs(), root); !exists || err != nil {
		if !exists {
			return nil, fmt.Errorf("database path not found")
		}
		return nil, err
	}
	return afero.NewBasePathFs(afero.NewOsFs(), root), nil
}

func new(fs afero.Fs) (*Database, error) {
	return &Database{
		items: &sync.Map{},
		fs:    fs,
	}, nil
}

func open(fs afero.Fs) (*Database, []ValidationError, error) {
	db, err := new(fs)
	if err != nil {
		return nil, []ValidationError{}, err
	}
	verrs, err := db.fsIndex()
	if err != nil {
		return nil, []ValidationError{}, err
	}

	return db, verrs, err
}

func (db Database) List() []vscode.Extension {
	exts := []vscode.Extension{}
	db.items.Range(func(key, value any) bool {
		exts = append(exts, value.(vscode.Extension))
		return true
	})
	return exts
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

func (db Database) SaveExtensionMetadata(ext vscode.Extension) error {
	// clear version information, this is saved per version later on
	ext.Versions = []vscode.Version{}
	if err := db.fs.MkdirAll(extensionPath(ext.UniqueID()), 0755); err != nil {
		return err
	}
	fpath := filepath.Join(extensionPath(ext.UniqueID()), extensionMetadataFilename)
	return afero.WriteFile(db.fs, fpath, []byte(ext.String()), os.ModePerm)
}

func (db Database) SaveVersionMetadata(uid vscode.UniqueID, v vscode.Version) error {
	p := assetPath(v.Tag(uid))
	if err := db.fs.MkdirAll(p, 0755); err != nil {
		return err
	}
	fpath := filepath.Join(p, versionMetadataFilename)
	return afero.WriteFile(db.fs, fpath, []byte(v.String()), os.ModePerm)
}

func (db Database) SaveAsset(tag vscode.VersionTag, atype vscode.AssetTypeKey, r io.ReadCloser) error {
	p := assetPath(tag)
	if err := db.fs.MkdirAll(p, 0755); err != nil {
		return err
	}
	fpath := filepath.Join(p, string(atype))
	return afero.WriteReader(db.fs, fpath, r)
}

func (db Database) LoadAsset(tag vscode.VersionTag, assetType vscode.AssetTypeKey) (io.ReadCloser, error) {
	return db.fs.Open(filepath.Join(assetPath(tag), string(assetType)))
}

func (db Database) DetectAssetContentType(tag vscode.VersionTag, assetType vscode.AssetTypeKey) (string, error) {
	file, err := db.fs.Open(filepath.Join(assetPath(tag), string(assetType)))
	if err != nil {
		return "", err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return "", err
	}

	return http.DetectContentType(buffer), nil
}

// Run will execute all aspects of a marketplace.Query against the database. This includes
// querying, sorting, limiting and paging.
func (db Database) Run(q marketplace.Query) (vscode.Results, error) {
	res := vscode.NewResults()

	extensions := []vscode.Extension{}

	if !q.IsValid() {
		return res, marketplace.ErrInvalidQuery
	}

	if q.IsEmptyQuery() {
		// empty queries sorted by number of installs equates to a @popular query
		extensions = append(extensions, db.List()...)
	} else {
		uniqueIDs := q.CriteriaValues(marketplace.FilterTypeExtensionName)
		if len(uniqueIDs) > 0 {
			for _, uidstr := range uniqueIDs {
				uid, ok := vscode.Parse(uidstr)
				if !ok {
					return res, fmt.Errorf("invalid unique id in query %s", uidstr)
				}
				if ext, found := db.FindByUniqueID(uid); found {
					extensions = append(extensions, ext)
				}
			}
		}

		searchValues := q.CriteriaValues(marketplace.FilterTypeSearchText)
		if len(searchValues) > 0 {
			for _, e := range db.List() {
				if e.MatchesQuery(searchValues...) {
					extensions = append(extensions, e)
				}
			}
		}
	}

	// set total count to all extensions found, before some might be removed if paginated
	res.SetTotalCount(len(extensions))

	// sort the result
	switch q.SortBy() {
	case marketplace.ByInstallCount:
		slices.SortFunc(extensions, vscode.SortFuncExtensionByInstallCount)
	case marketplace.ByName:
		slices.SortFunc(extensions, vscode.SortFuncExtensionByDisplayName)
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
	paths, err := afero.Glob(db.fs, filepath.Join(extensionPath(uid), "*/**"))
	if err != nil {
		return tags, err
	}
	for _, p := range paths {
		vpath, err := filepath.Rel(extensionPath(uid), p)
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

func (db Database) fsIndex() ([]ValidationError, error) {
	verrs := []ValidationError{}
	verrsMux := &sync.Mutex{}

	uids, err := db.fsListUniqueID()
	if err != nil {
		return verrs, err
	}

	const maxGoroutines = 10 // limit to 10 concurrent extensions
	sem := make(chan struct{}, maxGoroutines)
	wg := sync.WaitGroup{}
	var indexingError error
	for _, uid := range uids {
		if indexingError != nil {
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{} // block if the semaphore is full
			defer func() { <-sem }()

			ext, err := db.fsLoadExtensionMetadata(uid)
			if err != nil {
				if errors.Is(err, ErrExtensionMetadataNotFound) { // unexpected error exits the function
					tag := vscode.VersionTag{
						UniqueID: uid,
					}
					verrsMux.Lock()
					defer verrsMux.Unlock()
					verrs = append(verrs, ValidationError{Tag: tag, Error: err})
					return // skip loading version metadata if extension fails to load
				}
				indexingError = err
				return
			}

			tags, err := db.fsListVersionTags(uid)
			if err != nil {
				indexingError = err
				return
			}

			for _, tag := range tags {
				v, err := db.fsLoadVersionMetadata(tag)
				if err != nil {
					if errors.Is(err, ErrVersionMetadataNotFound) {
						verrs = append(verrs, ValidationError{Tag: tag, Error: err})
						continue
					}
					indexingError = err
					return
				}
				if err := db.ValidateVersion(tag); err != nil {
					verrs = append(verrs, ValidationError{Tag: tag, Error: err})
				}

				ext.Versions = append(ext.Versions, v)
			}
			slices.SortFunc(ext.Versions, func(v1, v2 vscode.Version) int {
				return semver.Compare("v"+v1.Version, "v"+v2.Version) * -1
			})

			db.items.Store(uid, ext)
		}()
	}
	wg.Wait()
	return verrs, indexingError
}

func (db Database) fsLoadVersionMetadata(tag vscode.VersionTag) (vscode.Version, error) {
	vmeta := vscode.Version{}
	bites, err := afero.ReadFile(db.fs, filepath.Join(assetPath(tag), versionMetadataFilename))
	if err != nil {
		if errors.Is(err, afero.ErrFileNotFound) {
			return vscode.Version{}, ErrVersionMetadataNotFound
		}
		return vscode.Version{}, err
	}
	return vmeta, json.Unmarshal(bites, &vmeta)
}

// ValidateVersion check if all assets for the given version exist in the database.
func (db Database) ValidateVersion(tag vscode.VersionTag) error {
	vmeta, err := db.fsLoadVersionMetadata(tag)
	if err != nil {
		return err
	}
	for _, asset := range vmeta.Files {
		found, err := afero.Exists(db.fs, filepath.Join(assetPath(tag), string(asset.Type)))
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("%w: %s", ErrMissingAsset, asset.Type)
		}
	}
	return nil
}

func (db Database) fsLoadExtensionMetadata(uid vscode.UniqueID) (vscode.Extension, error) {
	ext := vscode.Extension{}
	s, err := afero.Glob(db.fs, filepath.Join(extensionPath(uid), extensionMetadataFilename))
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
func extensionPath(uid vscode.UniqueID) string {
	return filepath.Join(uid.Publisher, uid.Name)
}

// assetPath returns the asset path for a given ExtensionTag. For example: redhat/java/1.23.3/darwin-arm64.
func assetPath(tag vscode.VersionTag) string {
	return filepath.Join(extensionPath(tag.UniqueID), tag.Version, tag.TargetPlatform)
}

// Remove will remove the item the given tag points to. If the tag contain all parts the target platform for a version is removed.
// If the tag can not be found it return ErrNotFound.
func (db Database) Remove(tag vscode.VersionTag) error {
	if !tag.Validate() {
		return fmt.Errorf("invalid tag")
	}

	path := ""
	if tag.TargetPlatform != "" { // target platform has a value, remove just the target platform
		path = assetPath(tag)
	} else if tag.Version != "" { // target platform is empty but version isn't, remove entire version, regardless of target platform
		path = filepath.Join(extensionPath(tag.UniqueID), tag.Version)
	} else { // both target platform and version is empty, remove the entire extension
		path = extensionPath(tag.UniqueID)
	}

	return db.fs.RemoveAll(path)
}
