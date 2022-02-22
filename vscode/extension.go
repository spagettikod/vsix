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
	propKeyExtensionPack = "Microsoft.VisualStudio.Code.ExtensionPack"
	debugEnvVar          = "VSIX_DEBUG"
)

var (
	ErrExtensionNotFound      = errors.New("extension could not be found at Marketplace")
	ErrExtensionHasNoVersions = errors.New("extension has no versions")
	ErrVersionNotFound        = errors.New("version was not found for this extension")
)

type extensionQueryResponse struct {
	Results []struct {
		Extensions []Extension `json:"extensions"`
	} `json:"results"`
}

type Extension struct {
	Publisher        Publisher   `json:"publisher"`
	ID               string      `json:"extensionId"`
	Name             string      `json:"extensionName"`
	DisplayName      string      `json:"displayName"`
	Flags            string      `json:"flags"`
	LastUpdated      time.Time   `json:"lastUpdated"`
	PublishedDate    time.Time   `json:"publishedDate"`
	ReleaseDate      time.Time   `json:"releaseDate"`
	ShortDescription string      `json:"shortDescription"`
	Versions         []Version   `json:"versions,omitempty"`
	Categories       []string    `json:"categories"`
	Tags             []string    `json:"tags"`
	Statistics       []Statistic `json:"statistics"`
	DeploymentType   int         `json:"deploymentType"`
	Path             string      `json:"-"`
}

type Publisher struct {
	ID              string `json:"publisherId"`
	Name            string `json:"publisherName"`
	DisplayName     string `json:"displayName"`
	Flags           string `json:"flags"`
	Domain          string `json:"domain"`
	IsDomainVerfied bool   `json:"isDomainVerified"`
}

type Statistic struct {
	Name  string  `json:"statisticName"`
	Value float32 `json:"value"`
}

func Search(query string, limit int, sortBy SortCriteria) ([]Extension, error) {
	eqr, err := RunQuery(searchQueryJSON(query, limit, sortBy))
	if err != nil {
		return []Extension{}, err
	}
	return eqr.Results[0].Extensions, nil
}

func NewExtension(uniqueID string) (Extension, error) {
	eqr, err := RunQuery(LatestQueryJSON(uniqueID))
	if err != nil {
		return Extension{}, err
	}
	uuid := eqr.Results[0].Extensions[0].ID
	eqr, err = RunQuery(listVersionsQueryJSON(uuid))
	if err != nil {
		return Extension{}, err
	}
	return eqr.Results[0].Extensions[0], err
}

// Assets return the assets for a certain version of an extension.
func (e Extension) Assets(version string) ([]Asset, bool) {
	for _, v := range e.Versions {
		if version == v.Version {
			return v.Files, true
		}
	}
	return []Asset{}, false
}

// Asset return the asset for a certain version on an extension.
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

func (e Extension) String() string {
	b, err := json.MarshalIndent(e, "", "   ")
	if err != nil {
		return "! JSON UNMARSHAL FAILED !"
	}
	return string(b)
}

func RunQuery(q string) (extensionQueryResponse, error) {
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

func (e Extension) Version(version string) (Version, bool) {
	for _, v := range e.Versions {
		if v.Version == version {
			return v, true
		}
	}
	return Version{}, false
}
