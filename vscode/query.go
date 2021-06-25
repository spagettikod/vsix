package vscode

import (
	"fmt"
)

type SortCritera int

const (
	SortByNone         SortCritera = 0
	SortByInstallCount SortCritera = 4
)

var latestVersionQueryTemplate string = `{
    "filters": [
        {
            "criteria": [
                {
                    "filterType": 8,
                    "value": "Microsoft.VisualStudio.Code"
                },
                {
                    "filterType": 10,
                    "value": "%s"
                },
                {
                    "filterType": 12,
                    "value": "4096"
                }
            ],
            "pageNumber": 1,
            "pageSize": %v,
            "sortBy": %v,
            "sortOrder": 0
        }
    ],
    "assetTypes": [],
    "flags": 946
}`

var listVersionsQueryTemplate string = `{
    "filters": [
        {
            "criteria": [
                {
                    "filterType": 8,
                    "value": "Microsoft.VisualStudio.Code"
                },
                {
                    "filterType": 4,
                    "value": "%s"
                },
                {
                    "filterType": 12,
                    "value": "4096"
                }
            ],
            "pageNumber": 1,
            "pageSize": 1,
            "sortBy": 0,
            "sortOrder": 0
        }
    ],
    "assetTypes": [],
    "flags": 51
}`

func ParseSortCritera(sortBy string) (SortCritera, error) {
	switch sortBy {
	case "install":
		return SortByInstallCount, nil
	case "none":
		return SortByNone, nil
	}
	return SortByNone, fmt.Errorf("%s is not a valid sort critera", sortBy)
}

func latestQueryJSON(uniqueID string) string {
	return fmt.Sprintf(latestVersionQueryTemplate, uniqueID, 1, SortByNone)
}

func listVersionsQueryJSON(uniqueID string) string {
	return fmt.Sprintf(listVersionsQueryTemplate, uniqueID)
}

func searchQueryJSON(query string, limit int8, sortBy SortCritera) string {
	return fmt.Sprintf(latestVersionQueryTemplate, query, limit, sortBy)
}
