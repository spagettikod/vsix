package cmd

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spf13/cobra"
)

func init() {
	dbAddCmd.Flags().StringSliceVar(&targetPlatforms, "platforms", []string{}, "comma-separated list of target platforms to sync (Universal is always fetched)")
	dbAddCmd.Flags().BoolVar(&preRelease, "pre-release", false, "sync should fetch pre-release versions")
	dbAddCmd.Flags().BoolVar(&force, "force", false, "download extension eventhough it already exists in database")
	dbCmd.AddCommand(dbAddCmd)
}

var dbAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add extension to the database",
	Long: `Downloads and adds the latest version on the given extension to the database.
Once added, use the sync command to keep the extension up to date with the marketplace.`,
	Example:               "  $ vsix db add redhat.java",
	DisableFlagsInUseLine: true,
	Args:                  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.With().Str("path", dbPath).Logger()
		uniqueID := args[0]
		start := time.Now()
		db, err := database.OpenFs(dbPath, false)
		if err != nil {
			log.Fatal().Err(err).Str("database_root", dbPath).Msg("could not open database")
		}

		existingExtensions := db.FindByUniqueID(true, uniqueID)
		if len(existingExtensions) > 0 && !force {
			fmt.Printf("extension with unique id '%v' already exist in the database\n", uniqueID)
			return
		}

		er := marketplace.ExtensionRequest{
			UniqueID:        uniqueID,
			TargetPlatforms: targetPlatforms,
			PreRelease:      preRelease,
		}
		if err := FetchExtension(er, db, force); err != nil {
			log.Fatal().Err(err).Str("database_root", dbPath).Msg("error occured while fetching extension")
		}

		logger.Debug().Msgf("total time for add %.3fs", time.Since(start).Seconds())
	},
}

func FetchExtension(er marketplace.ExtensionRequest, db *database.DB, force bool) error {
	elog := log.With().Str("unique_id", er.UniqueID).Logger()
	extStart := time.Now()

	extension, err := er.Download(preRelease)
	if err != nil {
		return err
	}

	if extension.IsExtensionPack() {
		elog.Info().Msg("is extension pack, getting pack contents")
		for _, itemUniqueID := range extension.ExtensionPack() {
			itemRequest := marketplace.ExtensionRequest{
				UniqueID:        itemUniqueID,
				TargetPlatforms: targetPlatforms,
				PreRelease:      preRelease,
			}
			if err := FetchExtension(itemRequest, db, force); err != nil {
				return err
			}
		}
	}

	if err := db.SaveExtensionMetadata(extension); err != nil {
		return err
	}
	for _, version := range extension.Versions {
		vlog := elog.With().Str("version", version.Version).Str("target_platform", version.RawTargetPlatform).Logger()
		if version.IsPreRelease() && !er.PreRelease && er.Version == "" {
			vlog.Debug().Msg("skipping, version is a pre-release")
			continue
		}
		if !er.ValidTargetPlatform(version) {
			vlog.Debug().Msg("skipping, unwanted target platform")
			continue
		}
		if existingVersion, found := db.GetVersion(extension.UniqueID(), version); found {
			// if the new version is no longer in pre-release state we're replacing
			// it with the new one
			if !(existingVersion.IsPreRelease() && !version.IsPreRelease()) && !force {
				vlog.Debug().Msg("skipping, version already exists")
				return nil
			}
		}
		if err := db.SaveVersionMetadata(extension, version); err != nil {
			return err
		}
		for _, asset := range version.Files {
			b, err := asset.Download()
			if err != nil {
				vlog.Err(err).Str("source", asset.Source).Msg("download failed")
				if err := db.Rollback(extension, version); err != nil {
					vlog.Err(err).Msg("rollback failed")
					return err
				}
				return err
			}
			if err := db.SaveAssetFile(extension, version, asset, b); err != nil {
				vlog.Err(err).Str("source", asset.Source).Msg("could not save asset file")
				if err := db.Rollback(extension, version); err != nil {
					vlog.Err(err).Msg("rollback failed")
					return err
				}
				return err
			}
		}
	}
	elog.Debug().Msgf("fetch took %.3fs", time.Since(extStart).Seconds())
	return nil
}
