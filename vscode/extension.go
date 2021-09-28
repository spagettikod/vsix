package vscode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

const (
	propKeyExtensionPack = "Microsoft.VisualStudio.Code.ExtensionPack"
	debugEnvVar          = "VSIX_DEBUG"
)

var (
	ErrExtensionNotFound      = errors.New("extension could not be found at Marketplace")
	ErrExtensionHasNoVersions = errors.New("extension has no versions")
)

type extensionQueryResponse struct {
	Results []struct {
		Extensions []Extension `json:"extensions"`
	} `json:"results"`
}

type Extension struct {
	Publisher struct {
		ID          string `json:"publisherId"`
		Name        string `json:"publisherName"`
		DisplayName string `json:"displayName"`
	} `json:"publisher"`
	Name             string    `json:"extensionName"`
	ID               string    `json:"extensionID"`
	DisplayName      string    `json:"displayName"`
	ShortDescription string    `json:"shortDescription"`
	ReleaseDate      time.Time `json:"releaseDate"`
	LastUpdated      time.Time `json:"lastUpdated"`
	Versions         []Version `json:"versions,omitempty"`
	Statistics       []struct {
		Name  string  `json:"statisticName"`
		Value float32 `json:"value"`
	} `json:"statistics"`
}

func BuildDatabase(root string) ([]Extension, error) {
	root = path.Join(root, "extensions")
	rootFS := os.DirFS(root)
	versions := []Version{}
	exts := []Extension{}
	ver := Version{}
	extName := ""
	err := fs.WalkDir(rootFS, ".", func(p string, d fs.DirEntry, err error) error {
		fmt.Println(p)
		// example: p == golang/go/metadata.json
		if p == path.Join(extName, "metadata.json") {
			b, err := ioutil.ReadFile(path.Join(root, p))
			if err != nil {
				return err
			}
			ext := Extension{}
			err = json.Unmarshal(b, &ext)
			if err != nil {
				return err
			}
			versions = append(versions, ver)
			ext.Versions = versions
			exts = append(exts, ext)
			return nil
		}
		// example: p == golang/go
		if strings.Count(p, "/") == 1 {
			extName = p
			versions = versions[:0]
			return nil
		}
		// example: p == golang/go/0.26.0
		if strings.Count(p, "/") == 2 {
			if ver.Version != "" {
				versions = append(versions, ver)
			}
			ver = Version{Version: strings.Split(p, "/")[2]}
			return nil
		}
		// example: p == golang/go/0.26.0/1623958451720/Microsoft.VisualStudio.Code.Manifest
		if strings.Count(p, "/") == 4 {
			asset := Asset{Source: p}
			asset.Type = AssetTypeKey(strings.Split(p, "/")[4])
			ver.Files = append(ver.Files, asset)
			return nil
		}
		return nil
	})
	return exts, err
}

func Search(query string, limit int, sortBy SortCriteria) ([]Extension, error) {
	eqr, err := runQuery(searchQueryJSON(query, limit, sortBy))
	if err != nil {
		return []Extension{}, err
	}
	return eqr.Results[0].Extensions, nil
}

func NewExtension(uniqueID string) (Extension, error) {
	eqr, err := runQuery(latestQueryJSON(uniqueID))
	if err != nil {
		return Extension{}, err
	}
	uuid := eqr.Results[0].Extensions[0].ID
	eqr, err = runQuery(listVersionsQueryJSON(uuid))
	if err != nil {
		return Extension{}, err
	}
	return eqr.Results[0].Extensions[0], err
}

func (e Extension) Assets(version string) ([]Asset, bool) {
	for _, v := range e.Versions {
		if version == v.Version {
			return v.Files, true
		}
	}
	return []Asset{}, false
}

func (e Extension) Asset(version string, assetType AssetTypeKey) (Asset, bool) {
	if assets, exists := e.Assets(version); exists {
		for _, a := range assets {
			if a.Is(assetType) {
				return a, true
			}
		}
	}
	return Asset{}, false
}

