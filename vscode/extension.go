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

// Extension TODO
type Extension struct {
	UniqueID string
	Version  string
	url      string
}

// ParseExtension TODO
func ParseExtension(s string) (Extension, error) {
	id := s[:strings.LastIndex(s, "-")]
	version := s[strings.LastIndex(s, "-"):]
	ext := Extension{UniqueID: id, Version: version}
	return ext, nil
}

// Filename TODO
func (e Extension) Filename() string {
	return fmt.Sprintf("%s.vsix", e)
}

func (e Extension) String() string {
	return fmt.Sprintf("%s-%s", e.UniqueID, e.Version)
}

type extensionQueryResponse struct {
	Results []struct {
		Extensions []struct {
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
		} `json:"extensions"`
	} `json:"results"`
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
	return eqr, err
}

// DownloadVSIX TODO
func DownloadVSIX(ext Extension) error {
	eqr, err := runQuery(latestQueryJSON(ext.UniqueID))
	if err != nil {
		return err
	}
	if len(eqr.Results[0].Extensions[0].Versions) == 0 {
		return fmt.Errorf("no versions found for extension '%s'", ext.UniqueID)
	}
	for _, ef := range eqr.Results[0].Extensions[0].Versions[0].Files {
		if ext.Version != "" && ext.Version != eqr.Results[0].Extensions[0].Versions[0].Version {
			return fmt.Errorf("version '%s' for extension '%s' was no found", ext.Version, ext.UniqueID)
		}
		if ef.AssetType == assetTypeVSIXPackage {
			ext.Version = eqr.Results[0].Extensions[0].Versions[0].Version
			ext.url = ef.Source
		}
	}
	return nil
}

// ListVersions TODO
func ListVersions(ext Extension) ([]string, error) {
	versions := []string{}
	eqr, err := runQuery(latestQueryJSON(ext.UniqueID))
	if err != nil {
		return versions, err
	}
	eqr, err = runQuery(listVersionsJSON(eqr.Results[0].Extensions[0].ID))
	if err != nil {
		return versions, err
	}
	if len(eqr.Results[0].Extensions[0].Versions) == 0 {
		return versions, fmt.Errorf("no versions found for extension '%s'", ext.UniqueID)
	}
	for _, v := range eqr.Results[0].Extensions[0].Versions {
		versions = append(versions, v.Version)
	}
	return versions, nil
}
