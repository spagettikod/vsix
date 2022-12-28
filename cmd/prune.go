package cmd

import (
	"sort"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

var dry bool // execute in dry run mode

func init() {
	pruneCmd.Flags().IntVar(&keep, "keep", 1, "number of versions to keep")
	pruneCmd.Flags().BoolVar(&dry, "dry", false, "execute command without actually removing anything")
	dbCmd.AddCommand(pruneCmd)
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove extension versions from the database",
	Long: `Remove extension versions from the database.

By default it removes all but the latest version. Use the --keep flag to keep
more than only the latest version.`,
	Example:               "  $ vsix db prune --keep 2",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.With().Str("path", dbPath).Logger()
		start := time.Now()
		db, err := database.OpenFs(dbPath, false)
		if err != nil {
			log.Fatal().Err(err).Str("database_root", dbPath).Msg("could not open database")
		}
		exts := db.List(false)
		for _, ext := range exts {
			versions := ext.Versions
			// filter out the top level versions if multi platform, we
			// assume all platform version of a top level version are release
			// approximately at the same time so we disregard the release date
			if ext.IsMultiPlatform(true) {
				logger.Debug().Str("extension", ext.UniqueID()).Msg("multi platform extension, cleaning away platform versions and keeping top level version")
				vermap := map[string]vscode.Version{}
				for _, v := range versions {
					vermap[v.Version] = v
				}
				versions = nil
				for _, v := range vermap {
					logger.Debug().Str("extension", ext.UniqueID()).Str("version", v.Version).Msg("keeping top level version")
					versions = append(versions, v)
				}
			}
			if len(versions) <= keep {
				logger.Info().Str("extension", ext.UniqueID()).Msgf("skipping prune, too few versions")
				continue
			}
			// sort by updated date, latest first
			sort.Slice(versions, func(i, j int) bool {
				return versions[i].LastUpdated.After(versions[j].LastUpdated)
			})
			for _, v := range versions[:keep] {
				logger.Info().Str("extension", ext.UniqueID()).Str("version", v.Version).Msg("keeping version")
			}
			for _, v := range versions[keep:] {
				if dry {
					logger.Info().Str("extension", ext.UniqueID()).Str("version", v.Version).Msg("would be removed without dry run")
				} else {
					err := db.DeleteVersion(ext, v)
					if err != nil {
						logger.Err(err).Send()
					}
				}
			}
		}
		logger.Info().Msgf("total time for prune %.3fs", time.Since(start).Seconds())
	},
}
