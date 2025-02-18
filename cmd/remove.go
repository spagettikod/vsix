package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spagettikod/vsix/storage"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

func init() {
	removeCmd.Flags().StringVarP(&dbPath, "data", "d", ".", "path where downloaded extensions are stored [VSIX_DB_PATH]")
	removeCmd.Flags().BoolVarP(&force, "force", "f", false, "don't prompt for confirmation before deleting")
	rootCmd.AddCommand(removeCmd)
}

var removeCmd = &cobra.Command{
	Use:     "remove [flags] <identifier[@version][:target platform]...>",
	Aliases: []string{"rm"},
	Short:   "Remove items from database",
	Long: `Remove items from database.

Remove uses the "tag"-format, which enables you to specify exact version and
platform to remove. This format is compatible with output from the
"list"-command which makes it possible to chain these commands and filter
which items to remove (see examples).

Tag-format
----------
This format extends the Marketplace defined Unique Identifier and enables you to
specify version and target platform to better pin-point a certain release.

Some examples:

   ms-vscode.cpptools
   ------------------
   Unique identifier, this tag will remove the entire extension "ms-vscode.cpptools".

   ms-vscode.cpptools@1.24.1
   -------------------------
   Tag with version, this tag will remove version 1.24.1 (regardless of target platform)
   for extension "ms-vscode.cpptools".

   ms-vscode.cpptools@1.24.1:win32-arm64
   -------------------------------------
   Tag with version and platform, this tag will remove platform "win32-arm64" in version
   1.24.1 for extension "ms-vscode.cpptools".
`,
	Example: `  Remove Java extension, all versions will be removed:
    $ vsix remove --data extensions redhat.java

  Remove all pre-release versions for all extension:
    $ vsix remove --data extensions $(vsix list --data extensions --pre-release --all --quiet)

  Remove all versions for platform win32-arm64:
    $ vsix remove --data extensions $(vsix list --data extensions --platforms win32-arm64 --all --quiet)
`,
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		argGrp := slog.Group("args", "cmd", "remove", "preRelease", preRelease)

		tags := []vscode.VersionTag{}
		for _, argTag := range args {
			tag, err := vscode.ParseVersionTag(argTag)
			if err != nil {
				slog.Error("error parsing arguments, exiting", "error", err, argGrp)
				os.Exit(1)
			}
			tags = append(tags, tag)
		}

		if !force && len(tags) > 0 {
			for _, tag := range tags {
				fmt.Println(tag.String())
			}
			fmt.Println("-----")
			if !confirm(len(tags)) {
				os.Exit(0)
			}
		}
		db, _, err := storage.Open(dbPath)
		if err != nil {
			slog.Error("failed open database, exiting", "error", err, argGrp)
			os.Exit(1)
		}
		for _, tag := range tags {
			if force { // force prints removed tags
				fmt.Println(tag)
			}
			if err := db.Remove(tag); err != nil {
				slog.Error("remove failed, exiting", "error", err, argGrp)
				os.Exit(1)
			}
		}
	},
}
