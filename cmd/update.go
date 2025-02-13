package cmd

import (
	"log/slog"
	"os"
	"time"

	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/storage"
	"github.com/spf13/cobra"
)

func init() {
	updateCmd.Flags().StringVarP(&dbPath, "data", "d", ".", "path where downloaded extensions are stored [VSIX_DB_PATH]")
	updateCmd.Flags().BoolVar(&preRelease, "pre-release", false, "update should fetch pre-release versions")
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update [flags] [identifier...]",
	Short: "Update extensions in the local storage with their latest version",
	Long: `Update will download the latest version of all the extensions currently in
the local storage.

Only the latest version will be downloaded each time update is run. If the extension
has had multiple releases between each run of update those versions will not be
downloaded.

Update will only update those platforms that already exist locally.

To only update a limited set of extensions you can list one or more unique identifers
and update will only update those.

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
		start := time.Now()
		argGrp := slog.Group("args", "cmd", "update", "path", dbPath, "preRelease", preRelease)

		db, err := storage.OpenFs(dbPath)
		if err != nil {
			slog.Error("could not open database, exiting", "error", err, argGrp)
			os.Exit(1)
		}

		extensionsToAdd := []marketplace.ExtensionRequest{}
		if len(args) > 0 {
			for _, uid := range argsToUniqueIDOrExit(args) {
				ext, found := db.FindByUniqueID(uid)
				if !found {
					slog.Error("could not find extension with given unique id, add it before updating", argGrp)
					os.Exit(1)
				}
				er := marketplace.ExtensionRequest{
					UniqueID:        ext.UniqueID(),
					TargetPlatforms: ext.Platforms(),
					PreRelease:      preRelease,
				}
				extensionsToAdd = append(extensionsToAdd, er)
			}
		} else {
			for _, ext := range db.List() {
				er := marketplace.ExtensionRequest{
					UniqueID:        ext.UniqueID(),
					TargetPlatforms: ext.Platforms(),
					PreRelease:      preRelease,
				}
				extensionsToAdd = append(extensionsToAdd, er)
			}
		}
		extensionsToAdd = marketplace.Deduplicate(extensionsToAdd)
		if len(extensionsToAdd) == 0 {
			slog.Error("no extensions to update")
			os.Exit(1)
		}

		CommonFetchAndSave(db, extensionsToAdd, start, argGrp)
	},
}
