package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/vscode"

	"github.com/spf13/cobra"
)

func init() {
	infoCmd.Flags().BoolVar(&preRelease, "pre-release", false, "include pre-release versions")
	rootCmd.AddCommand(infoCmd)
}

var infoCmd = &cobra.Command{
	Use:   "info <identifier>",
	Short: "Display package information from Marketplace",
	Long: `Display package information from Marketplace.

This command displays much of the same basic information about
an extension that can be found at Marketplace. 

Extension pack
--------------
If the extension is an extensions pack this section will show
which extentions the pack includes.
`,
	Example:               "  $ vsix info golang.Go",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		argGrp := slog.Group("args", "cmd", "info", "preRelease", preRelease)
		uid, ok := vscode.Parse(args[0])
		if !ok {
			slog.Error("invalid unique identifier", argGrp)
		}
		ext, err := marketplace.LatestVersion(uid, preRelease)
		if err != nil {
			slog.Error("error fetching latest version from Marketplace", "error", err, argGrp)
			os.Exit(1)
		}
		s := `Name:                 %s
Publisher:            %s
Latest version:       %s
Pre-relase version:   %v
Target platforms:     %s
Released on:          %s
Last updated:         %s
Extension pack:       %s

%s

`
		version, _ := ext.Version(ext.LatestVersion(preRelease))
		targetPlatforms := []string{}
		for _, v := range version {
			targetPlatforms = append(targetPlatforms, v.TargetPlatform())
		}
		slices.Sort(targetPlatforms)
		fmt.Printf(s,
			ext.Name,
			ext.Publisher.DisplayName,
			ext.LatestVersion(preRelease),
			version[0].IsPreRelease(),
			strings.Join(targetPlatforms, "\n                      "),
			ext.ReleaseDate.Format("2006-01-02 15:04 UTC"),
			ext.LastUpdated.Format("2006-01-02 15:04 UTC"),
			strings.Join(ext.ExtensionPack(), "\n                      "),
			ext.ShortDescription)
	},
}
