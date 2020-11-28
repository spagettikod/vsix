package cmd

import (
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
			verboseLog.Printf("%s: looking up latest version", args[0])
			ext, err = vscode.Latest(args[0])
			if err != nil {
				errLog.Fatalln(err)
			}
			version = ext.Versions[0].Version
			verboseLog.Printf("%s: found version %s\n", args[0], version)
		} else {
			verboseLog.Printf("%s: looking for version %s", args[0], args[1])
			ext, err = vscode.ListVersions(args[0])
			if err != nil {
				errLog.Fatalln(err)
			}
			for _, v := range ext.Versions {
				if v.Version == args[1] {
					version = v.Version
				}
			}
			if version == "" {
				errLog.Fatalf("%s: could not find version %s", args[0], args[1])
			}
			verboseLog.Printf("%s: found version %s", args[0], args[1])
			version = args[1]
		}
		if err := download(outputPath, version, ext); err != nil {
			errLog.Fatalf("%s: %s", args[0], err)
		}
	},
}
