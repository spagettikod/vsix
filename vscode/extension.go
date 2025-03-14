package vscode

import (
	"encoding/json"
	"errors"
	"fmt"
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

// SortFuncExtensionByInstallCount func to use for slices.SortFunc to sort extensions by install count, most installs comes first.
var SortFuncExtensionByInstallCount = func(e1, e2 Extension) int {
	if e1.InstallCount() < e2.InstallCount() {
		return 1
	} else if e1.InstallCount() > e2.InstallCount() {
		return -1
	}
	return 0
}

// SortFuncExtensionByDisplayName func to use for slices.SortFunc to sort extensions by display name, most installs comes first.
var SortFuncExtensionByDisplayName = func(e1, e2 Extension) int {
	return strings.Compare(strings.ToLower(e1.DisplayName), strings.ToLower(e2.DisplayName))
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

func (e Extension) UniqueID() UniqueID {
	uid, ok := Parse(e.Publisher.Name + "." + e.Name)
	if !ok {
		panic(fmt.Sprintf("could not parse unique id %s %s", e.Publisher.Name, e.Name))
	}
	return uid
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

	slices.Sort(platforms)
	return slices.Compact(platforms)
}

func (e Extension) VersionByTag(tag VersionTag) (Version, bool) {
	for _, v := range e.Versions {
		if tag == v.Tag(tag.UniqueID) {
			return v, true
		}
	}
	return Version{}, false
}

// RewriteAssetURL returns a copy of the extension but where the assets are directed to the local VSIX serve external URL.
func (e Extension) RewriteAssetURL(externalURL string) Extension {
	cp := e
	cp.Versions = []Version{}
	for _, v := range e.Versions {
		tag := v.Tag(e.UniqueID())
		cpVersion := v
		cpVersion.Files = []Asset{}
		for _, a := range v.Files {
			cpAsset := a
			cpAsset.Source = fmt.Sprintf("%s/%s", externalURL, tag.PatternByAssetType(a.Type))
			cpVersion.Files = append(cpVersion.Files, cpAsset)
		}
		cpVersion.AssetURI = fmt.Sprintf("%s/%s", externalURL, tag.Pattern())
		cpVersion.FallbackAssetURI = fmt.Sprintf("%s/%s", externalURL, tag.Pattern())
		cp.Versions = append(cp.Versions, cpVersion)
	}
	return cp
}

// MatchesQuery test if the extension matches any of the text terms in the input.
func (e Extension) MatchesQuery(terms ...string) bool {
	for _, term := range terms {
		term := strings.ToLower(term)
		if strings.Contains(e.Name, term) ||
			strings.Contains(e.DisplayName, term) ||
			strings.Contains(e.Publisher.Name, term) ||
			strings.Contains(e.ShortDescription, term) {
			return true
		}
	}
	return false
}

// MatchesTargetPlatforms returns true if one of the given target platforms matches a platform in any of the versions.
func (e Extension) MatchesTargetPlatforms(targetPlatforms ...string) bool {
	for _, tp := range targetPlatforms {
		if slices.Contains(e.Platforms(), tp) {
			return true
		}
	}
	return false
}

// HasPreRelease returns true if any of the versions are pre-release.
func (e Extension) HasPreRelease() bool {
	for _, v := range e.Versions {
		if v.IsPreRelease() {
			return true
		}
	}
	return false
}
