package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/storage"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

func init() {
	dbAddCmd.Flags().StringVarP(&dbPath, "data", "d", ".", "path where downloaded extensions are stored [VSIX_DB_PATH]")
	// FIXME disable this for now
	// dbAddCmd.Flags().IntVar(&threads, "threads", 10, "number of simultaneous download threads")
	dbAddCmd.Flags().StringSliceVar(&targetPlatforms, "platforms", []string{}, "comma-separated list to limit which target platforms to add")
	dbAddCmd.Flags().BoolVar(&preRelease, "pre-release", false, "include pre-release versions, these are skipped by default")
	rootCmd.AddCommand(dbAddCmd)
}

var dbAddCmd = &cobra.Command{
	Use:   "add <identifier...>",
	Short: "Add extension(s) from Marketplace to local storage",
	Long: `Add extension(s) from Marketplace to local storage

Downloads the latest version of the given extension(s) from Marketplace to local storage.
Once added, use the update command to keep the extension up to date with Marketplace. Use
the serve-command to host your own Marketplace with the downloaded extensions.

Multiple identifiers, separated by space, can be used to add multiple extensions at once.

Target platforms
----------------
By default all platform versions of an extension are added. You can limit which platforms
to add by using the platforms-flag. This is a comma separated list of platforms. You can
view available platforms for an extension by using the info-command. Please not that,
if only those platforms given will be downloaded. The default "universal" platform will
not be added is not included in the list.

Known platforms:
  * darwin-arm64
  * darwin-x64
  * linux-arm64
  * linux-x64
  * universal
  * web
  * win32-x64

Pre-releases
------------
By default add skips extension versions marked as pre-release. If the latest version
is marked as pre-release the add-command will traverse the list of versions until it
finds the latest version not marked as pre-release. To enable adding an extension and
selecting the latest version, regardless if marked as pre-release, use the
pre-release-flag.
`,
	Example: `  Add Java extension
    $ vsix add --data extensions redhat.java 

  Add 100 most popular extensions
    $ vsix add --data extensions $(vsix search --limit 100)
`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		argGrp := slog.Group("args", "cmd", "add", "path", dbPath, "preRelease", preRelease, "targetPlatforms", targetPlatforms)

		db, err := storage.OpenFs(dbPath, false)
		if err != nil {
			slog.Error("could not open database, exiting", "error", err, argGrp)
			os.Exit(1)
		}

		// loop all args (extension unique identifiers)
		extensionsToAdd := []marketplace.ExtensionRequest{}
		for _, arg := range args {
			uid, ok := vscode.Parse(arg)
			if !ok {
				slog.Error("invalid unique id, exiting", "uniqueId", arg)
				os.Exit(1)
				return
			}
			er := marketplace.ExtensionRequest{
				UniqueID:        uid,
				TargetPlatforms: targetPlatforms,
				PreRelease:      preRelease,
			}
			extensionsToAdd = append(extensionsToAdd, er)
		}
		extensionsToAdd = marketplace.Deduplicate(extensionsToAdd)

		slog.Info("processing extensions", "extensionsToAdd", len(extensionsToAdd))

		total := 0
		skipped := 0
		failures := 0
		matched := 0
		assets := 0
		for _, er := range extensionsToAdd {
			ext, err := marketplace.LatestVersion(er.UniqueID, er.PreRelease)
			if err != nil {
				slog.Error("error while fetching latest version", "uniqueId", er.UniqueID)
				return
			}
			total = total + len(ext.Versions)
			latestVersionNumber := ext.LatestVersion(preRelease)
			for _, v := range ext.Versions {
				extGrp := slog.Group("extension", "uniqueId", ext.UniqueID(), "version", v.Version, "targetPlatform", v.TargetPlatform(), "preRelease", v.IsPreRelease())
				if v.Version != latestVersionNumber {
					slog.Debug("skipping version", extGrp, argGrp)
					skipped++
					continue
				}
				if targetPlatformMatches(er.TargetPlatforms, v) { // if universal was given in command and the target platform is not included in the version json
					if v.IsPreRelease() && !er.PreRelease {
						slog.Debug("skipping version", extGrp, argGrp)
						skipped++
						continue
					}
					slog.Debug("version matched", extGrp, argGrp)
					if err := db.SaveExtensionMetadata(ext); err != nil {
						slog.Error("error saving extension metadata, continuing with next extension", "error", err, extGrp, argGrp)
						failures++
						continue
					}
					if err := db.SaveVersionMetadata(er.UniqueID, v); err != nil {
						slog.Error("error saving version metadata, continuing with next extension", "error", err, extGrp, argGrp)
						failures++
						continue
					}
					for _, a := range v.Files {
						aGrp := slog.Group("asset", "type", a.Type, "url", a.Source)
						slog.Debug("saving asset", aGrp, extGrp, argGrp)
						size, err := FetchAndSaveAsset(db, v.Tag(er.UniqueID), a)
						if err != nil {
							slog.Error("error saving asset, continuing with next asset", "error", err, aGrp, extGrp, argGrp)
							failures++
							continue
						}
						slog.Debug("asset downloaded", "contentLength", size)
						assets++
					}
					matched++
				} else {
					slog.Debug("skipping version", extGrp, argGrp)
					skipped++
				}
			}
			if matched == 0 {
				slog.Warn("no extension versions found matching given parameters", slog.Group("extension", "uniqueId", ext.UniqueID()), argGrp)
			}
			slog.Info("extension processed", slog.Group("extension", "uniqueId", ext.UniqueID()), "elapsedTime", time.Since(start).Round(time.Millisecond), argGrp)
		}
		statusGrp := slog.Group("versions", "matched", matched, "failed", failures, "skipped", skipped, "downloadedAssets", assets)
		slog.Info("finished add", "elapsedTime", time.Since(start).Round(time.Millisecond), statusGrp, argGrp)
	},
}

func FetchAndSaveAsset(db *storage.Database, tag vscode.ExtensionTag, asset vscode.Asset) (int64, error) {
	resp, err := http.Get(asset.Source)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("expected status %v but got %v", http.StatusOK, resp.StatusCode)
	}
	defer resp.Body.Close()
	slen := resp.Header.Get("Content-length")
	ilen, _ := strconv.ParseInt(slen, 10, 64)
	return ilen, db.SaveAsset(tag, asset.Type, resp.Body)
}

func targetPlatformMatches(requested []string, version vscode.Version) bool {
	return len(requested) == 0 || // no target platform given, matches all platforms
		slices.Contains(requested, version.RawTargetPlatform) || // is the specific platform given in the command, universal will not be matched as it is not included in the version json (see next condition)
		(slices.Contains(requested, "universal") && version.RawTargetPlatform == "") // if universal was given in command and the target platform is not included in the version json
}
