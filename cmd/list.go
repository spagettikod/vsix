package cmd

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spf13/cobra"
)

func init() {
	listCmd.Flags().StringVarP(&dbPath, "data", "d", ".", "path where downloaded extensions are stored [VSIX_DB_PATH]")
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List extensions from the local storage",
	Long: `List extensions from the local storage.
	
Command will list all extension with their unique identifier.`,
	Example:               "  $ vsix list --data extensions",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.With().Str("path", dbPath).Logger()
		start := time.Now()
		db, err := database.OpenFs(dbPath, false)
		if err != nil {
			log.Fatal().Err(err).Str("data_root", dbPath).Msg("could not open folder")
		}
		exts := db.List(true)
		for _, ext := range exts {
			fmt.Printf("%s\n", ext.UniqueID())
		}
		logger.Debug().Msgf("total time for list %.3fs", time.Since(start).Seconds())
	},
}
