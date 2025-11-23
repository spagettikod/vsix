package cmd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spagettikod/vsix/cli"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/storage"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

func init() {
	dbAddCmd.Flags().StringSliceVar(&targetPlatforms, "platforms", defaults["VSIX_PLATFORMS"].([]string), "comma-separated list to limit which target platforms to add [VSIX_PLATFORMS]")
	if err := viper.BindPFlag("VSIX_PLATFORMS", dbAddCmd.Flags().Lookup("platforms")); err != nil {
		log.Fatalln(err)
	}
	dbAddCmd.Flags().BoolVar(&preRelease, "pre-release", false, "include pre-release versions, these are skipped by default")
	dbAddCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "don't produce any output except errors")
	rootCmd.AddCommand(dbAddCmd)
}

var dbAddCmd = &cobra.Command{
	Use:   "add [flags] <identifier...>",
	Short: "Add extensions to your local storage",
	Long: `Add extensions to your local storage from the official marketplace.

Add downloads the latest version of the given extension(s) from the official marketplace.
Multiple identifiers, separated by space, can be used to add multiple extensions at once.

Target platforms
----------------
By default all platform versions of an extension are added. You can limit which platforms
to add by using --platforms, which is a comma separated list of platforms. When using
--platforms please note that only those platforms given will be downloaded. The default
"universal" platform will not be added unless it is included in the list.

Pre-releases
------------
By default add skips extension versions marked as pre-release. If the latest version
is marked as pre-release the add-command will traverse the list of versions until it
finds the latest version not marked as pre-release. To enable adding an extension and
selecting the latest version, regardless if marked as pre-release, use --pre-release.
`,
	Example: `  Add the Red Hat Java extension:
    $ vsix add redhat.java 

  Add 100 most popular extensions, download the latest version that is not a pre-release:
    $ vsix add $(vsix search --limit 100 --sort install --quiet)

  Add the 100 most popular extensions, use the latest version regardless if it's a pre-release or not:
    $ vsix add --pre-release $(vsix search --limit 100 --sort install --quiet)
`,
	DisableFlagsInUseLine: true,
	Args:                  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		targetPlatforms := viper.GetStringSlice("VSIX_PLATFORMS")
		argGrp := slog.Group("args", "cmd", "add", "preRelease", preRelease, "targetPlatforms", targetPlatforms)

		p := cli.NewProgress(0, "Starting up", !(verbose || debug || quiet))
		go p.DoWork()
		// loop all args (extension unique identifiers)
		extensionsToAdd := []marketplace.ExtensionRequest{}
		for _, uid := range argsToUniqueIDOrExit(args) {
			er := marketplace.ExtensionRequest{
				UniqueID:        uid,
				TargetPlatforms: targetPlatforms,
				PreRelease:      preRelease,
			}
			// Check to see if an extentsion with that unique identifier and platform
			// combination already exists. If the combination already exist we don't
			// need to call the Marketplace to check if we want it.
			if len(targetPlatforms) > 0 {
				for _, tp := range targetPlatforms {
					exist, err := cache.Exists(uid, tp)
					if err != nil {
						slog.Error("error looking up if platform exists in cache, exiting", "error", err)
						os.Exit(1)
					}
					// found platform, skip calling market place, otherwise add extension to list for look up
					if exist {
						slog.Debug("skipping, extension found in cache", "uid", uid, "targetPlatform", tp)
					} else {
						extensionsToAdd = append(extensionsToAdd, er)
					}
				}
			} else {
				extensionsToAdd = append(extensionsToAdd, er)
			}
		}
		extensionsToAdd = marketplace.Deduplicate(extensionsToAdd)

		p.StopWork()
		p.Max(len(extensionsToAdd))
		p.Text("Adding extensions")
		CommonFetchAndSave(extensionsToAdd, start, p, argGrp)
		p.Done()
	},
}

type RequestResult struct {
	VersionsSkipped int
	TotalAssets     int
	UniqueID        vscode.UniqueID
	Versions        []vscode.Version
}

