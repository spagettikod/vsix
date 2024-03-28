package marketplace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spagettikod/vsix/vscode"
)

type QueryResults struct {
	Results []struct {
		Extensions     []vscode.Extension `json:"extensions"`
		ResultMetadata []struct {
			MetadataType  string `json:"metadataType"`
			MetadataItems []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"metadataItems"`
		} `json:"resultMetadata"`
	} `json:"results"`
}

type Query struct {
	Filters    []Filter      `json:"filters"`
	AssetTypes []interface{} `json:"assetTypes"`
	Flags      QueryFlag     `json:"flags"`
}

type Filter struct {
	Criteria   []Criteria   `json:"criteria"`
	PageNumber int          `json:"pageNumber"`
	PageSize   int          `json:"pageSize"`
	SortBy     SortCriteria `json:"sortBy"`
	SortOrder  int          `json:"sortOrder"`
}

func (f Filter) GetCriteria(filterType FilterType) (Criteria, bool) {
	for _, c := range f.Criteria {
		if c.FilterType == filterType {
			return c, true
		}
	}
	return Criteria{}, false
}

type Criteria struct {
	FilterType FilterType `json:"filterType"`
	Value      string     `json:"value"`
}

type SortCriteria int
type FilterType int

const (
	ByNone          SortCriteria = 0
	ByName          SortCriteria = 2
	ByInstallCount  SortCriteria = 4
	ByPublishedDate SortCriteria = 10
	ByRating        SortCriteria = 12

	FlagIncludeVersions            QueryFlag = 0x1
	FlagIncludeFiles               QueryFlag = 0x2
	FlagIncludeCatergoryAndTags    QueryFlag = 0x4
	FlagIncludeSharedAccounts      QueryFlag = 0x8
	FlagIncludeVersionProperties   QueryFlag = 0x10
	FlagExcludeNonValidated        QueryFlag = 0x20
	FlagIncludeInstallationTargets QueryFlag = 0x40
	FlagIncludeAssetURI            QueryFlag = 0x80
	FlagIncludeStatistics          QueryFlag = 0x100
	FlagIncludeLatestVersionOnly   QueryFlag = 0x200
	FlagUnpublished                QueryFlag = 0x1000

	FilterTypeTag           FilterType = 1
	FilterTypeExtensionID   FilterType = 4
	FilterTypeCatergory     FilterType = 5
	FilterTypeExtensionName FilterType = 7
	FilterTypeTarget        FilterType = 8
	FilterTypeFeatured      FilterType = 9
	FilterTypeSearchText    FilterType = 10

	MaximumPageSize = 1000

	debugEnvVar = "VSIX_DEBUG"
)

var (
	MSVSCodeCriteria          = Criteria{FilterType: FilterTypeTarget, Value: "Microsoft.VisualStudio.Code"}
	SomeUnknownCriteria       = Criteria{FilterType: 12, Value: "4096"}
	ErrExtensionNotFound      = errors.New("extension could not be found at Marketplace")
	ErrExtensionHasNoVersions = errors.New("extension has no versions")
	ErrInvalidQuery           = errors.New("query is not valid, it might be incomplete or malformatted")
)

type extensionQueryResponse struct {
	Results []struct {
		Extensions []vscode.Extension `json:"extensions"`
	} `json:"results"`
}

type QueryFlag int

func (f QueryFlag) Is(f2 QueryFlag) bool {
	return (f2 & f) == f2
}

func NewQuery() Query {
	q := Query{}
	f := Filter{
		Criteria: []Criteria{
			MSVSCodeCriteria,
			SomeUnknownCriteria,
		},
		PageNumber: 1,
		PageSize:   1,
		SortBy:     ByNone,
		SortOrder:  0,
	}
	q.Filters = append(q.Filters, f)
	q.Flags = FlagIncludeLatestVersionOnly | FlagExcludeNonValidated | FlagIncludeAssetURI | FlagIncludeVersionProperties | FlagIncludeFiles | FlagIncludeCatergoryAndTags
	return q
}

func (q Query) AddCriteria(c Criteria) {
	nonDefaultCritera := q.Filters[0].Criteria[1 : len(q.Filters[0].Criteria)-1]
	nonDefaultCritera = append(nonDefaultCritera, c)
	q.Filters[0].Criteria = []Criteria{}
	q.Filters[0].Criteria = append(q.Filters[0].Criteria, MSVSCodeCriteria)
	q.Filters[0].Criteria = append(q.Filters[0].Criteria, nonDefaultCritera...)
	q.Filters[0].Criteria = append(q.Filters[0].Criteria, SomeUnknownCriteria)
}

// CriteriaValues returns an array of values among all critera in the query matching the supplied filterType.
func (q Query) CriteriaValues(filterType FilterType) []string {
	values := []string{}
	for _, f := range q.Filters {
		for _, c := range f.Criteria {
			if c.FilterType == filterType {
				values = append(values, c.Value)
			}
		}
	}
	return values
}

// IsEmptyQuery returns true if there are no other criteria than the default ones. See NewQuery
// as an example what defines an empty query.
func (q Query) IsEmptyQuery() bool {
	if len(q.Filters) == 1 {
		if len(q.Filters[0].Criteria) == 2 {
			if c, _ := q.Filters[0].GetCriteria(SomeUnknownCriteria.FilterType); c != SomeUnknownCriteria {
				return false
			}
			if c, _ := q.Filters[0].GetCriteria(MSVSCodeCriteria.FilterType); c != MSVSCodeCriteria {
				return false
			}
			return true
		}
	}
	return false
}

// IsValid return true if the query contain all necessary items and values to be regarded as a valid query.
func (q Query) IsValid() bool {
	for _, f := range q.Filters {
		if c, _ := f.GetCriteria(MSVSCodeCriteria.FilterType); c == MSVSCodeCriteria {
			return true
		}
	}
	return false
}

// RunAll executes the Run function until all pages with extensions are fetched.
func (q Query) RunAll(limit int) ([]vscode.Extension, error) {
	if limit == 0 || limit >= MaximumPageSize {
		q.Filters[0].PageSize = MaximumPageSize
	} else {
		q.Filters[0].PageSize = limit
	}

	exts := []vscode.Extension{}
	for {
		eqr, err := q.Run()
		if err != nil {
			return exts, err
		}
		exts = append(exts, eqr.Results[0].Extensions...)
		// if there is a limit set (limit is larger than 0) exit when we've got the requested number of extensions or more
		if (limit > 0 && len(exts) >= limit) || len(eqr.Results[0].Extensions) < q.Filters[0].PageSize {
			break
		}
		q.Filters[0].PageNumber = q.Filters[0].PageNumber + 1
	}
	// if there is no limit (limit is zero), we return all extensions found, otherwise slice the correct number of extensions
	if limit == 0 || len(exts) < limit {
		return exts, nil
	} else {
		return exts[:limit], nil
	}
}

func (q Query) Run() (extensionQueryResponse, error) {
	if _, debug := os.LookupEnv(debugEnvVar); debug {
		os.WriteFile("query.json", []byte(q.ToJSON()), 0644)
	}
	eqr := extensionQueryResponse{}
	req, err := http.NewRequest(http.MethodPost, "https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery", strings.NewReader(q.ToJSON()))
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
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return eqr, err
	}
	if _, debug := os.LookupEnv(debugEnvVar); debug {
		os.WriteFile("response.json", b, 0644)
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

func (q Query) ToJSON() string {
	qjson, _ := json.Marshal(q)
	return string(qjson)
}

func (q Query) SortBy() SortCriteria {
	return q.Filters[0].SortBy
}

func QueryNoCritera(sortBy SortCriteria) Query {
	q := NewQuery()
	q.Filters[0].Criteria = []Criteria{}
	q.Filters[0].Criteria = append(q.Filters[0].Criteria, MSVSCodeCriteria)
	q.Filters[0].Criteria = append(q.Filters[0].Criteria, SomeUnknownCriteria)
	q.Flags = FlagIncludeLatestVersionOnly | FlagExcludeNonValidated | FlagIncludeAssetURI | FlagIncludeVersionProperties | FlagIncludeFiles | FlagIncludeCatergoryAndTags | FlagIncludeStatistics
	q.Filters[0].SortBy = sortBy
	// q.Filters[0].PageSize = limit
	return q
}

func QueryLastestVersionByText(query string, sortBy SortCriteria) Query {
	q := NewQuery()
	q.AddCriteria(Criteria{
		FilterType: FilterTypeSearchText,
		Value:      query,
	})
	q.Flags = FlagIncludeLatestVersionOnly | FlagExcludeNonValidated | FlagIncludeAssetURI | FlagIncludeVersionProperties | FlagIncludeFiles | FlagIncludeCatergoryAndTags | FlagIncludeStatistics
	q.Filters[0].SortBy = sortBy
	// q.Filters[0].PageSize = limit
	return q
}

func QueryLatestVersionByUniqueID(uniqueID string) Query {
	q := NewQuery()
	q.AddCriteria(Criteria{
		FilterType: FilterTypeExtensionName,
		Value:      uniqueID,
	})
	q.Flags = FlagIncludeLatestVersionOnly | FlagExcludeNonValidated | FlagIncludeAssetURI | FlagIncludeVersionProperties | FlagIncludeFiles | FlagIncludeCatergoryAndTags | FlagIncludeStatistics
	return q
}

func QueryAllVersionsByUniqueID(uniqueID string) Query {
	q := NewQuery()
	q.AddCriteria(Criteria{
		FilterType: FilterTypeExtensionID,
		Value:      uniqueID,
	})
	q.Flags = FlagIncludeVersions | FlagIncludeFiles | FlagIncludeVersionProperties | FlagExcludeNonValidated | FlagIncludeStatistics
	return q
}
