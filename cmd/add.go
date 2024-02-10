package cmd

import (
	"slices"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spf13/cobra"
)

func init() {
	dbAddCmd.Flags().IntVar(&threads, "threads", 10, "number of simultaneous download threads")
	dbAddCmd.Flags().StringSliceVar(&targetPlatforms, "platforms", []string{}, "comma-separated list to limit which target platforms to add")
	dbAddCmd.Flags().BoolVar(&preRelease, "pre-release", false, "include pre-release versions, these are skipped by default")
	dbAddCmd.Flags().BoolVar(&force, "force", false, "download extension eventhough it already exists in database")
	rootCmd.AddCommand(dbAddCmd)
}

var dbAddCmd = &cobra.Command{
	Use:   "add <identifier...>",
	Short: "Add extension(s) from Marketplace to the database",
	Long: `Downloads the latest version of the given extension(s) from Marketplace to the database.
Once added, use the sync command to keep the extension up to date with Marketplace.

Multiple identifiers, separated by space, can be used to add multiple extensions at once.

Target platforms
----------------
By default all platform versions of an extension is added. You can limit which platforms
to add by using the platforms-flag. This is a comma separated list of platforms. You can
view available platforms for an extension by using the info-command. The
universal-platform is always added, regardless of the platforms-flag.

Pre-releases
------------
By default add skips extension versions marked as pre-release. If the latest version
is marked as pre-release the add-command will traverse the list of versions until it
finds the latest version not marked as pre-release. To enable adding an extension and
selecting the latest version, regardless if marked as pre-release, use the
pre-release-flag.
`,
	Example:               "  $ vsix db add redhat.java",
	DisableFlagsInUseLine: true,
	Args:                  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.With().Str("path", dbPath).Logger()
		start := time.Now()
		db, err := database.OpenFs(dbPath, false)
		if err != nil {
			log.Fatal().Err(err).Str("database_root", dbPath).Msg("could not open database")
		}

		extensionsToAdd := []marketplace.ExtensionRequest{}
		for _, arg := range args {
			er := marketplace.ExtensionRequest{
				UniqueID:        arg,
				TargetPlatforms: targetPlatforms,
				PreRelease:      preRelease,
			}
			if ext, found := db.GetByUniqueID(false, arg); found {
				if slices.Compare(ext.Platforms(), targetPlatforms) == 0 {
					logger.Info().Msgf("extension %v for the given platforms already exists", arg)
					continue
				}
			}
			extensionsToAdd = append(extensionsToAdd, er)
		}
		extensionsToAdd = marketplace.Deduplicate(extensionsToAdd)

		fetchCount, errCount := fetchThreaded(db, extensionsToAdd, threads, logger)
		if errCount > 0 {
			logger.Error().Msgf("%v extensions were added and %v errors occured, command took %.3fs", fetchCount, errCount, time.Since(start).Seconds())
		} else {
			logger.Info().Msgf("%v extensions were added and %v errors occured, command took %.3fs", fetchCount, errCount, time.Since(start).Seconds())
		}
	},
}
