package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/marketplace"

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
		log.Info().Str("identifier", args[0]).Msg("looking up extension at Marketplace")
		ext, err := marketplace.FetchExtension(args[0])
		if err != nil {
			fmt.Println(err)
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
