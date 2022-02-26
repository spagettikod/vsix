package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/db"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dumpCmd)
}

var dumpCmd = &cobra.Command{
	Use:   "dump <path>",
	Short: "Print all extensions in the database",
	Long: `Dump will print all extensions found in the the database at the given path. The
output format is the same as the input format used by the sync command to sync
extensions. This makes the command ideal to migrate an existing database to
another if the database format changed between versions.
`,
	Example:               "  $ vsix dump /db",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		db, err := db.Open(args[0])
		if err != nil {
			if err != nil {
				log.Fatal().Err(err).Str("path", args[0]).Msg("error while opening database")
			}
		}
		exts := db.List()
		for _, e := range exts {
			for _, v := range e.Versions {
				fmt.Printf("%s %s\n", e.UniqueID(), v.Version)
			}
		}
	},
}
