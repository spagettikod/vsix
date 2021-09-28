package cmd

import (
	"fmt"
	"strings"

	"github.com/spagettikod/vsix/vscode"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(infoCmd)
}

var infoCmd = &cobra.Command{
	Use:                   "info <identifier>",
	Short:                 "Display package information",
	Example:               "vsix info golang.Go",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		VerboseLog.Printf("%s: looking up extension at Marketplace", args[0])
		ext, err := vscode.NewExtension(args[0])
		if err != nil {
			ErrLog.Fatalln(err)
		}
		s := `Name:           %s
Publisher:      %s
Latest version: %s
Released on:    %s
Last updated:   %s
Extension pack: %s

%s

`
		fmt.Printf(s, ext.Name, ext.Publisher.DisplayName, ext.Versions[0].Version, ext.ReleaseDate.Format("2006-01-02 15:04 UTC"), ext.LastUpdated.Format("2006-01-02 15:04 UTC"), strings.Join(ext.ExtensionPack(), ",\n                "), ext.ShortDescription)
	},
}