// Dir return the path of this extension. The path does not include the output directory, only the path within the output directory.
func (e Extension) Dir() string {
	return path.Join("extensions", strings.ToLower(e.Publisher.Name), strings.ToLower(e.Name))
}

// MetaPath return the path to the metadata.json file for this extension. The path does not include the output directory, only the path within the output directory.
func (e Extension) MetaPath() string {
	return path.Join(e.Dir(), "metadata.json")
}

func (e Extension) SaveMetadata(outputPath string) error {
	// re-run query to populate statistics, list versions query does not populate statistics, is there another way?
	eqr, err := runQuery(latestQueryJSON(e.UniqueID()))
	if err != nil {
		return err
	}
	newExt := eqr.Results[0].Extensions[0]
	newExt.Versions = []Version{}
	j, err := json.Marshal(newExt)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(outputPath, e.MetaPath()), j, os.ModePerm)
}

func (e Extension) IsExtensionPack() bool {
	return len(e.ExtensionPack()) > 0
}

func (e Extension) ExtensionPack() []string {
	pack := []string{}
	for _, p := range e.Versions[0].Properties {
		if p.Key == propKeyExtensionPack {
			if len(p.Value) > 0 {
				pack = strings.Split(p.Value, ",")
			}
			break
		}
	}
	return pack
}

func (e Extension) VersionExists(version, outputPath string) (bool, error) {
	_, err := os.Stat(path.Join(outputPath, e.Dir(), version))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (e Extension) HasVersion(version string) bool {
	for _, v := range e.Versions {
		if v.Version == version {
			return true
		}
	}
	return false
}

func (e Extension) UniqueID() string {
	return e.Publisher.Name + "." + e.Name
}

func (e Extension) InstallCount() int {
	for _, stat := range e.Statistics {
		if stat.Name == "install" {
			return int(stat.Value)
		}
	}
	return -1
}

func (e Extension) AverageRating() float32 {
	for _, stat := range e.Statistics {
		if stat.Name == "averagerating" {
			return stat.Value
		}
	}
	return -1
}

func (e Extension) RatingCount() int {
	for _, stat := range e.Statistics {
		if stat.Name == "ratingcount" {
			return int(stat.Value)
		}
	}
	return -1
}

func (e Extension) KeepVersions(versions ...string) Extension {
	newExt := e
	newExt.Versions = e.Versions[:0]
	for _, v := range e.Versions {
		for _, keep := range versions {
			if v.Version == keep {
				newExt.Versions = append(newExt.Versions, v)
			}
		}
	}
	return newExt
}

func runQuery(q string) (extensionQueryResponse, error) {
	if _, debug := os.LookupEnv(debugEnvVar); debug {
		ioutil.WriteFile("query.json", []byte(q), 0644)
	}
	eqr := extensionQueryResponse{}
	req, err := http.NewRequest(http.MethodPost, "https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery", strings.NewReader(q))
	if err != nil {
		return eqr, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json;api-version=3.0-preview.1")
	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return eqr, err
	}
	if resp.StatusCode != http.StatusOK {
		return eqr, fmt.Errorf("marketplace.visualstudio.com returned HTTP %v", resp.StatusCode)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return eqr, err
	}
	if _, debug := os.LookupEnv(debugEnvVar); debug {
		ioutil.WriteFile("response.json", b, 0644)
	}
	err = json.Unmarshal(b, &eqr)
	if err != nil {
		return eqr, err
	}
	if len(eqr.Results[0].Extensions) == 0 {
		return eqr, ErrExtensionNotFound
	}
	if len(eqr.Results[0].Extensions[0].Versions) == 0 {
		return eqr, ErrExtensionHasNoVersions
	}
	return eqr, err
}

// LatestVersion returns the latest version number for the extension with the given unique ID.
func (e Extension) LatestVersion() string {
	return e.Versions[0].Version
}
