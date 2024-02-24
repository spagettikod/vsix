package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

var dry bool // execute in dry run mode
var rmEmptyExt bool

func init() {
	pruneCmd.Flags().BoolVar(&rmEmptyExt, "rm-empty-ext", false, "remove extensions without any versions")
	pruneCmd.Flags().IntVar(&keep, "keep-versions", 0, "number of versions to keep, 0 keeps all (default 0)")
	pruneCmd.Flags().BoolVar(&dry, "dry", false, "execute command without actually removing anything")
	rootCmd.AddCommand(pruneCmd)
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune the database",
	Long: `Prune the database.

By default, without any flags this command will clean the database from any files
and folders that don't belong there. These files and folders will be removed unless
running with the --dry flag.

If the command finds an extension without any version it will print these giving you
the chance to add versions to the extension with the add-command. If you instead
want these removed you can run the command with the flag --rm-empty-ext.

Pruning versions
================
Adding the --keep-versions X flag will keep the X latest versions and remove the rest.
For example: --keep-versions 2, will remove all versions except the latest two.
`,
	Example:               "  $ vsix db prune --keep-versions 2",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.With().Str("path", dbPath).Str("command", "prune").Logger()
		start := time.Now()

		// begin by cleaning the database
		cleanResult, err := database.CleanDBFiles(dbPath)
		if err != nil {
			logger.Fatal().Err(err).Send()
		}
		if len(cleanResult.Removed) == 0 && len(cleanResult.Optional) == 0 {
			logger.Info().Msg("database did not need any cleanup")
		}
		for _, file := range cleanResult.Removed {
			logger.Info().Msgf("removing file: %s", file)
			if !dry {
				os.RemoveAll(file)
			}
		}
		if len(cleanResult.Optional) > 0 {
			if dry || !rmEmptyExt {
				fmt.Printf("the following extensions did not have any versions, use \"vsix add\" to add versions or run this command again with the flag --rm-empty-ext:\n\n")
				for _, file := range cleanResult.Optional {
					file, _ = filepath.Rel(dbPath, file)
					fmt.Printf("   %s\n", strings.Replace(file, "/", ".", 1))
				}
			}

			if !dry && rmEmptyExt {
				for _, file := range cleanResult.Optional {
					dir := filepath.Dir(file)
					logger.Info().Msgf("removing file: %s", dir)
					os.RemoveAll(dir)
				}
			}
		}

		if keep > 0 {
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
		} else {
			logger.Info().Msg("versions will not be pruned")
		}
		logger.Info().Msgf("total time for prune %.3fs", time.Since(start).Seconds())
	},
}
