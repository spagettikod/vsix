package cmd

import (
	"fmt"
	"slices"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spf13/cobra"
)

func init() {
	dbAddCmd.Flags().StringSliceVar(&targetPlatforms, "platforms", []string{}, "comma-separated list to limit which target platforms to add")
	dbAddCmd.Flags().BoolVar(&preRelease, "pre-release", false, "include pre-release versions, these are skipped by default")
	dbAddCmd.Flags().BoolVar(&force, "force", false, "download extension eventhough it already exists in database")
	dbCmd.AddCommand(dbAddCmd)
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

		for _, arg := range args {
			existingExtensions := db.FindByUniqueID(true, arg)
			if len(existingExtensions) > 0 && !force {
				fmt.Printf("extension with unique id '%v' already exist in the database\n", arg)
				continue
			}

			er := marketplace.ExtensionRequest{
				UniqueID:        arg,
				TargetPlatforms: targetPlatforms,
				PreRelease:      preRelease,
			}
			_, err := FetchExtension(er, db, force, []string{er.UniqueID}, "add")
			if err != nil {
				log.Fatal().Err(err).Str("database_root", dbPath).Msg("error occured while fetching extension")
			}
		}

		logger.Debug().Msgf("total time for add %.3fs", time.Since(start).Seconds())
	},
}

// FetchExtension downloads the extension given in the extension request from Visual Studio Code Marketplace.
// When downloaded it is added to the database and can be served using the serve command. Besides errors
// it returns false if the extension version already exists and no download occured. Otherwise it returns true.
func FetchExtension(req marketplace.ExtensionRequest, db *database.DB, force bool, stack []string, compontent string) (bool, error) {
	downloadOccurred := false
	elog := log.With().Str("unique_id", req.UniqueID).Str("component", compontent).Logger()
	start := time.Now()

	extension, err := req.Download(preRelease)
	if err != nil {
		return false, err
	}

	if extension.IsExtensionPack() {
		elog.Info().Msg("is extension pack, getting pack contents")
		for _, itemUniqueID := range extension.ExtensionPack() {
			if slices.Contains(stack, itemUniqueID) {
				elog.Warn().Msg("circular extension pack reference, skipping to avoid infinite loop")
				continue
			}
			itemRequest := marketplace.ExtensionRequest{
				UniqueID:        itemUniqueID,
				TargetPlatforms: targetPlatforms,
				PreRelease:      preRelease,
			}
			downloadOccurred, err = FetchExtension(itemRequest, db, force, append(stack, itemUniqueID), compontent)
			if err != nil {
				return false, err
			}
		}
	}

	if err := db.SaveExtensionMetadata(extension); err != nil {
		return false, err
	}
	elog.Debug().Msgf("extension has %v matching versions", len(extension.Versions))
	for _, version := range extension.Versions {
		vlog := elog.With().Str("version", version.Version).Str("version_id", version.ID()).Str("target_platform", version.TargetPlatform()).Logger()
		if version.IsPreRelease() && !req.PreRelease && req.Version == "" {
			vlog.Debug().Msg("skipping, version is a pre-release")
			continue
		}
		if !req.ValidTargetPlatform(version) {
			vlog.Debug().Msg("skipping, unwanted target platform")
			continue
		}
		if existingVersion, found := db.GetVersion(extension.UniqueID(), version); found {
			// if the new version is no longer in pre-release state we're replacing
			// it with the new one
			if !(existingVersion.IsPreRelease() && !version.IsPreRelease()) && !force {
				vlog.Debug().Msg("skipping, version already exists")
				continue
			}
		}
		if err := db.SaveVersionMetadata(extension, version); err != nil {
			return false, err
		}
		for _, asset := range version.Files {
			b, err := asset.Download()
			if err != nil {
				vlog.Err(err).Str("source", asset.Source).Msg("download failed")
				if err := db.Rollback(extension, version); err != nil {
					vlog.Err(err).Msg("rollback failed")
					return false, err
				}
				return false, err
			}
			if err := db.SaveAssetFile(extension, version, asset, b); err != nil {
				vlog.Err(err).Str("source", asset.Source).Msg("could not save asset file")
				if err := db.Rollback(extension, version); err != nil {
					vlog.Err(err).Msg("rollback failed")
					return false, err
				}
				return false, err
			}
			downloadOccurred = true
		}
	}
	elog.Debug().Msgf("fetch took %.3fs", time.Since(start).Seconds())
	return downloadOccurred, nil
}
