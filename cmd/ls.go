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
	Use:                   "ls PACKAGE",
	Short:                 "List extension versions",
	Example:               "vsix ls golang.Go",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		ext, err := vscode.ListVersions(args[0])
		if err != nil {
			errLog.Fatalln(err)
		}
		for _, v := range ext.Versions {
			fmt.Println(v.Version)
		}
	},
}
