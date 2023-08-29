package cmd

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spf13/cobra"
)

func init() {
	dbCmd.AddCommand(dbListCmd)
}

var dbListCmd = &cobra.Command{
	Use:                   "list",
	Aliases:               []string{"ls"},
	Short:                 "List extensions in the database",
	Long:                  `List extensions in the database.`,
	Example:               "  $ vsix db list",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.With().Str("path", dbPath).Logger()
		start := time.Now()
		db, err := database.OpenFs(dbPath, false)
		if err != nil {
			log.Fatal().Err(err).Str("database_root", dbPath).Msg("could not open database")
		}
		exts := db.List(true)
		for _, ext := range exts {
			fmt.Printf("%s\n", ext.UniqueID())
		}
		logger.Debug().Msgf("total time for list %.3fs", time.Since(start).Seconds())
	},
}
