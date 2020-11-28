package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/spagettikod/vsix/vscode"
)

func download(outputPath, version string, e vscode.Extension) error {
	_, err := os.Stat(outputPath + "/" + e.Filename(version))
	if !os.IsNotExist(err) {
		return fmt.Errorf("skipping download, version %s already exist", version)
	}
	verboseLog.Printf("%s: downloading version %s", e.Name, version)
	resp, err := http.Get(e.PackageURL(version))
	if err != nil {
		return err
	}
	verboseLog.Printf("%s: finished downloading version %s", e.Name, version)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	verboseLog.Printf("%s: saving to file %s", e.Name, outputPath+"/"+e.Filename(version))
	return ioutil.WriteFile(outputPath+"/"+e.Filename(version), b, os.ModePerm)
}
