package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spagettikod/vsix/marketplace"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionsCmd)
}

var versionsCmd = &cobra.Command{
	Use:                   "versions <identifier>",
	Short:                 "List available versions at Marketplace for an extension",
	Example:               "  $ vsix versions golang.Go",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		argGrp := slog.Group("args", "cmd", "versions")
		ext, err := marketplace.FetchExtension(args[0])
		if err != nil {
			slog.Error("error fetching latest version from Marketplace", "error", err, argGrp)
			os.Exit(1)
		}
		for _, v := range ext.Versions {
			fmt.Println(v.Version)
		}
	},
}
