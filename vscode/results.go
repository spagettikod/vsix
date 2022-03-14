package vscode

import (
	"encoding/json"
	"path"
)

type Results struct {
	Results []*Result `json:"results"`
}

type Result struct {
	Extensions     []Extension      `json:"extensions"`
	PagingToken    interface{}      `json:"pagingToken"`
	ResultMetadata []ResultMetadata `json:"resultMetadata"`
}

type ResultMetadata struct {
	MetadataType  string         `json:"metadataType"`
	MetadataItems []MetadataItem `json:"metadataItems"`
}

type MetadataItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func NewResults(exts []Extension) Results {
	return Results{
		Results: []*Result{
			{
				Extensions: exts,
				ResultMetadata: []ResultMetadata{
					{
						MetadataType: "ResultCount",
						MetadataItems: []MetadataItem{
							{
								Name:  "TotalCount",
								Count: len(exts),
							},
						},
					},
				},
			},
		},
	}
}

func (r Results) Deduplicate() {
	for _, result := range r.Results {
		notExist := map[string]bool{}
		dedup := []Extension{}
		for _, i := range result.Extensions {
			notExist[i.ID] = true
		}
		for _, i := range result.Extensions {
			if notExist[i.ID] {
				dedup = append(dedup, i)
				notExist[i.ID] = false
			}
		}
		result.Extensions = dedup
	}
}

func (r Results) SetAssetEndpoint(assetEndpoint string) {
	for _, res := range r.Results {
		for _, e := range res.Extensions {
			for j, v := range e.Versions {
				for i, f := range v.Files {
					v.Files[i].Source = assetEndpoint + f.Source
					e.Versions[j].AssetURI = assetEndpoint + path.Dir(f.Source)
					e.Versions[j].FallbackAssetURI = assetEndpoint + path.Dir(f.Source)
				}
			}
		}
	}
}

func (r Results) String() string {
	b, err := json.MarshalIndent(r, "", "   ")
	if err != nil {
		return "! JSON UNMARSHAL FAILED !"
	}
	return string(b)
}
