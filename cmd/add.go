package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/storage"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

func init() {
	dbAddCmd.Flags().StringVarP(&dbPath, "data", "d", ".", "path where downloaded extensions are stored [VSIX_DB_PATH]")
	dbAddCmd.Flags().StringSliceVar(&targetPlatforms, "platforms", []string{}, "comma-separated list to limit which target platforms to add")
	dbAddCmd.Flags().BoolVar(&preRelease, "pre-release", false, "include pre-release versions, these are skipped by default")
	rootCmd.AddCommand(dbAddCmd)
}

var dbAddCmd = &cobra.Command{
	Use:   "add [flags] <identifier...>",
	Short: "Add extension(s) from Marketplace to local database",
	Long: `Add extension(s) from Marketplace to local database.

Downloads the latest version of the given extension(s) from Marketplace to the local database.
Once added, use the update command to keep the extension up to date with Marketplace. Use
the serve-command to host your own Marketplace with the downloaded extensions.

Multiple identifiers, separated by space, can be used to add multiple extensions at once.

Target platforms
----------------
By default all platform versions of an extension are added. You can limit which platforms
to add by using --platforms, which is a comma separated list of platforms. You can
view available platforms for an extension by using the info-command. When using --platforms
please note that only those platforms given will be downloaded. The default "universal"
platform will not be added unless it is included in the list.

Pre-releases
------------
By default add skips extension versions marked as pre-release. If the latest version
is marked as pre-release the add-command will traverse the list of versions until it
finds the latest version not marked as pre-release. To enable adding an extension and
selecting the latest version, regardless if marked as pre-release, use --pre-release.
`,
	Example: `  Add the Red Hat Java extension:
    $ vsix add --data extensions redhat.java 

  Add 100 most popular extensions, download the latest version that is not a pre-release:
    $ vsix add --data extensions $(vsix search --limit 100 --quiet)

  Add the 100 most popular extensions, use the latest version regardless if it's a pre-release or not:
    $ vsix add --data extensions --pre-release $(vsix search --limit 100 --quiet)
`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		argGrp := slog.Group("args", "cmd", "add", "path", dbPath, "preRelease", preRelease, "targetPlatforms", targetPlatforms)

		db, verrs, err := storage.Open(dbPath)
		if err != nil {
			slog.Error("could not open database, exiting", "error", err, argGrp)
			os.Exit(1)
		}
		printValidationErrors(verrs)

		// loop all args (extension unique identifiers)
		extensionsToAdd := []marketplace.ExtensionRequest{}
		for _, uid := range argsToUniqueIDOrExit(args) {
			er := marketplace.ExtensionRequest{
				UniqueID:        uid,
				TargetPlatforms: targetPlatforms,
				PreRelease:      preRelease,
			}
			extensionsToAdd = append(extensionsToAdd, er)
		}
		extensionsToAdd = marketplace.Deduplicate(extensionsToAdd)

		CommonFetchAndSave(db, extensionsToAdd, start, argGrp)
	},
}

type RequestResult struct {
	VersionsSkipped int
	TotalAssets     int
	Versions        []vscode.Version
}

func CommonFetchAndSave(db *storage.Database, extensionsToAdd []marketplace.ExtensionRequest, start time.Time, argGrp slog.Attr) {
	slog.Info("processing extensions", "extensionsToAdd", len(extensionsToAdd))

	skipped := 0
	matched := 0
	assets := 0
	bar := progressbar.NewOptions(len(extensionsToAdd),
		progressbar.OptionShowCount(),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetVisibility(!(verbose || debug)),
		progressbar.OptionShowDescriptionAtLineEnd(),
		progressbar.OptionSetPredictTime(false),
		progressbar.OptionSetElapsedTime(true),
		progressbar.OptionClearOnFinish(),
	)
	for _, er := range extensionsToAdd {
		extStart := time.Now()
		bar.Describe(er.UniqueID.String() + ": downloading metadata")
		res, err := FetchAndSaveMetadata(db, er)
		if err != nil {
			slog.Error("error fetching extension metadata", "error", err, "uniqueId", er.UniqueID)
			bar.Add(1)
			continue
		}
		skipped += res.VersionsSkipped
		matched += len(res.Versions)
		extAsset := 0
		for _, v := range res.Versions {
			for _, a := range v.Files {
				extAsset++
				bar.Describe(v.Tag(er.UniqueID).String() + fmt.Sprintf(": downloading asset %v of %v", extAsset, res.TotalAssets))
				aGrp := slog.Group("asset", "type", a.Type, "url", a.Source)
				slog.Debug("saving asset", aGrp, argGrp)
				size, err := FetchAndSaveAsset(db, v.Tag(er.UniqueID), a)
				if err != nil {
					slog.Error("error saving asset, continuing with next asset", "error", err, aGrp, argGrp)
					continue
				}
				slog.Debug("asset downloaded", "contentLength", size)
				assets++
			}
		}
		slog.Info("extension processed", slog.Group("extension", "uniqueId", er.UniqueID.String()), "elapsedTime", time.Since(extStart).Round(time.Millisecond), argGrp)
		bar.Add(1)
	}
	statusGrp := slog.Group("versions", "found", matched+skipped, "matched", matched, "skipped", skipped, "downloadedAssets", assets)
	slog.Info("done", "elapsedTime", time.Since(start).Round(time.Millisecond), statusGrp, argGrp)
}

func FetchAndSaveMetadata(db *storage.Database, request marketplace.ExtensionRequest) (RequestResult, error) {
	res := RequestResult{}

	// fetch metadata for latest version
	ext, err := marketplace.LatestVersion(request.UniqueID, request.PreRelease)
	if err != nil {
		return res, err
	}

	if err := db.SaveExtensionMetadata(ext); err != nil {
		return res, fmt.Errorf("error saving extension metadata: %w", err)
	}
	for _, v := range ext.Versions {
		// skip if this version exists
		tag := v.Tag(request.UniqueID)
		tag.PreRelease = request.PreRelease
		_, exists := db.FindByVersionTag(tag)
		if exists {
			res.VersionsSkipped++
			continue
		}
		// save version metadata and add it to the list of versions to download if it matches the request
		if request.Matches(v.Tag(request.UniqueID)) && v.Version == ext.LatestVersion(request.PreRelease) {
			if err := db.SaveVersionMetadata(request.UniqueID, v); err != nil {
				return res, fmt.Errorf("error saving version metadata: %w", err)
			}
			res.TotalAssets += len(v.Files)
			res.Versions = append(res.Versions, v)
		} else {
			res.VersionsSkipped++
		}
	}
	return res, nil
}

func FetchAndSaveAsset(db *storage.Database, tag vscode.VersionTag, asset vscode.Asset) (int64, error) {
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

func argsToUniqueIDOrExit(args []string) []vscode.UniqueID {
	uids := []vscode.UniqueID{}
	for _, arg := range args {
		uid, ok := vscode.Parse(arg)
		if !ok {
			slog.Error("invalid unique id, exiting", "uniqueId", arg)
			os.Exit(1)
		}
		uids = append(uids, uid)
	}
	return uids
}

func printValidationErrors(verrs []storage.ValidationError) {
	for _, verr := range verrs {
		slog.Warn("invalid extension, run prune to fix", "version_tag", verr.Tag.String(), "validation_error", verr.Error)
	}
}
