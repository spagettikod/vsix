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
	syncCmd.Flags().BoolVar(&preRelease, "pre-release", false, "sync should fetch pre-release versions")
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
			extensions[i].PreRelease = preRelease
		}
		extensions = marketplace.Deduplicate(extensions)
		log.Debug().Msgf("found %v extensions to sync in total", len(extensions))
		log.Debug().Msgf("parsing took %.3fs", time.Since(start).Seconds())
		db, err := database.OpenFs(out, false)
		if err != nil {
			log.Fatal().Err(err).Str("database_root", out).Msg("could not open database")
		}
		downloads, loggedErrors := downloadExtensions(extensions, db)
		dllog := log.With().Str("path", out).Int("downloaded_versions", downloads).Int("sync_errors", loggedErrors).Logger()
		dllog.Info().Msgf("total time for sync %.3fs", time.Since(start).Seconds())
		if downloads > 0 {
			dllog.Debug().Msg("notifying database")
			err = db.Modified()
			if err != nil {
				dllog.Fatal().Err(err).Msg("could not notify database of extension update")
			}
		}
		if loggedErrors > 0 {
			os.Exit(78)
		}
	},
}

func downloadExtensions(extensions []marketplace.ExtensionRequest, db *database.DB) (downloadCount int, errorCount int) {
	versionDownloadCounter := map[string]bool{}
	for _, pe := range extensions {
		extStart := time.Now()
		extension, err := pe.Download()
		if err != nil {
			log.Error().Str("unique_id", pe.UniqueID).Str("version", pe.Version).Err(err).Msg("unexpected error occured while syncing")
			errorCount++
			continue
		}
		elog := log.With().Str("unique_id", extension.UniqueID()).Logger()
		if err := db.SaveExtensionMetadata(extension); err != nil {
			elog.Err(err).Msg("could not save extension to database")
		}
		for _, v := range extension.Versions {
			vlog := elog.With().Str("version", v.Version).Str("target_platform", v.TargetPlatform).Logger()
			if v.IsPreRelease() && !pe.PreRelease && pe.Version == "" {
				vlog.Debug().Msg("skipping, version is a pre-release")
				continue
			}
			if !pe.ValidTargetPlatform(v) {
				vlog.Debug().Msg("skipping, unwanted target platform")
				continue
			}
			if existingVersion, found := db.GetVersion(extension.UniqueID(), v); found {
				// if the new version is no longer in pre-relase state we're replacing
				// it with the new one
				if !(existingVersion.IsPreRelease() && !v.IsPreRelease()) {
					vlog.Debug().Msg("skipping, version already exists")
					continue
				}
			}
			if err := db.SaveVersionMetadata(extension, v); err != nil {
				vlog.Err(err).Msg("could not save version to database")
			}
			for _, a := range v.Files {
				b, err := a.Download()
				if err != nil {
					errorCount++
					vlog.Err(err).Str("source", a.Source).Msg("download failed")
					if err := db.Rollback(extension, v); err != nil {
						vlog.Err(err).Msg("rollback failed")
					}
					continue
				}
				if err := db.SaveAssetFile(extension, v, a, b); err != nil {
					vlog.Err(err).Str("source", a.Source).Msg("could not save asset file")
					if err := db.Rollback(extension, v); err != nil {
						vlog.Err(err).Msg("rollback failed")
					}
					continue
				}
				versionDownloadCounter[pe.UniqueID] = true
			}
		}
		downloadCount = len(versionDownloadCounter)
		elog.Debug().Msgf("sync took %.3fs", time.Since(extStart).Seconds())
	}
	return
}