func CommonFetchAndSave(extensionsToAdd []marketplace.ExtensionRequest, start time.Time, p cli.Progresser, argGrp slog.Attr) {
	slog.Info("processing extensions", "extensionsToAdd", len(extensionsToAdd))

	// Create a buffered channel to limit concurrency to 5
	semaphore := make(chan struct{}, 5)
	// Use a mutex to safely update shared counters
	var mu sync.Mutex
	// Use errgroup to manage parallel execution and error handling
	var g errgroup.Group

	skipped := 0
	matched := 0
	assets := 0
	for _, er := range extensionsToAdd {
		semaphore <- struct{}{}
		g.Go(func() error {
			// Always release semaphore slot when done
			defer func() {
				p.Next()
				<-semaphore
			}()
			s, m, a := FetchExtension(er, argGrp)
			mu.Lock()
			skipped += s
			matched += m
			assets += a
			mu.Unlock()
			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		slog.Error("error fetching extension", "error", err)
		os.Exit(1)
	}
	statusGrp := slog.Group("versions", "found", matched+skipped, "matched", matched, "skipped", skipped, "downloadedAssets", assets)
	slog.Info("done", "elapsedTime", time.Since(start).Round(time.Millisecond), statusGrp, argGrp)
}

func FetchExtension(er marketplace.ExtensionRequest, argGrp slog.Attr) (int, int, int) {
	extStart := time.Now()
	res, err := FetchAndSaveMetadata(er)
	if err != nil {
		slog.Error("error fetching extension metadata", "error", err, "uniqueId", er.UniqueID.String(), argGrp)
		return 0, 0, 0
	}
	skipped := res.VersionsSkipped
	matched := len(res.Versions)
	assets := 0
	for _, v := range res.Versions {
		for _, a := range v.Files {
			aGrp := slog.Group("asset", "type", a.Type, "url", a.URI(v))
			if err := FetchAndSaveAsset(v.Tag(res.UniqueID), v, a); err != nil {
				slog.Error("error saving asset, cleaning up this version and continuing with next version", "error", err, aGrp, argGrp)
				if err := backend.Remove(v.Tag(er.UniqueID)); err != nil {
					slog.Error("failed to remove from backend", "error", err, argGrp)
					os.Exit(1)
				}
				if err := cache.Delete(v.Tag(er.UniqueID)); err != nil {
					slog.Error("failed to remove from cache", "error", err, argGrp)
					os.Exit(1)
				}
				break
			}
			assets++
		}
	}
	slog.Info("extension processed", slog.Group("extension", "uniqueId", res.UniqueID.String()), "elapsedTime", time.Since(extStart).Round(time.Millisecond), argGrp)
	return skipped, matched, assets
}

func FetchAndSaveMetadata(request marketplace.ExtensionRequest) (RequestResult, error) {
	res := RequestResult{}

	// fetch metadata for latest version
	ext, err := marketplace.LatestVersion(request.UniqueID, request.PreRelease)
	if err != nil {
		return res, err
	}

	res.UniqueID = ext.UniqueID()
	slog.Debug("found", "extension", res.UniqueID)
	if err := backend.SaveExtensionMetadata(ext); err != nil {
		return res, fmt.Errorf("error saving extension metadata: %w", err)
	}

	for _, v := range ext.Versions {
		// skip if this version exists
		tag := v.Tag(res.UniqueID)
		tag.PreRelease = request.PreRelease
		_, err := cache.FindByVersionTag(tag)
		if err == nil {
			res.VersionsSkipped++
			slog.Debug("skipping version, found in cache", "tag", v.Tag(res.UniqueID))
			continue
		}
		// return an error if FindByVersionTag returned an error that was not a cache miss
		if !errors.Is(err, storage.ErrCacheNotFound) {
			return res, err
		}
		// save version metadata and add it to the list of versions to download if it matches the request
		if request.Matches(v.Tag(res.UniqueID)) && v.Version == ext.LatestVersion(request.PreRelease) {
			slog.Debug("saving version", "tag", v.Tag(res.UniqueID))
			if err := backend.SaveVersionMetadata(res.UniqueID, v); err != nil {
				return res, fmt.Errorf("error saving version metadata: %w", err)
			}
			res.TotalAssets += len(v.Files)
			res.Versions = append(res.Versions, v)
		} else {
			res.VersionsSkipped++
			slog.Debug("skipping version, doesn't match criteria", "tag", v.Tag(res.UniqueID), "preRelease", v.IsPreRelease())
		}
	}
	slog.Debug("updating extension cache", "extension", res.UniqueID)
	cache.IndexExtension(backend, ext.UniqueID())
	return res, nil
}

func FetchAndSaveAsset(tag vscode.VersionTag, v vscode.Version, asset vscode.Asset) error {
	resp, err := http.Get(asset.URI(v))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status %v but got %v", http.StatusOK, resp.StatusCode)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	ct := resp.Header.Get("Content-type")
	// parse the mime type from Content-type, example: "application/zip; api-version=7.2-preview.1" becomes "application/zip"
	if strings.Index(ct, ";") > 0 {
		ct = ct[:strings.Index(ct, ";")]
	}
	slog.Debug("saving asset", "tag", tag.String(), "url", asset.URI(v), "contentType", ct, "size", len(data))
	return backend.SaveAsset(tag, asset.Type, ct, data)
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
