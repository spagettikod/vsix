package vscode

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	propKeyExtensionPack = "Microsoft.VisualStudio.Code.ExtensionPack"
)

var (
	ErrVersionNotFound = errors.New("version was not found for this extension")
)

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

func (e Extension) IsMultiPlatform() bool {
	v, _ := e.Version(e.LatestVersion())
	if len(v) > 1 {
		return v[0].TargetPlatform != ""
	}
	return false
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

// LatestVersion returns the latest version number for the extension with the given unique ID.
func (e Extension) LatestVersion() string {
	return e.Versions[0].Version
}

func (e Extension) Version(version string) ([]Version, bool) {
	versions := []Version{}
	for _, v := range e.Versions {
		if v.Version == version {
			versions = append(versions, v)
		}
	}
	return versions, len(versions) > 0
}
