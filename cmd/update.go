package cmd

import (
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

func init() {
	updateCmd.Flags().StringVarP(&dbPath, "data", "d", ".", "path where downloaded extensions are stored [VSIX_DB_PATH]")
	updateCmd.Flags().IntVar(&threads, "threads", 3, "number of simultaneous download threads")
	updateCmd.Flags().BoolVar(&preRelease, "pre-release", false, "update should fetch pre-release versions")
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update extensions in the local storage with their latest version",
	Long: `Update will download the latest version of all the extensions currently in
the local storage.

Only the latest version will be downloaded each time update is run. If the extension
has had multiple releases between each run of update those versions will not be
downloaded.

The command will exit with exit code 78 if one of the extensions can not be found
or a given version does not exist. These errors will be logged to stderr
output but the execution will not stop.

Target platforms
----------------
Only the target platforms that exist in the local storage are updated.

Pre-releases
------------
By default update skips extension versions marked as pre-release. If the latest version
is marked as pre-release the command will traverse the list of versions until it
finds the latest version not marked as pre-release. To enable downloading an extension
and selecting the latest version, regardless if marked as pre-release, use the
pre-release-flag.`,
	Example:               `  $ vsix update --data extensions `,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		if threads < 1 {
			fmt.Println("invalid threads value, must be atleast 1 or above")
			os.Exit(1)
		}
		start := time.Now()
		lg := log.With().Str("data_root", dbPath).Str("component", "update").Logger()
		db, err := database.OpenFs(dbPath, false)
		if err != nil {
			lg.Fatal().Err(err).Msg("could not open folder")
		}
		lg.Debug().Msgf("open local extensions took %.3fs", time.Since(start).Seconds())
		// FIXME latest check is broken since new add
		// exts := db.List(true)

		ers := []marketplace.ExtensionRequest{}
		// for _, ext := range exts {
		// vlog := lg.With().Str("unique_id", ext.UniqueID()).Logger()
		// get latest version from Marketplace
		// marketplaceLatestVersion, err := marketplace.LatestVersion(ext.UniqueID(), preRelease)
		// if err != nil {
		// 	vlog.Err(err).Msg("error while fetching latest version from marketplace")
		// 	continue
		// }
		// vlog = vlog.With().Str("unique_id", ext.UniqueID()).Str("local_version", ext.LatestVersion(preRelease)).Str("marketplace_version", marketplaceLatestVersion).Logger()

		// if marketplaceLatestVersion == "" {
		// 	vlog.Error().Msg("could not determine marketplace version, skipping this extension")
		// 	continue
		// }

		// if ext.LatestVersion(preRelease) == marketplaceLatestVersion {
		// 	vlog.Debug().Msg("skipping, already latest version")
		// 	continue
		// } else {
		// 	vlog.Debug().Msg("new version exist, adding to list of items to get")
		// 	er := marketplace.ExtensionRequest{
		// 		UniqueID:        ext.UniqueID(),
		// 		TargetPlatforms: ext.Platforms(),
		// 		PreRelease:      preRelease,
		// 		Force:           force,
		// 	}
		// 	ers = append(ers, er)
		// }
		// }
		fetchCount, errCount := fetchThreaded(db, ers, threads, lg)

		lg = lg.With().Int("downloads", fetchCount).Int("errors", errCount).Logger()
		lg.Info().Msgf("total time for update %.3fs", time.Since(start).Seconds())
		if fetchCount > 0 {
			lg.Debug().Msg("notifying server")
			err = db.Modified()
			if err != nil {
				lg.Fatal().Err(err).Msg("could not notify server of extension update")
			}
		}
		if errCount > 0 {
			os.Exit(78)
		}
	},
}

