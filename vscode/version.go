package vscode

type Version struct {
	Version    string  `json:"version"`
	Files      []Asset `json:"files"`
	Properties []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"properties"`
}
