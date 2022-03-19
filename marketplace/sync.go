package marketplace

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
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
