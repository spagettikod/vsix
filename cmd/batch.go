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

	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

func init() {
	batchCmd.Flags().StringVarP(&out, "output", "o", ".", "Output directory for downloaded files")
	rootCmd.AddCommand(batchCmd)
}

var batchCmd = &cobra.Command{
	Use:   "batch <file|dir>",
	Short: "Download multiple packages specified in a input file or files in a directory",
	Long: `Batch will download all the extensions specified in a text file. If a directory is
given as input all text files in that directory (and its sub directories) will be parsed
in search of extensions to download.

Input file example:
  # This is a comment
  # This will download the latest version 
  golang.Go
  # This will download version 0.17.0 of the golang extension
  golang.Go 0.17.0
	
Extensions are downloaded to the current folder unless the output-flag is set.
	
The command will exit with a non zero value if one of the extensions can not be found
or a given version does not exist. These errors will be logged to standard error
output but the execution will not stop.`,
	Example: `  vsix batch my_extensions.txt
  vsix batch -o downloads my_extensions.txt`,
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		extensions, err := findExtensions(args[0])
		if err != nil {
			errLog.Fatal(err)
		}
		if len(extensions) == 0 {
			errLog.Fatalf("no extensions found at path '%s'", args[0])
		}
		loggedErrors := 0
		for _, pe := range extensions {
			err := download(pe, out)
			if err != nil {
				if errors.Is(err, errVersionNotFound) || errors.Is(err, vscode.ErrExtensionNotFound) {
					errLog.Printf("%s: %s\n", pe, err)
					loggedErrors++
				} else {
					errLog.Fatalf("%s: %s", pe, err)
				}
			}
		}
		if loggedErrors > 0 {
			os.Exit(1)
		}
	},
}

type parsedExtension struct {
	UniqueID string
	Version  string
}

func (pe parsedExtension) String() string {
	if pe.Version == "" {
		return fmt.Sprintf("%s", pe.UniqueID)
	}
	return fmt.Sprintf("%s-%s", pe.UniqueID, pe.Version)
}

func isPlainText(data []byte) bool {
	mime := http.DetectContentType(data)
	return strings.Index(mime, "text/plain") == 0
}

func isDir(p string) (bool, error) {
	info, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func parse(data []byte) (extensions []parsedExtension, err error) {
	buf := bytes.NewBuffer(data)
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Index(line, "#") == 0 || len(line) == 0 {
			continue
		}
		splitLine := strings.Split(line, " ")
		ext := parsedExtension{}
		if len(splitLine) > 0 && len(splitLine) < 3 {
			ext.UniqueID = strings.TrimSpace(splitLine[0])
			if len(splitLine) == 2 {
				ext.Version = strings.TrimSpace(splitLine[1])
			}
			extensions = append(extensions, ext)
			if ext.Version == "" {
				verboseLog.Printf("  parsed extension: %s\n", ext.UniqueID)
			} else {
				verboseLog.Printf("  parsed extension: %s, version %s\n", ext.UniqueID, ext.Version)
			}
		}
	}
	return extensions, scanner.Err()
}

func findExtensions(p string) (extensions []parsedExtension, err error) {
	dir, err := isDir(p)
	if err != nil {
		return
	}
	if dir {
		verboseLog.Printf("found directory '%s'\n", p)
		fis, err := ioutil.ReadDir(p)
		if err != nil {
			return extensions, err
		}
		for _, fi := range fis {
			if !fi.IsDir() {
				exts, err := findExtensions(fmt.Sprintf("%s%s%s", p, string(os.PathSeparator), fi.Name()))
				if err != nil {
					return extensions, err
				}
				extensions = append(extensions, exts...)
			}
		}
	} else {
		verboseLog.Printf("found file %s\n", p)
		data, err := ioutil.ReadFile(p)
		if err != nil {
			return extensions, err
		}
		if isPlainText(data) {
			verboseLog.Printf("parsing file %s\n", p)
			exts, err := parse(data)
			if err != nil {
				return extensions, err
			}
			extensions = append(extensions, exts...)
		} else {
			verboseLog.Println("  skipping, not a text file")
		}
	}
	return
}
