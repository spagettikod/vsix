package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/vscode"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionsCmd)
}

var versionsCmd = &cobra.Command{
	Use:                   "versions <identifier>",
	Short:                 "List avilable versions for an extension",
	Example:               "vsix versions golang.Go",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info().Str("identifier", args[0]).Msg("looking up extension at Marketplace")
		ext, err := vscode.NewExtension(args[0])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		for _, v := range ext.Versions {
			fmt.Println(v.Version)
		}
	},
}
