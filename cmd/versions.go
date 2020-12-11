package cmd

import (
	"github.com/spagettikod/vsix/vscode"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionsCmd)
}

var versionsCmd = &cobra.Command{
	Use:                   "versions <package>",
	Short:                 "List avilable versions for an extension",
	Example:               "vsix versions golang.Go",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		verboseLog.Printf("%s: looking up extension at Marketplace", args[0])
		ext, err := vscode.NewExtension(args[0])
		if err != nil {
			errLog.Fatalln(err)
		}
		for _, v := range ext.Versions {
			infLog.Println(v.Version)
		}
	},
}
