package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/spagettikod/vsix/cmd"
	"github.com/spagettikod/vsix/vscode"
)

var (
	errLog       = log.New(os.Stderr, "", 0)
	infoLog      *log.Logger
	uniqueID     string
	version      string
	verbose      bool
	listVersions bool
	outputPath   string
	inputPath    string
)

func printUsage(msg string) {
	if msg != "" {
		fmt.Println(msg)
	}
	fmt.Println(`
Usage: dlvsix [OPTIONS] {NAME [VERSION]}

Download Visual Studio Extensions.

Options:
  -l			List available versions
  -f PATH		Read extension names from file or folder at path
  -o PATH		Output path
  -v 			Be more talkative`)
}

func init() {
	flag.BoolVar(&verbose, "v", false, "Be more talkative")
	flag.BoolVar(&listVersions, "l", false, "List versions")
	flag.StringVar(&outputPath, "o", ".", "Output path")
	flag.StringVar(&inputPath, "f", "", "Output path")
	flag.CommandLine.Init(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(ioutil.Discard)
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

// func parse(data []byte, destFiles map[string]bool) (extensions []vscode.Extension, err error) {
// 	buf := bytes.NewBuffer(data)
// 	scanner := bufio.NewScanner(buf)
// 	for scanner.Scan() {
// 		line := strings.TrimSpace(scanner.Text())
// 		if strings.Index(line, "#") == 0 || len(line) == 0 {
// 			continue
// 		}
// 		splitLine := strings.Split(line, " ")
// 		ext := vscode.Extension{}
// 		if len(splitLine) > 0 {
// 			ext.UniqueID = splitLine[0]
// 			if len(splitLine) == 2 {
// 				ext.Version = splitLine[1]
// 			}
// 			if destFiles[ext.Filename()] {
// 				infoLog.Printf("  extension '%s' already exist at destination, skipping", ext.String())
// 			} else {
// 				extensions = append(extensions, ext)
// 				infoLog.Printf("  extension '%s' has not been downloaded", ext.String())
// 			}
// 		}
// 	}
// 	return extensions, scanner.Err()
// }

func findExtensions(p string, destFiles map[string]bool) (extensions []vscode.Extension, err error) {
	dir, err := isDir(p)
	if err != nil {
		return
	}
	if dir {
		infoLog.Printf("found directory '%s'\n", p)
		fis, err := ioutil.ReadDir(p)
		if err != nil {
			return extensions, err
		}
		for _, fi := range fis {
			if !fi.IsDir() {
				return findExtensions(fmt.Sprintf("%s%s%s", p, string(os.PathSeparator), fi.Name()), destFiles)
			}
		}
	} else {
		// infoLog.Printf("found file '%s'\n", p)
		// data, err := ioutil.ReadFile(p)
		// if err != nil {
		// 	return extensions, err
		// }
		// if isPlainText(data) {
		// 	exts, err := parse(data, destFiles)
		// 	if err != nil {
		// 		return extensions, err
		// 	}
		// 	extensions = append(extensions, exts...)
		// }
	}

	return
}

func listDestFiles(dest string) (files map[string]bool, err error) {
	if _, e := os.Stat(dest); os.IsNotExist(e) {
		infoLog.Printf("destination directory '%s' not found, creating", dest)
		os.Mkdir(dest, os.ModeDir|os.ModePerm)
	}
	fis, err := ioutil.ReadDir(dest)
	if err != nil {
		return
	}
	files = make(map[string]bool)
	for _, fi := range fis {
		files[fi.Name()] = true
	}
	return
}

func fromPath(inPath, outPath string) {
	destFiles, err := listDestFiles(outputPath)
	if err != nil {
		errLog.Fatal(err)
	}
	extensions, err := findExtensions(inputPath, destFiles)
	if err != nil {
		errLog.Fatal(err)
	}
	if len(extensions) == 0 {
		errLog.Fatalf("no extensions found at path '%s'", inputPath)
	}
}

func main() {
	cmd.Execute()
}
