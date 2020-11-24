package cmd

import (
	"io/ioutil"
	"net/http"
	"os"

	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

var outputPath string

func init() {
	getCmd.Flags().StringVarP(&outputPath, "output", "o", ".", "Output directory for downloaded files")
	rootCmd.AddCommand(getCmd)
}

var getCmd = &cobra.Command{
	Use:                   "get [flags] <package> [version]",
	Short:                 "Download a package",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		var ext vscode.Extension
		var version string
		var err error
		if len(args) == 1 {
			ext, err = vscode.Latest(args[0])
			if err != nil {
				errLog.Fatalln(err)
			}
			version = ext.Versions[0].Version
		} else {
			ext, err = vscode.ListVersions(args[0])
			if err != nil {
				errLog.Fatalln(err)
			}
			version = args[1]
		}
		resp, err := http.Get(ext.PackageURL(version))
		if err != nil {
			errLog.Fatalln(err)
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errLog.Fatalln(err)
		}
		if err := ioutil.WriteFile(outputPath+"/"+ext.Filename(version), b, os.ModePerm); err != nil {
			errLog.Fatalln(err)
		}
	},
}
