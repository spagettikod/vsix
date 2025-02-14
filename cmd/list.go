package cmd

import (
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
	listCmd.Flags().StringVarP(&dbPath, "data", "d", ".", "path where downloaded extensions are stored [VSIX_DB_PATH]")
	listCmd.Flags().StringSliceVar(&targetPlatforms, "platforms", []string{}, "comma-separated list to limit the results to the given platforms")
	listCmd.Flags().BoolVar(&preRelease, "pre-release", false, "limit result to only pre-release versions")
	listCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "only print unique identifier")
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:     "list [flags] [prefix]",
	Aliases: []string{"ls"},
	Short:   "List downloaded extensions",
	Long: `List extensions available locally. By default all extension
versions are listed in a table format. Using an argument to the command
will limit the result by only showing extensions where the unique
identifier begins with the given prefix.

Adding the --quiet flag, without any other flags will list only the
unique identifiers for each extension.

If you want to limit which versions to show you can filter the result by
using the --pre-release or the --platforms flags. The results are
presented in a table format. Combining filters with --quiet will list
the unique tag for each version matching the filter.`,
	Example: `  List all extension versions where unique identifier starts with "redhat"
    $ vsix ls --data extensions redhat

  List all versions matching platforms linux-x64 and web
    $ vsix ls --data extensions --platforms linux-x64,web

  Combine commands and remove all pre-release versions
    $ vsix rm --data extensions $(vsix list --data extensions --pre-release --quiet)
`,
	Args:                  cobra.MaximumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		prefix := ""
		if len(args) > 0 {
			prefix = args[0]
		}
		argGrp := slog.Group("args", "cmd", "list", "data", dbPath, "preRelease", preRelease, "targetPlatforms", targetPlatforms, "prefix", prefix)
		start := time.Now()
		db, verrs, err := storage.Open(dbPath)
		if err != nil {
			slog.Error("could not open database, exiting", "error", err, argGrp)
			os.Exit(1)
		}
		printValidationErrors(verrs)

		if quiet {
			exts := filterExtensions(db, targetPlatforms, preRelease, prefix)
			if len(targetPlatforms) == 0 && !preRelease {
				// if no filters are applied list only the UniqueIDs (ie all extensions).
				for _, ext := range exts {
					fmt.Printf("%s\n", ext.UniqueID().String())
				}
			} else {
				// if filters are applied we list the version tags instead (since filters are on the Version level)
				for _, tag := range filterVersions(exts) {
					fmt.Printf("%s\n", tag.String())
				}
			}
		} else {
			fexts := filterExtensions(db, targetPlatforms, preRelease, prefix)
			printTable(filterVersions(fexts))
		}

		slog.Debug("done", "elapsedTime", time.Since(start).Round(time.Millisecond), argGrp)
	},
}

// filterExtensions filter extenions in the database. If no filter is applied all extenions are listed.
func filterExtensions(db *storage.Database, targetPlatforms []string, preRelease bool, prefix string) []vscode.Extension {
	exts := []vscode.Extension{}
	if len(targetPlatforms) > 0 {
		exts = append(exts, db.FindByTargetPlatforms(targetPlatforms...)...)
	} else {
		exts = db.List()
	}
	if prefix != "" {
		exts = slices.DeleteFunc(exts, func(e vscode.Extension) bool {
			return strings.Index(e.UniqueID().String(), prefix) == -1
		})
	}
	slices.SortFunc(exts, func(e1, e2 vscode.Extension) int {
		return strings.Compare(e1.UniqueID().String(), e2.UniqueID().String())
	})
	return exts
}

// filterVersions apply filters and return VersionTags matching the filters. If not filters are applied all VersionTags are returned.
func filterVersions(exts []vscode.Extension) []vscode.VersionTag {
	result := []vscode.VersionTag{}
	for _, ext := range exts {
		for _, v := range ext.Versions {
			if len(targetPlatforms) > 0 && !slices.Contains(targetPlatforms, v.TargetPlatform()) {
				continue
			}
			if preRelease && !v.IsPreRelease() {
				continue
			}
			result = append(result, v.Tag(ext.UniqueID()))
		}
	}
	return result
}

func printTable(tags []vscode.VersionTag) {
	data := [][]string{}
	for _, tag := range tags {
		extData := []string{}
		extData = append(extData, tag.UniqueID.String())
		extData = append(extData, tag.Version)
		extData = append(extData, tag.TargetPlatform)
		extData = append(extData, fmt.Sprintf("%v", tag.PreRelease))
		data = append(data, extData)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Unique ID", "Version", "Target platform", "Pre-release"})
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
