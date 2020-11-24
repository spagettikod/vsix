package vscode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	assetTypeVSIXPackage = "Microsoft.VisualStudio.Services.VSIXPackage"
)

// ParseExtension TODO
// func ParseExtension(s string) (id string, version string) {
// 	id = s[:strings.LastIndex(s, "-")]
// 	version = s[strings.LastIndex(s, "-"):]
// 	ext := Extension{UniqueID: id, Version: version}
// 	return
// }

type extensionQueryResponse struct {
	Results []struct {
		Extensions []Extension `json:"extensions"`
	} `json:"results"`
}

// Extension TODO
type Extension struct {
	Publisher struct {
		ID   string `json:"publisherId"`
		Name string `json:"publisherName"`
	} `json:"publisher"`
	Name     string `json:"extensionName"`
	ID       string `json:"extensionID"`
	Versions []struct {
		Version string `json:"version"`
		Files   []struct {
			AssetType string `json:"assetType"`
			Source    string `json:"source"`
		} `json:"files"`
	} `json:"versions"`
}

// PackageURL TODO
func (e Extension) PackageURL(version string) string {
	for _, v := range e.Versions {
		if v.Version == version {
			for _, f := range v.Files {
				if f.AssetType == assetTypeVSIXPackage {
					return f.Source
				}
			}
		}
	}
	return ""
}

// Filename TODO
func (e Extension) Filename(version string) string {
	return fmt.Sprintf("%s.%s-%s.vsix", e.Publisher.Name, e.Name, version)
}

func runQuery(q string) (extensionQueryResponse, error) {
	eqr := extensionQueryResponse{}
	req, err := http.NewRequest(http.MethodPost, "https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery", strings.NewReader(q))
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
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return eqr, err
	}
	err = json.Unmarshal(b, &eqr)
	if err != nil {
		return eqr, err
	}
	if len(eqr.Results[0].Extensions) == 0 {
		return eqr, errors.New("extension could not be found")
	}
	if len(eqr.Results[0].Extensions[0].Versions) == 0 {
		return eqr, fmt.Errorf("extension has no versions")
	}
	return eqr, err
}

// DownloadVSIX TODO
func DownloadVSIX(id, version string) error {
	// eqr, err := runQuery(latestQueryJSON(id))
	// if err != nil {
	// 	return err
	// }
	// for _, ef := range eqr.Results[0].Extensions[0].Versions[0].Files {
	// 	if version != "" && version != eqr.Results[0].Extensions[0].Versions[0].Version {
	// 		return fmt.Errorf("version '%s' for extension '%s' was no found", version, ext.UniqueID)
	// 	}
	// }
	// resp, err := http.Get(ext.url)
	// if err != nil {
	// 	return err
	// }
	// b, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	return err
	// }
	// err = ioutil.WriteFile(ext.Filename(), b, os.ModePerm)
	// return err
	return nil
}

// ListVersions TODO
func ListVersions(id string) (Extension, error) {
	ext := Extension{}
	eqr, err := runQuery(latestQueryJSON(id))
	if err != nil {
		return ext, err
	}
	eqr, err = runQuery(listVersionsJSON(eqr.Results[0].Extensions[0].ID))
	if err != nil {
		return ext, err
	}
	ext = eqr.Results[0].Extensions[0]
	return ext, nil
}

// Latest TODO
func Latest(id string) (Extension, error) {
	ext := Extension{}
	eqr, err := runQuery(latestQueryJSON(id))
	if err != nil {
		return ext, err
	}
	ext = eqr.Results[0].Extensions[0]
	return ext, nil
}
