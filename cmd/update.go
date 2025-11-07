package cmd

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/spagettikod/vsix/cli"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/storage"
	"github.com/spf13/cobra"
)

func init() {
	updateCmd.Flags().BoolVar(&preRelease, "pre-release", false, "update should fetch pre-release versions")
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update [flags] [identifier...]",
	Short: "Update extensions in the database to their latest version",
	Long: `Update will download the latest version of all the extensions currently in
the database.

Only the latest version will be downloaded each time update is run. If the extension
has had multiple releases between each run of update those versions will not be
downloaded.

Update will only update those platforms that already exist locally. Use the add-command
to add more platforms to the database.

To only update a limited set of extensions you can list one or more unique identifers
and update will only update those.

Pre-releases
------------
By default update skips extension versions marked as pre-release. If the latest version
is marked as pre-release the command will traverse the list of versions until it
finds the latest version not marked as pre-release. To enable downloading an extension
and selecting the latest version, regardless if marked as pre-release, use the
pre-release-flag.`,
	Example:               `  $ vsix update`,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		argGrp := slog.Group("args", "cmd", "update", "preRelease", preRelease)

		extensionsToUpdate := []marketplace.ExtensionRequest{}
		p := cli.NewProgress(0, "Listing extensions", !(verbose || debug))
		go p.DoWork()
		if len(args) > 0 {
			for _, uid := range argsToUniqueIDOrExit(args) {
				ext, err := cache.FindByUniqueID(uid)
				if err != nil {
					if errors.Is(err, storage.ErrCacheNotFound) {
						slog.Error("could not find extension with given unique id, add it before updating", argGrp)
					} else {
						slog.Error("error occured while looking up extension in cache", "error", err, argGrp)
					}
					os.Exit(1)
				}
				er := marketplace.ExtensionRequest{
					UniqueID:        ext.UniqueID(),
					TargetPlatforms: ext.Platforms(),
					PreRelease:      preRelease,
				}
				extensionsToUpdate = append(extensionsToUpdate, er)
			}
		} else {
			q := storage.NewQuery()
			q.IncludePreRelease = true
			qr, err := cache.Query(q)
			if err != nil {
				slog.Error("error listing extensions from cache", "error", err, argGrp)
				os.Exit(1)
			}
			for _, r := range qr {
				platforms, err := cache.ListPlatforms(r.Tag.UniqueID)
				if err != nil {
					slog.Error("error listing platforms from cache", "error", err, "uniqueID", r.Tag.UniqueID.String(), argGrp)
					os.Exit(1)
				}
				er := marketplace.ExtensionRequest{
					UniqueID:        r.Tag.UniqueID,
					TargetPlatforms: platforms,
					PreRelease:      preRelease,
				}
				extensionsToUpdate = append(extensionsToUpdate, er)
			}
		}
		extensionsToUpdate = marketplace.Deduplicate(extensionsToUpdate)
		if len(extensionsToUpdate) == 0 {
			slog.Error("no extensions to update")
			os.Exit(1)
		}
		p.StopWork()
		p.Max(len(extensionsToUpdate))
		p.Text("Updating")
		CommonFetchAndSave(extensionsToUpdate, start, p, argGrp)
		p.Done()
	},
}
