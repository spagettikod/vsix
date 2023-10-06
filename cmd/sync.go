package cmd

import (
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spf13/cobra"
)

func init() {
	syncCmd.Flags().StringSliceVar(&targetPlatforms, "platforms", []string{}, "comma-separated list of target platforms to sync")
	syncCmd.Flags().BoolVar(&preRelease, "pre-release", false, "sync should fetch pre-release versions")
	dbCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Update extensions in the database with their latest version",
	Long: `Sync will download the latest version of all the extensions currently in
the database.

Only the latest version will be downloaded each time sync is run. If the extension
has had multiple releases between each run of sync those versions will not be
downloaded.

The command will exit with exit code 78 if one of the extensions can not be found
or a given version does not exist. These errors will be logged to stderr
output but the execution will not stop.

Target platforms
----------------
By default all platform versions of an extension is synced. You can limit which platforms
to sync by using the platforms-flag. This is a comma separated list of platforms. You can
view available platforms for an extension by using the info-command. The
universal-platform is always added, regardless of the platforms-flag.

Pre-releases
------------
By default sync skips extension versions marked as pre-release. If the latest version
is marked as pre-release the command will traverse the list of versions until it
finds the latest version not marked as pre-release. To enable downloading an extension
and selecting the latest version, regardless if marked as pre-release, use the
pre-release-flag.`,
	Example: `  $ vsix sync -d downloads
	
  $ docker run --rm \
	-v downloads:/data \
	-w /data \
	spagettikod/vsix sync`,
	// Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		lg := log.With().Str("database_root", dbPath).Str("component", "sync").Logger()
		db, err := database.OpenFs(dbPath, false)
		if err != nil {
			lg.Fatal().Err(err).Msg("could not open database")
		}
		lg.Debug().Msgf("open database took %.3fs", time.Since(start).Seconds())
		exts := db.List(true)

		// maxRunning := 20
		// running := 0
		processed := sync.Map{}
		// ch := make(chan string)
		errCount := 0
		fetchCount := 0
		for _, ext := range exts {
			lg = log.With().Str("extension_id", ext.UniqueID()).Logger()
			// if ext.IsExtensionPack() {
			// 	fmt.Println("skipping, is pack", ext.UniqueID())
			// 	continue
			// }
			// if running >= maxRunning {
			// 	fmt.Println("waiting", ext.UniqueID())
			// 	<-ch
			// 	running--
			// }
			if _, found := processed.Load(ext.UniqueID()); found {
				lg.Debug().Msg("already processed, skipping")
				continue
			}
			// fmt.Println("running", ext.UniqueID())
			processed.Store(ext.UniqueID(), true)

			// running++
			// go func(inch chan string, e vscode.Extension) {
			er := marketplace.ExtensionRequest{
				UniqueID: ext.UniqueID(),
				// UniqueID:        e.UniqueID(),
				TargetPlatforms: targetPlatforms,
				PreRelease:      preRelease,
			}
			fetched, err := FetchExtension(er, db, force, []string{er.UniqueID}, "sync")
			if err != nil {
				lg.Err(err).Msg("error occured while fetching extension")
				errCount++
				continue
			}
			if fetched {
				fetchCount++
			}
			// fmt.Println("exiting", e.UniqueID())
			// 	inch <- e.UniqueID()
			// }(ch, ext)
		}
		// <-ch --skip this?
		// fmt.Println("done", <-ch)

		// handle extension packs to avoid writing to same
		// for _, ext := range exts {
		// 	if ext.IsExtensionPack() {
		// 		processed.Store(ext.UniqueID(), true)
		// 		fmt.Println(ext.UniqueID())
		// 	}
		// }
		// for _, ext := range exts {
		// 	if ext.IsExtensionPack() {
		// 		continue
		// 	}
		// 	fmt.Println(ext.UniqueID())
		// }
		// downloads, loggedErrors := downloadExtensions(extensions, targetPlatforms, preRelease, db)
		dllog := lg.With().Int("downloads", fetchCount).Int("errors", errCount).Logger()
		dllog.Info().Msgf("total time for sync %.3fs", time.Since(start).Seconds())
		if fetchCount > 0 {
			dllog.Debug().Msg("notifying database")
			err = db.Modified()
			if err != nil {
				dllog.Fatal().Err(err).Msg("could not notify database of extension update")
			}
		}
		if errCount > 0 {
			os.Exit(78)
		}
	},
}
