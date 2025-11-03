package cmd

import (
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(reindexCmd)
}

var reindexCmd = &cobra.Command{
	Use:     "reindex",
	Aliases: []string{"ri"},
	Short:   "Index items in the backend storage",
	Long: `Remove items from database.
`,
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		argGrp := slog.Group("args", "cmd", "reindex")
		start := time.Now()

		// recreate the cache storage, dropping tables and recreating them
		if err := cache.Reset(); err != nil {
			slog.Error("error resetting cache, exiting", "error", err)
			os.Exit(1)
		}

		extCount, verCount, err := cache.ReindexP(backend)
		if err != nil {
			slog.Error("error opening backend storage, exiting", "error", err)
			os.Exit(1)
		}

		slog.Info("done", "elapsedTime", time.Since(start).Round(time.Millisecond), "indexedExtensions", extCount, "indexedVersions", verCount, argGrp)
	},
}
