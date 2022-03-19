package marketplace

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/vscode"
)

type ExtensionRequest struct {
	UniqueID        string
	Version         string
	TargetPlatforms []string
	PreRelease      bool
}

var (
	ErrVersionNotFound           error = errors.New("could not find version at Marketplace")
	ErrMultiplatformNotSupported error = errors.New("multi-platform extensions are not supported yet")
	ErrOutDirNotFound            error = errors.New("output dir does not exist")
)

func Deduplicate(ers []ExtensionRequest) []ExtensionRequest {
	result := []ExtensionRequest{}
	for _, er := range ers {
		// add the first value
		if len(result) == 0 {
			result = append(result, er)
			continue
		}
		found := false
		for _, er2 := range result {
			if er.Equals(er2) {
				found = true
				break
			}
		}
		if !found {
			result = append(result, er)
		}
	}
	return result
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
	if er.UniqueID == er2.UniqueID && er.Version == er2.Version && er.PreRelease == er2.PreRelease {
		for _, tp := range er2.TargetPlatforms {
			if !er.HasTargetPlatform(tp) {
				return false
			}
		}
		return len(er.TargetPlatforms) == len(er2.TargetPlatforms)
	}
	return false
}

func (pe ExtensionRequest) ValidTargetPlatform(v vscode.Version) bool {
	// empty target platform equals Universal and is always valid so is
	// an empty list of unwanted platforms
	if v.TargetPlatform == "" || len(pe.TargetPlatforms) == 0 {
		return true
	}
	for _, tp := range pe.TargetPlatforms {
		if v.TargetPlatform == tp {
			return true
		}
	}
	return false
}

func (pe ExtensionRequest) String() string {
	if pe.Version == "" {
		return pe.UniqueID
	}
	return fmt.Sprintf("%s-%s", pe.UniqueID, pe.Version)
}

// rewrite this, half the code is the same as Download, recursive function complicates things,
// maybe rethink the entire setup?
func (pe ExtensionRequest) DownloadVSIXPackage(root string) error {
	elog := log.With().Str("extension", pe.UniqueID).Str("dir", root).Logger()

	elog.Debug().Msg("only VSIXPackage will be fetched")
	elog.Debug().Msg("checking if output directory exists")
	if exists, err := outDirExists(root); !exists {
		return err
	}

	elog.Info().Msg("searching for extension at Marketplace")
	ext, err := FetchExtension(pe.UniqueID)
	if err != nil {
		return err
	}
	if ext.IsExtensionPack() {
		elog.Info().Msg("is extension pack, getting pack contents")
		for _, pack := range ext.ExtensionPack() {
			erPack := ExtensionRequest{UniqueID: pack}
			err := erPack.DownloadVSIXPackage(root)
			if err != nil {
				return err
			}
		}
	}

	if pe.Version == "" {
		elog.Debug().Msg("version was not specified, setting to latest version")
		pe.Version = ext.LatestVersion()
	}
	if _, found := ext.Version(pe.Version); !found {
		return ErrVersionNotFound
	}
	elog = elog.With().Str("version", pe.Version).Logger()

	elog.Debug().Msg("version has been determined")

	if ext.IsMultiPlatform() {
		return ErrMultiplatformNotSupported
	}

	filename := path.Join(root, fmt.Sprintf("%s-%s.vsix", ext.UniqueID(), pe.Version))
	elog = elog.With().Str("destination", filename).Logger()
	elog.Debug().Msg("checking if destination already exists")
	if _, err = os.Stat(filename); err == nil {
		elog.Info().Msg("skipping download, version already exist at output path")
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	asset, found := ext.Asset(pe.Version, vscode.VSIXPackage)
	if !found {
		return fmt.Errorf("version %s did not contain a VSIX package", pe.Version)
	}
	elog.Info().
		Str("source", asset.Source).
		Msg("downloading")
	// download setting filename to asset type
	b, err := asset.Download()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, b, 0666)
}

// Download will fetch the extension all its assets making it ready to be
// served by the serve command. It returns true if download succeeded and
// false if the requested version already exists at output.
func (extReq ExtensionRequest) Download() (vscode.Extension, error) {
	elog := log.With().Str("extension", extReq.UniqueID).Str("extension_version", extReq.Version).Logger()

	elog.Debug().Msg("searching for extension at Marketplace")
	ext, err := FetchExtension(extReq.UniqueID)
	if err != nil {
		return vscode.Extension{}, err
	}

	// TODO ms-vscode-remote.vscode-remote-extensionpack seems to have a VSIX-file, does this mean
	// we don't have to download all extensions? If we need to download all extensions how do
	// we know which version?
	// if ext.IsExtensionPack() {
	// 	elog.Info().Msg("is extension pack, getting pack contents")
	// 	for _, pack := range ext.ExtensionPack() {
	// 		erPack := ExtensionRequest{UniqueID: pack}
	// 		_, err := erPack.Download(db)
	// 		if err != nil {
	// 			return false, err
	// 		}
	// 	}
	// }

	// set version to the latest since no version was given in the request
	if extReq.Version == "" {
		elog.Debug().Msg("version was not specified, setting to latest version")
		extReq.Version = ext.LatestVersion()
	}

	// only keep the version from the request
	ext = ext.KeepVersions(extReq.Version)
	if len(ext.Versions) == 0 {
		elog.Debug().Msg("requested version was not found at Marketplace")
		return vscode.Extension{}, ErrVersionNotFound
	}

	elog.Debug().Msg("found version at Marketplace")

	return ext, nil
}

func outDirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, ErrOutDirNotFound
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
