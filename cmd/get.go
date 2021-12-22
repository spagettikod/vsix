package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	getCmd.Flags().StringVarP(&out, "output", "o", ".", "output directory for downloaded files")
	getCmd.Flags().BoolVarP(&forceget, "force", "f", false, "force download eventhough the version already exists in output folder")
	rootCmd.AddCommand(getCmd)
}

var getCmd = &cobra.Command{
	Use:   "get [flags] <identifier> [version]",
	Short: "Download extension",
	Long: `Get will download the extension from the Marketplace. Extension identifier
can be found on the Visual Studio Code Marketplace web page for a given extension
where it's called "Unique Identifier". If the extension is a "Extension Pack",
which is a collection of extentions, all those extension will also be downloaded
as well.

If version is not specified the latest version will be downloaded. The extension is
downloaded to the current directory unless the output-flag is set. Download is skipped
if the extension already exists in the output directory.

The command will exit with a non zero value if the extension can not be found or the
given version does not exist.`,
	Example: `  vsix get golang.Go
  vsix get golang.Go 0.17.0
  vsix get -o downloads golang.Go`,
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		pe := ExtensionRequest{UniqueID: args[0]}
		if len(args) == 2 {
			pe.Version = args[1]
		}
		get(pe)
	},
}

func get(pe ExtensionRequest) {
	if err := pe.Download(out); err != nil {
		fmt.Printf("%s: %s\n", pe, err)
		os.Exit(1)
	}
}
