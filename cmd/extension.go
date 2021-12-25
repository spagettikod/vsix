package cmd

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

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/vscode"
)

type ExtensionRequest struct {
	UniqueID string
	Version  string
}

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
	return asset.Download(filename)
}

// Download will fetch the extension all its assets making it ready to be
// served by the serve command. It returns true if a download occured and
// false if the requested version already exists at output.
func (pe ExtensionRequest) Download(root string) (bool, error) {
	elog := log.With().Str("extension", pe.UniqueID).Str("dir", root).Logger()

	elog.Debug().Msg("checking if output directory exists")
	if exists, err := outDirExists(root); !exists {
		return false, err
	}

	elog.Debug().Msg("searching for extension at Marketplace")
	ext, err := vscode.NewExtension(pe.UniqueID)
	if err != nil {
		return false, err
	}
	if ext.IsExtensionPack() {
		elog.Info().Msg("is extension pack, getting pack contents")
		for _, pack := range ext.ExtensionPack() {
			erPack := ExtensionRequest{UniqueID: pack}
			_, err := erPack.Download(root)
			if err != nil {
				return false, err
			}
		}
	}
	if pe.Version == "" {
		elog.Debug().Msg("version was not specified, setting to latest version")
		pe.Version = ext.LatestVersion()
	}
	if _, found := ext.Version(pe.Version); !found {
		return false, ErrVersionNotFound
	}
	elog.Debug().Str("version", pe.Version).Msg("found version")
	if exists, err := ext.VersionExists(pe.Version, root); exists || err != nil {
		if exists {
			elog.Info().Str("version", pe.Version).Msg("skipping download, version already exist at output path")
			return false, nil
		}
		return false, err
	}

	// create version directory where files are saved
	versionDir := ext.AbsVersionDir(root, pe.Version)
	elog.Debug().Str("destination", versionDir).Msg("checking if version destination already exists")
	if err := os.MkdirAll(versionDir, os.ModePerm); err != nil {
		return false, err
	}

	assets, _ := ext.Assets(pe.Version)
	for _, asset := range assets {
		// download setting filename to asset type
		filename := path.Join(versionDir, string(asset.Type))
		elog.Info().
			Str("source", asset.Source).
			Str("destination", filename).
			Msg("downloading")
		if err := asset.Download(filename); err != nil {
			return false, err
		}
	}

	elog.Debug().
		Str("destination", ext.AbsVersionDir(root, pe.Version)).
		Msg("saving version metadata")
	version, found := ext.Version(pe.Version)
	if !found {
		return false, fmt.Errorf("error while saving version metadata %w", vscode.ErrVersionNotFound)
	}
	if err := version.SaveMetadata(ext.AbsVersionDir(root, pe.Version)); err != nil {
		return false, err
	}

	elog.Debug().
		Str("destination", ext.AbsMetadataFile(root)).
		Msg("saving extension metadata")
	return true, ext.SaveMetadata(root)
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
