package vscode

import (
	"encoding/json"
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

func NewResults() Results {
	return Results{
		Results: []*Result{
			{
				Extensions: []Extension{},
				ResultMetadata: []ResultMetadata{
					{
						MetadataType: "ResultCount",
						MetadataItems: []MetadataItem{
							{
								Name:  "TotalCount",
								Count: 0,
							},
						},
					},
				},
			},
		},
	}
}

func (r Results) SetTotalCount(v int) {
	r.Results[0].ResultMetadata[0].MetadataItems[0].Count = v
}

func (r Results) AddExtensions(exts []Extension) {
	for _, e := range exts {
		if !r.Contains(e) {
			r.Results[0].Extensions = append(r.Results[0].Extensions, e)
		}
	}
}

func (r Results) Contains(e Extension) bool {
	for _, e1 := range r.Results[0].Extensions {
		if e1.ID == e.ID {
			return true
		}
	}
	return false
}

func (r Results) RewriteAssetURL(externalURL string) {
	for _, res := range r.Results {
		for i := range res.Extensions {
			res.Extensions[i] = res.Extensions[i].RewriteAssetURL(externalURL)
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
