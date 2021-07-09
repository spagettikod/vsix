package vscode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	assetTypeVSIXPackage = "Microsoft.VisualStudio.Services.VSIXPackage"
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
	Versions         []struct {
		Version string `json:"version"`
		Files   []struct {
			AssetType string `json:"assetType"`
			Source    string `json:"source"`
		} `json:"files"`
		Properties []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"properties"`
	} `json:"versions"`
	Statistics []struct {
		Name  string  `json:"statisticName"`
		Value float32 `json:"value"`
	} `json:"statistics"`
}

func Search(query string, limit int8, sortBy SortCritera) ([]Extension, error) {
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

// Download fetches the extensions with the given version from the Marketplace and saves it to outputPath.
func (e Extension) Download(version, outputPath string) error {
	url := ""
	for _, v := range e.Versions {
		if version == v.Version {
			for _, f := range v.Files {
				if f.AssetType == assetTypeVSIXPackage {
					url = f.Source
					break
				}
			}
		}
	}
	if url == "" {
		return fmt.Errorf("version %s could not be found", version)
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(outputPath+"/"+e.VsixFilename(version), b, os.ModePerm)
}

func (e Extension) SaveMetadata(version, outputPath string) error {
	newExt := e.KeepVersions(version)
	j, err := json.Marshal(newExt)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(outputPath+"/"+e.MetaFilename(version), j, os.ModePerm)
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

func (e Extension) VsixFilename(version string) string {
	return fmt.Sprintf("%s.%s-%s.vsix", e.Publisher.Name, e.Name, version)
}

func (e Extension) MetaFilename(version string) string {
	return fmt.Sprintf("%s.%s-%s.json", e.Publisher.Name, e.Name, version)
}

func (e Extension) FileExists(version, outputPath string) (bool, error) {
	_, err := os.Stat(outputPath + "/" + e.VsixFilename(version))
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
