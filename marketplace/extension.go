package marketplace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/spagettikod/vsix/vscode"
)

type ExtensionRequest struct {
	UniqueID        vscode.UniqueID
	Version         string
	TargetPlatforms []string
	PreRelease      bool
	Force           bool
}

var (
	ErrVersionNotFound           error = errors.New("could not find version at Marketplace")
	ErrMultiplatformNotSupported error = errors.New("multi-platform extensions are not supported yet")
	ErrOutDirNotFound            error = errors.New("output dir does not exist")
)

func Deduplicate(ers []ExtensionRequest) []ExtensionRequest {
	return slices.CompactFunc(ers, func(a, b ExtensionRequest) bool {
		if a.UniqueID.IsZero() || b.UniqueID.IsZero() {
			return true
		}
		return a.Equals(b)
	})
}

func LatestVersion(uid vscode.UniqueID, preRelease bool) (vscode.Extension, error) {
	ext := vscode.Extension{}
	resp, err := http.Get(fmt.Sprintf("https://www.vscode-unpkg.net/_gallery/%s/%s/latest", uid.Publisher, uid.Name))
	if err != nil {
		return ext, err
	}
	if resp.StatusCode != http.StatusOK {
		return ext, fmt.Errorf("check for latest extension version returned HTTP %v", resp.StatusCode)
	}
	bites, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return ext, err
	}
	if err := json.Unmarshal(bites, &ext); err != nil {
		return ext, err
	}
	return ext, nil
}

func FetchExtension(uniqueID string) (vscode.Extension, error) {
	eqr, err := QueryLatestVersionByUniqueID(uniqueID).Run()
	if err != nil {
		return vscode.Extension{}, err
	}
	uuid := eqr.Results[0].Extensions[0].ID
	eqr, err = QueryAllVersionsByUniqueID(uuid).Run()
	if err != nil {
		return vscode.Extension{}, err
	}
	return eqr.Results[0].Extensions[0], err
}

func (er ExtensionRequest) HasTargetPlatform(tp string) bool {
	for _, t := range er.TargetPlatforms {
		if tp == t {
			return true
		}
	}
	return false
}

func (er ExtensionRequest) Equals(er2 ExtensionRequest) bool {
	if er.UniqueID.Equals(er2.UniqueID) && er.Version == er2.Version && er.PreRelease == er2.PreRelease {
		for _, tp := range er2.TargetPlatforms {
			if !er.HasTargetPlatform(tp) {
				return false
			}
		}
		return len(er.TargetPlatforms) == len(er2.TargetPlatforms)
	}
	return false
}

func (er ExtensionRequest) Matches(tag vscode.VersionTag) bool {
	// skip if the version is a pre-release and the request doesn't want those
	if tag.PreRelease && !er.PreRelease {
		return false
	}
	return len(er.TargetPlatforms) == 0 || // no target platform given, matches all platforms
		slices.Contains(er.TargetPlatforms, tag.TargetPlatform) // is the specific platform given in the command, universal will not be matched as it is not included in the version json (see next condition)
}
