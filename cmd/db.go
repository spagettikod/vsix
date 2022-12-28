package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	addDataFlag(dbCmd)
	rootCmd.AddCommand(dbCmd)
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Manage the database with downloaded extensions",
}
