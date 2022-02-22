package vscode

import (
	"encoding/json"
)

type QueryResults struct {
	Results []struct {
		Extensions     []Extension `json:"extensions"`
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

type Criteria struct {
	FilterType FilterType `json:"filterType"`
	Value      string     `json:"value"`
}

type SortCriteria int
type QueryFlag int
type FilterType int

const (
	ByNone          SortCriteria = 0
	ByName          SortCriteria = 2
	ByPublishedDate SortCriteria = 5
	ByInstallCount  SortCriteria = 4
	ByRating        SortCriteria = 12

	FlagAllVersions   QueryFlag = 51
	FlagLatestVersion QueryFlag = 950

	FilterTypeTag           FilterType = 1
	FilterTypeExtensionID   FilterType = 4
	FilterTypeCatergory     FilterType = 5
	FilterTypeExtensionName FilterType = 7
	FilterTypeTarget        FilterType = 8
	FilterTypeFeatured      FilterType = 9
	FilterTypeSearchText    FilterType = 10
)

var (
	MSVSCodeCriteria    = Criteria{FilterType: FilterTypeTarget, Value: "Microsoft.VisualStudio.Code"}
	SomeUnknownCriteria = Criteria{FilterType: 12, Value: "4096"}
)

var latestVersionQueryTemplate2 = Filter{
	Criteria: []Criteria{
		MSVSCodeCriteria,
		SomeUnknownCriteria,
	},
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
	q.Flags = FlagLatestVersion
	return q
}

func LatestQueryJSON(uniqueID string) string {
	q := NewQuery()
	q.AddCriteria(Criteria{
		FilterType: FilterTypeExtensionName,
		Value:      uniqueID,
	})
	q.Flags = FlagLatestVersion
	b, _ := json.Marshal(q)
	return string(b)
}

func listVersionsQueryJSON(uniqueID string) string {
	q := NewQuery()
	q.AddCriteria(Criteria{
		FilterType: FilterTypeExtensionID,
		Value:      uniqueID,
	})
	q.Flags = FlagAllVersions
	b, _ := json.Marshal(q)
	return string(b)
}

func searchQueryJSON(query string, limit int, sortBy SortCriteria) string {
	q := NewQuery()
	q.AddCriteria(Criteria{
		FilterType: FilterTypeSearchText,
		Value:      query,
	})
	q.Flags = FlagLatestVersion
	q.Filters[0].SortBy = sortBy
	q.Filters[0].PageSize = limit
	b, _ := json.Marshal(q)
	return string(b)
}