func fetchThreaded(db *database.DB, extensions []marketplace.ExtensionRequest, threads int, lg zerolog.Logger) (int, int) {
	if len(extensions) == 0 {
		return 0, 0
	}
	maxRunning := threads
	running := 0
	processed := sync.Map{}
	ch := make(chan FetchResult)
	errCount := 0
	fetchCount := 0
	for _, ext := range extensions {
		lg := lg.With().Str("extension_id", ext.UniqueID.String()).Logger()
		if running >= maxRunning {
			lg.Debug().Msg("maximum thread count reached, waiting")
			result := <-ch
			fetchCount += result.Downloads
			running--
		}
		if _, found := processed.Load(ext.UniqueID); found {
			lg.Debug().Msg("already processed, skipping")
			continue
		}
		processed.Store(ext.UniqueID, true)

		running++
		lg = lg.With().Int("thread", running).Logger()
		lg.Debug().Msg("thread started")
		go doFetch(ch, db, ext, lg)
	}
	for result := range ch {
		fetchCount += result.Downloads
		running--
		if running <= 0 {
			break
		}
	}

	return fetchCount, errCount
}

func doFetch(ch chan FetchResult, db *database.DB, er marketplace.ExtensionRequest, lg zerolog.Logger) {
	result := fetchExtension(er, db, []string{er.UniqueID.String()}, "fetch_thread")
	if result.Err != nil {
		lg.Err(result.Err).Msg("error occured while fetching extension")
	}
	lg.Debug().Msg("exiting thread")
	ch <- result
}

type FetchResult struct {
	Downloads int
	Err       error
}

// FetchExtension downloads the extension given in the extension request from Visual Studio Code Marketplace.
// When downloaded it is added to the database and can be served using the serve command. Besides errors
// it returns false if the extension version already exists and no download occured. Otherwise it returns true.
func fetchExtension(req marketplace.ExtensionRequest, db *database.DB, stack []string, compontent string) FetchResult {
	result := FetchResult{0, nil}
	elog := log.With().Str("unique_id", req.UniqueID.String()).Str("component", compontent).Logger()
	start := time.Now()

	extension, err := req.Download(preRelease)
	if err != nil {
		return FetchResult{0, err}
	}

	if extension.IsExtensionPack() {
		elog.Info().Msg("is extension pack, getting pack contents")
		for _, itemUniqueID := range extension.ExtensionPack() {
			if slices.Contains(stack, itemUniqueID) {
				elog.Warn().Msg("circular extension pack reference, skipping to avoid infinite loop")
				continue
			}
			uid, _ := vscode.Parse(itemUniqueID)
			itemRequest := marketplace.ExtensionRequest{
				UniqueID:        uid,
				TargetPlatforms: req.TargetPlatforms,
				PreRelease:      preRelease,
				Force:           req.Force,
			}
			packResult := fetchExtension(itemRequest, db, append(stack, itemUniqueID), compontent)
			result.Downloads += packResult.Downloads
		}
	}

	if err := db.SaveExtensionMetadata(extension); err != nil {
		return FetchResult{0, err}
	}
	elog.Debug().Msgf("extension has %v versions", len(extension.Versions))
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
			return FetchResult{0, err}
		}
		for _, asset := range version.Files {
			b, err := asset.Download()
			if err != nil {
				vlog.Err(err).Str("source", asset.Source).Msg("download failed")
				if err := db.Rollback(extension, version); err != nil {
					vlog.Err(err).Msg("rollback failed")
					return FetchResult{0, err}
				}
				return FetchResult{0, err}
			}
			if err := db.SaveAssetFile(extension, version, asset, b); err != nil {
				vlog.Err(err).Str("source", asset.Source).Msg("could not save asset file")
				if err := db.Rollback(extension, version); err != nil {
					vlog.Err(err).Msg("rollback failed")
					return FetchResult{0, err}
				}
				return FetchResult{0, err}
			}
		}
		vlog.Info().Msgf("version downloaded in %.3fs", time.Since(start).Seconds())
		result.Downloads++
	}
	return result
}

func platformsToAdd(requestedPlatforms []string, versions []vscode.Version) []string {
	existingPlatforms := []string{}
	for _, v := range versions {
		existingPlatforms = append(existingPlatforms, v.TargetPlatform())
	}
	return slices.DeleteFunc(requestedPlatforms, func(p string) bool {
		return slices.Contains(existingPlatforms, p)
	})
}
