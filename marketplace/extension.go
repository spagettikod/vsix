package marketplace

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spagettikod/vsix/vscode"
)

type ExtensionRequest struct {
	UniqueID        string
	Version         string
	TargetPlatforms []string
}

var (
	ErrVersionNotFound           error = errors.New("could not find version at Marketplace")
	ErrMultiplatformNotSupported error = errors.New("multi-platform extensions are not supported yet")
	ErrOutDirNotFound            error = errors.New("output dir does not exist")
)

// NewFromFile will walk the given path in search of text files that contain valid extension request definitions.
func NewFromFile(p string) ([]ExtensionRequest, error) {
	ers := []ExtensionRequest{}
	dir, err := isDir(p)
	if err != nil {
		return ers, err
	}
	if dir {
		log.Debug().Str("path", p).Msg("found directory")
		fis, err := ioutil.ReadDir(p)
		if err != nil {
			return ers, err
		}
		for _, fi := range fis {
			if !fi.IsDir() {
				newErs, err := NewFromFile(fmt.Sprintf("%s%s%s", p, string(os.PathSeparator), fi.Name()))
				if err != nil {
					return ers, err
				}
				ers = append(ers, newErs...)
			}
		}
	} else {
		newErs, err := parseFile(p)
		if err != nil {
			return newErs, err
		}
		ers = append(ers, newErs...)
	}
	log.Debug().Msgf("found %v extensions in total", len(ers))
	return ers, err
}

func parseFile(p string) ([]ExtensionRequest, error) {
	exts := []ExtensionRequest{}

	plog := log.With().Str("filename", p).Logger()

	plog.Info().Msg("found file")
	data, err := ioutil.ReadFile(p)
	if err != nil {
		return exts, err
	}
	if isPlainText(data) {
		plog.Debug().Msg("parsing file")
		exts, err = parse(data)
		if err != nil {
			return exts, err
		}
		plog.Info().Msgf("found %v extentions in file", len(exts))
	} else {
		plog.Info().Msg("skipping, not a text file")
	}

	return exts, nil
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

// isPlainText will try to auto detect if the given data is a text file.
func isPlainText(data []byte) bool {
	mime := http.DetectContentType(data)
	return strings.Index(mime, "text/plain") == 0
}

// isDir returns true if the given path is a directory
func isDir(p string) (bool, error) {
	info, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// parse will process file data and extract all valid extension requests from the file
func parse(data []byte) (extensions []ExtensionRequest, err error) {
	buf := bytes.NewBuffer(data)
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Index(line, "#") == 0 || len(line) == 0 {
			continue
		}
		splitLine := strings.Split(line, " ")
		ext := ExtensionRequest{}
		if len(splitLine) > 0 && len(splitLine) < 3 {
			ext.UniqueID = strings.TrimSpace(splitLine[0])
			if len(splitLine) == 2 {
				ext.Version = strings.TrimSpace(splitLine[1])
			}
			extensions = append(extensions, ext)
			if ext.Version == "" {
				log.Debug().Str("extension", ext.UniqueID).Msg("parsed")
			} else {
				log.Debug().Str("extension", ext.UniqueID).Str("version", ext.Version).Msg("parsed")
			}
		}
	}
	return extensions, scanner.Err()
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
	ext, err := vscode.NewExtension(pe.UniqueID)
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
	ext, err := vscode.NewExtension(extReq.UniqueID)
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

func DownloadExtensions(extensions []ExtensionRequest, db *database.DB) (downloadCount int, errorCount int) {
	for _, pe := range extensions {
		extStart := time.Now()
		extension, err := pe.Download()
		if err != nil {
			// if errors.Is(err, ErrVersionNotFound) || errors.Is(err, vscode.ErrExtensionNotFound) {
			log.Error().Str("unique_id", pe.UniqueID).Str("version", pe.Version).Err(err).Msg("unexpected error occured while syncing")
			errorCount++
			continue
		}
		downloadCount++
		if err := db.SaveExtensionMetadata(extension); err != nil {
			log.Err(err).Msg("could not save extension to database")
		}
		for _, v := range extension.Versions {
			if err := db.SaveVersionMetadata(extension, v); err != nil {
				log.Err(err).Msg("could not save version to database")
			}
			for _, a := range v.Files {
				b, err := a.Download()
				if err != nil {
					log.Err(err).Str("source", a.Source).Msg("download failed")
					err = db.Rollback(extension, v)
					if err != nil {
						log.Err(err).Msg("rollback failed")
					}
				}
				if err := db.SaveAssetFile(extension, v, a, b); err != nil {
					log.Err(err).Str("source", a.Source).Msg("could not save asset file")
					err = db.Rollback(extension, v)
					if err != nil {
						log.Err(err).Msg("rollback failed")
					}
				}
			}
		}
		log.Debug().Str("unique_id", pe.UniqueID).Str("version", pe.Version).Msgf("sync took %.3fs", time.Since(extStart).Seconds())
	}
	return
}
