package cmd

import (
	"fmt"

	"github.com/spagettikod/vsix/vscode"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(lsCmd)
}

var lsCmd = &cobra.Command{
	Use:   "ls [unique package name]",
	Short: "List extension versions",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		versions, err := vscode.ListVersions(vscode.Extension{UniqueID: args[0]})
		if err != nil {
			errLog.Fatalln(err)
		}
		for _, v := range versions {
			fmt.Println(v)
		}
	},
}
