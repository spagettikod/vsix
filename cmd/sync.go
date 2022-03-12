package cmd

import (
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spf13/cobra"
)

func init() {
	syncCmd.Flags().StringVarP(&out, "output", "o", ".", "output directory for downloaded files")
	syncCmd.Flags().StringSliceVar(&targetPlatforms, "platforms", []string{}, "comma-seaprated list of target platforms to sync")
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
	
Extensions are downloaded to the current folder unless the output-flag is set.
	
The command will exit with exit code 78 if one of the extensions can not be found
or a given version does not exist. These errors will be logged to stderr
output but the execution will not stop.`,
	Example: `  $ vsix sync -o downloads my_extensions.txt
	
  $ docker run --rm \
	-v downloads:/data \
	-v extensions_to_sync:/extensions_to_sync \
	spagettikod/vsix sync /extensions_to_sync`,
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		extensions, err := marketplace.NewFromFile(args[0])
		if err != nil {
			log.Fatal().Err(err).Str("path", args[0]).Msg("error while reading extension specification files")
		}
		if len(extensions) == 0 {
			log.Fatal().Msgf("no extensions found at path '%s', exiting", args[0])
		}
		for i := range extensions {
			extensions[i].TargetPlatforms = targetPlatforms
		}
		log.Debug().Msgf("parsing took %.3fs", time.Since(start).Seconds())
		db, err := database.OpenFs(out, false)
		if err != nil {
			log.Fatal().Err(err).Str("database_root", out).Msg("could not open database")
		}
		downloads, loggedErrors := marketplace.DownloadExtensions(extensions, db)
		log.Info().Str("path", out).Int("downloads", downloads).Int("download_errors", loggedErrors).Msgf("total time for sync %.3fs", time.Since(start).Seconds())
		if downloads > 0 {
			log.Debug().Str("path", out).Int("downloads", downloads).Int("download_errors", loggedErrors).Msg("notifying database")
			err = db.Modified()
			if err != nil {
				log.Fatal().Err(err).Str("path", out).Int("downloads", downloads).Int("download_errors", loggedErrors).Msg("could not notify database of extension update")
			}
		}
		if loggedErrors > 0 {
			os.Exit(78)
		}
	},
}
