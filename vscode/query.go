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
				Name  string `json:"name`
				Count int    `json:"count`
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
	MSVSCodeCriteria    = Criteria{FilterType: 8, Value: "Microsoft.VisualStudio.Code"}
	SomeUnknownCriteria = Criteria{FilterType: 12, Value: "4096"}
)

var latestVersionQueryTemplate2 = Filter{
	Criteria: []Criteria{
		MSVSCodeCriteria,
		SomeUnknownCriteria,
	},
}

func (q Query) AddCriteria(c Criteria) {
	q.Filters[0].Criteria = append(
		q.Filters[0].Criteria,
		c,
	)
}

func baseQuery() Query {
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

func latestQueryJSON(uniqueID string) string {
	q := baseQuery()
	q.AddCriteria(Criteria{
		FilterType: FilterTypeSearchText,
		Value:      uniqueID,
	})
	q.Flags = 512
	b, _ := json.Marshal(q)
	return string(b)
}

func listVersionsQueryJSON(uniqueID string) string {
	q := baseQuery()
	q.AddCriteria(Criteria{
		FilterType: FilterTypeExtensionID,
		Value:      uniqueID,
	})
	// q.Flags = 950
	b, _ := json.Marshal(q)
	return string(b)
}

func searchQueryJSON(query string, limit int, sortBy SortCriteria) string {
	q := baseQuery()
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
