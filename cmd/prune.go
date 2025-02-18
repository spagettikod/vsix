package cmd

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spagettikod/vsix/storage"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

func init() {
	pruneCmd.Flags().StringVarP(&dbPath, "data", "d", ".", "path where downloaded extensions are stored [VSIX_DB_PATH]")
	pruneCmd.Flags().IntVar(&keep, "keep", 0, "number of versions to keep, 0 keeps all (default 0)")
	pruneCmd.Flags().BoolVar(&dry, "dry-run", false, "execute command without actually removing anything")
	pruneCmd.Flags().BoolVarP(&force, "force", "f", false, "don't prompt for confirmation before deleting")
	rootCmd.AddCommand(pruneCmd)
}

var pruneCmd = &cobra.Command{
	Use:   "prune [flags]",
	Short: "Prune database",
	Long: `Prune database from incomplete extensions or limit number of versions.

By default, without any flags, this command will print a list and prompt you
before removing anything. The list shows you which items are affected and the
reason why they will be removed by prune. You can also use --dry-run to show the
list without prompting you for removal.

Pruning versions
----------------
Adding the --keep-versions X flag will keep the X latest versions and remove the rest.
`,
	Example: `  Keep the latest two versions and remove the rest:
    $ vsix prune --data extensions --keep-versions 2
`,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		argGrp := slog.Group("args", "prune", "add", "path", dbPath, "preRelease", preRelease, "targetPlatforms", targetPlatforms)

		db, results, err := storage.Open(dbPath)
		if err != nil {
			slog.Error("failed open database, exiting", "error", err, argGrp)
			os.Exit(1)
		}

		if keep > 0 {
			for _, ext := range db.List() {
				// reduce to keep only the version numbers, we'll prune versions regardless of platform
				reducedVersions := slices.CompactFunc(ext.Versions, func(v1, v2 vscode.Version) bool {
					return v1.Version == v2.Version
				})
				if len(reducedVersions) > keep { // are there more versions than we want to keep?
					for _, v := range reducedVersions[keep:] { // loop through all versions we don't want to keep
						res := storage.ValidationError{
							Tag:   vscode.VersionTag{UniqueID: ext.UniqueID(), Version: v.Version},
							Error: fmt.Errorf("version pruning, keep %v latest versions", keep),
						}
						results = append(results, res)
					}
				}
			}
		}

		if dry {
			printValidationErrorsTable(results)
			os.Exit(0)
		} else {
			if !force { // print items that will be removed before confirmation
				printValidationErrorsTable(results)
				if len(results) == 0 { // exit if we printed nothing
					os.Exit(0)
				}
				fmt.Println("-----")
			}
			if force || confirm(len(results)) {
				for _, result := range results {
					if force { // force prints removed tags
						fmt.Println(result.Tag)
					}
					if err := db.Remove(result.Tag); err != nil {
						slog.Error("remove failed, exiting", "error", err, argGrp)
						os.Exit(1)
					}
				}
			}
		}

		slog.Info("done", "elapsedTime", time.Since(start).Round(time.Millisecond), argGrp)
	},
}

func printValidationErrorsTable(verrs []storage.ValidationError) {
	data := [][]string{}
	for _, verr := range verrs {
		extData := []string{}
		extData = append(extData, verr.Tag.String())
		extData = append(extData, verr.Error.Error())
		data = append(data, extData)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Item", "Reason"})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)
	table.AppendBulk(data) // Add Bulk Data
	table.Render()
}

func confirm(items int) bool {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Listed %v items will be removed, are you sure you want to continue? (y/N): ", items)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input) // Remove any trailing newline characters

	return strings.ToLower(input) == "y"
}
