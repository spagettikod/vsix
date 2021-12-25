package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

func init() {
	syncCmd.Flags().StringVarP(&out, "output", "o", ".", "output directory for downloaded files")
	rootCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:   "sync [flags] <file|dir>",
	Short: "Download packages to be served by the serve command",
	Long: `Sync will download all the extensions specified in a text file. If a directory is
given as input all text files in that directory (and its sub directories) will be parsed
in search of extensions to download.

Input file example:
  # This is a comment
  # This will download the latest version 
  golang.Go
  # This will download version 0.17.0 of the golang extension
  golang.Go 0.17.0
	
Extensions are downloaded to the current folder unless the output-flag is set. It 
returns number of extensions that were downloaded to stdout.
	
The command will exit with exit code 78 if one of the extensions can not be found
or a given version does not exist. These errors will be logged to standard error
output but the execution will not stop.`,
	Example: `  vsix sync my_extensions.txt
  vsix sync -o downloads my_extensions.txt`,
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		extensions, err := NewFromFile(args[0])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if len(extensions) == 0 {
			fmt.Printf("no extensions found at path '%s', exiting\n", args[0])
			os.Exit(1)
		}
		log.Debug().Msgf("parsing took %.3fs", time.Since(start).Seconds())
		loggedErrors := 0
		downloads := 0
		for _, pe := range extensions {
			extStart := time.Now()
			success, err := pe.Download(out)
			if err != nil {
				if errors.Is(err, ErrVersionNotFound) || errors.Is(err, vscode.ErrExtensionNotFound) {
					log.Error().
						Str("unique_id", pe.UniqueID).
						Str("version", pe.Version).
						Msg(err.Error())
					loggedErrors++
				} else {
					fmt.Printf("%s: %s\n", pe, err)
					os.Exit(1)
				}
			}
			if success {
				downloads++
			}
			log.Debug().
				Str("unique_id", pe.UniqueID).
				Str("version", pe.Version).
				Msgf("sync took %.3fs", time.Since(extStart).Seconds())
		}
		log.Info().Msgf("total time for sync %.3fs", time.Since(start).Seconds())
		fmt.Println(downloads)
		if loggedErrors > 0 {
			os.Exit(78)
		}
	},
}
