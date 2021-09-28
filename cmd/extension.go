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
		VerboseLog.Printf("found directory '%s'\n", p)
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

	VerboseLog.Printf("found file %s\n", p)
	data, err := ioutil.ReadFile(p)
	if err != nil {
		return exts, err
	}
	if isPlainText(data) {
		VerboseLog.Printf("parsing file %s\n", p)
		exts, err := parse(data)
		if err != nil {
			return exts, err
		}
	} else {
		VerboseLog.Println("  skipping, not a text file")
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
				VerboseLog.Printf("  parsed extension: %s\n", ext.UniqueID)
			} else {
				VerboseLog.Printf("  parsed extension: %s, version %s\n", ext.UniqueID, ext.Version)
			}
		}
	}
	return extensions, scanner.Err()
}

func (pe ExtensionRequest) Download(outDir string) error {
	if exists, err := outDirExists(outDir); !exists {
		return err
	}
	VerboseLog.Printf("%s: searching for extension at Marketplace", pe)
	ext, err := vscode.NewExtension(pe.UniqueID)
	if err != nil {
		return err
	}
	if ext.IsExtensionPack() {
		VerboseLog.Printf("%s: is extension pack, getting pack contents", pe)
		for _, pack := range ext.ExtensionPack() {
			erPack := ExtensionRequest{UniqueID: pack}
			err := erPack.Download(outDir)
			if err != nil {
				return err
			}
		}
	}
	if pe.Version == "" {
		pe.Version = ext.LatestVersion()
	}
	if !ext.HasVersion(pe.Version) {
		return ErrVersionNotFound
	}
	VerboseLog.Printf("%s: found version %s", pe, pe.Version)
	if exists, err := ext.VersionExists(pe.Version, outDir); !forceget && (exists || err != nil) {
		if exists {
			VerboseLog.Printf("%s: skipping download, version already exist at output path\n", pe)
			return nil
		}
		return err
	}
	assets, _ := ext.Assets(pe.Version)
	for _, asset := range assets {
		VerboseLog.Printf("%s: downloading %s to %s", pe, asset.Source, asset.Abs(outDir))
		if err := asset.Download(outDir); err != nil {
			return err
		}
	}
	VerboseLog.Printf("%s: saving metadata to %s", pe, path.Join(outDir, ext.MetaPath()))
	return ext.SaveMetadata(outDir)
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
