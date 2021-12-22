package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

func init() {
	batchCmd.Flags().StringVarP(&out, "output", "o", ".", "output directory for downloaded files")
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
		extensions, err := NewFromFile(args[0])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if len(extensions) == 0 {
			fmt.Printf("no extensions found at path '%s'", args[0])
			os.Exit(1)
		}
		loggedErrors := 0
		for _, pe := range extensions {
			err := pe.Download(out)
			if err != nil {
				if errors.Is(err, ErrVersionNotFound) || errors.Is(err, vscode.ErrExtensionNotFound) {
					log.Error().Msgf("%s: %s\n", pe, err)
					loggedErrors++
				} else {
					fmt.Printf("%s: %s\n", pe, err)
					os.Exit(1)
				}
			}
		}
		if loggedErrors > 0 {
			os.Exit(1)
		}
	},
}
