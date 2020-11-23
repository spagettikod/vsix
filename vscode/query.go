package vscode

import (
	"fmt"
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
            "pageSize": 1,
            "sortBy": 0,
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

func latestQueryJSON(uniqueID string) string {
	return fmt.Sprintf(latestVersionQueryTemplate, uniqueID)
}

func listVersionsJSON(uniqueID string) string {
	return fmt.Sprintf(listVersionsQueryTemplate, uniqueID)
}
