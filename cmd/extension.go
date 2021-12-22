package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
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
		log.Info().Str("path", p).Msg("found directory")
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
		exts, err := parseFile(p)
		if err != nil {
			return ers, err
		}
		ers = append(ers, exts...)
	}
	return ers, err
}

func parseFile(p string) ([]ExtensionRequest, error) {
	exts := []ExtensionRequest{}

	plog := log.With().Str("path", p).Logger()

	plog.Info().Msg("found file")
	data, err := ioutil.ReadFile(p)
	if err != nil {
		return exts, err
	}
	if isPlainText(data) {
		plog.Info().Msg("parsing file")
		exts, err := parse(data)
		if err != nil {
			return exts, err
		}
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
				log.Info().Str("extension", ext.UniqueID).Msg("parsed")
			} else {
				log.Info().Str("extension", ext.UniqueID).Str("version", ext.Version).Msg("parsed")
			}
		}
	}
	return extensions, scanner.Err()
}

func (pe ExtensionRequest) Download(root string) error {
	if exists, err := outDirExists(root); !exists {
		return err
	}
	elog := log.With().Str("extension", pe.UniqueID).Logger()
	elog.Info().Msg("searching for extension at Marketplace")
	ext, err := vscode.NewExtension(pe.UniqueID)
	if err != nil {
		return err
	}
	if ext.IsExtensionPack() {
		elog.Info().Msg("is extension pack, getting pack contents")
		for _, pack := range ext.ExtensionPack() {
			erPack := ExtensionRequest{UniqueID: pack}
			err := erPack.Download(root)
			if err != nil {
				return err
			}
		}
	}
	if pe.Version == "" {
		pe.Version = ext.LatestVersion()
	}
	if _, found := ext.Version(pe.Version); !found {
		return ErrVersionNotFound
	}
	elog.Info().Str("version", pe.Version).Msg("found version")
	if exists, err := ext.VersionExists(pe.Version, root); !forceget && (exists || err != nil) {
		if exists {
			elog.Info().Str("version", pe.Version).Msg("skipping download, version already exist at output path")
			return nil
		}
		return err
	}

	// create version directory where files are saved
	versionDir := ext.AbsVersionDir(root, pe.Version)
	if err := os.MkdirAll(versionDir, os.ModePerm); err != nil {
		return err
	}

	assets, _ := ext.Assets(pe.Version)
	for _, asset := range assets {
		elog.Info().
			Str("version", pe.Version).
			Str("source", asset.Source).
			Str("destination", versionDir).
			Msg("downloading")
		if err := asset.Download(versionDir); err != nil {
			return err
		}
	}

	elog.Info().
		Str("version", pe.Version).
		Str("destination", ext.AbsVersionDir(root, pe.Version)).
		Msg("saving version metadata")
	version, found := ext.Version(pe.Version)
	if !found {
		return fmt.Errorf("error while saving version metadata %w", vscode.ErrVersionNotFound)
	}
	if err := version.SaveMetadata(ext.AbsVersionDir(root, pe.Version)); err != nil {
		return err
	}

	elog.Info().
		Str("version", pe.Version).
		Str("destination", ext.AbsMetadataFile(root)).
		Msg("saving extension metadata")
	return ext.SaveMetadata(root)
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
