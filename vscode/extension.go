package vscode

import (
	"encoding/json"
	"errors"
	"slices"
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

type StatisticName string

const (
	StatisticInstall StatisticName = "install"
)

// Sort extensions by installs in descending order
type ByPopularity []Extension

func (a ByPopularity) Len() int      { return len(a) }
func (a ByPopularity) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByPopularity) Less(i, j int) bool {
	return a[i].Statistic(string(StatisticInstall)) > a[j].Statistic(string(StatisticInstall))
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

func (e Extension) Copy() Extension {
	e2 := e
	e2.Categories = append([]string{}, e.Categories...)
	e2.Tags = append([]string{}, e.Tags...)
	e2.Statistics = append([]Statistic{}, e.Statistics...)
	e2.Versions = []Version{}
	for _, v := range e.Versions {
		e2.Versions = append(e2.Versions, v.Copy())
	}
	return e2
}

func (e Extension) IsExtensionPack() bool {
	return len(e.ExtensionPack()) > 0
}

func (e Extension) IsMultiPlatform(preRelease bool) bool {
	v, _ := e.Version(e.LatestVersion(preRelease))
	if len(v) > 1 {
		return v[0].RawTargetPlatform != ""
	}
	return false
}

func (e Extension) ExtensionPack() []string {
	pack := []string{}
	if len(e.Versions) > 0 {
		if packages, found := e.Versions[0].GetProperty(propKeyExtensionPack); found && packages.Value != "" {
			pack = strings.Split(packages.Value, ",")
		}
	}
	return pack
}

func (e Extension) Statistic(name string) float32 {
	for _, s := range e.Statistics {
		if s.Name == name {
			return s.Value
		}
	}
	return 0
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
	// newExt.Versions = e.Versions
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
func (e Extension) LatestVersion(preRelease bool) string {
	if len(e.Versions) == 0 {
		return ""
	}
	if preRelease {
		return e.Versions[0].Version
	}
	for _, v := range e.Versions {
		if !v.IsPreRelease() {
			return v.Version
		}
	}
	return ""
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

// Platforms returns target platforms for the extension. The returned
// list will include platforms across all versions eventhough target
// platform might differ across versions.
func (e Extension) Platforms() []string {
	platforms := []string{}

	for _, v := range e.Versions {
		platforms = append(platforms, v.TargetPlatform())
	}

	return slices.Compact(platforms)
}
