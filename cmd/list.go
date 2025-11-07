package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spagettikod/vsix/storage"
	"github.com/spf13/cobra"
)

var (
	all      bool
	installs bool
)

func init() {
	listCmd.Flags().StringSliceVar(&targetPlatforms, "platforms", []string{}, "comma-separated list to limit the results to the given platforms")
	listCmd.Flags().BoolVar(&preRelease, "pre-release", false, "limit result to only pre-release versions")
	listCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "only print unique identifier")
	listCmd.Flags().BoolVarP(&all, "all", "a", false, "print version details")
	listCmd.Flags().BoolVar(&installs, "installs", false, "sort by number of installs")
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:     "list [flags] [query]",
	Aliases: []string{"ls"},
	Short:   "List extensions in local database",
	Long: `List extensions available locally. By default all extensions are listed
in a table format. Use --all to list all extensions and their individual versions.

Using an argument to the command will limit the result by only showing extensions
where the unique identifier contains the given text.

If you want to limit which versions to show you can filter the result by using
--pre-release or --platforms. The results are presented in a table format.
Combining filters with --quiet will list the unique tag for each version matching
the filter.

Adding --quiet, without any other flags will list only the unique identifiers
for each extension. Using --all together with --quiet will list the version tags
for every version in the database.

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
   1.24.1 for extension "ms-vscode.cpptools`,
	Example: `  List all extension versions where unique identifier starts with "redhat":
    $ vsix list redhat

  List all versions matching platforms linux-x64 and web:
    $ vsix list --platforms linux-x64,web

  Combine commands and remove all pre-release versions:
    $ vsix remove $(vsix list --pre-release --all --quiet)
`,
	Args:                  cobra.MaximumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		query := ""
		if len(args) > 0 {
			query = args[0]
		}
		argGrp := slog.Group("args", "cmd", "list", "preRelease", preRelease, "targetPlatforms", targetPlatforms, "prefix", query)
		start := time.Now()

		q := storage.NewQuery()
		q.Platforms = targetPlatforms
		q.IncludePreRelease = preRelease
		q.Latest = !all
		q.Text = query
		if installs {
			q.SortOrder = storage.OrderByInstalls
		}
		qr, err := cache.Query(q)
		if err != nil {
			slog.Error("error listing exstensions from cache, exiting", "error", err, argGrp)
			os.Exit(1)
		}
		if all {
			if quiet {
				for _, r := range qr {
					fmt.Printf("%s\n", r.Tag.String())
				}
			} else {
				data := [][]string{}
				for _, r := range qr {
					tagData := []string{}
					tagData = append(tagData, r.Tag.UniqueID.String())
					tagData = append(tagData, r.Tag.Version)
					tagData = append(tagData, r.Tag.TargetPlatform)
					tagData = append(tagData, r.LastUpdated.Format("2006-01-02 15:04"))
					tagData = append(tagData, fmt.Sprintf("%v", r.Tag.PreRelease))
					data = append(data, tagData)
				}
				printTable([]string{"Unique ID", "Version", "Target Platform", "Release Date", "Pre-release"}, data)
			}
		} else {
			if quiet {
				for _, r := range qr {
					fmt.Printf("%s\n", r.Tag.UniqueID.String())
				}
			} else {
				data := [][]string{}
				for _, r := range qr {
					extData := []string{}
					extData = append(extData, r.Tag.UniqueID.String())
					extData = append(extData, r.Tag.Version)
					extData = append(extData, r.LastUpdated.Format("2006-01-02 15:04"))
					if preRelease {
						extData = append(extData, fmt.Sprintf("%v", r.Tag.PreRelease))
					}
					extData = append(extData, fmt.Sprintf("%v", r.Installs))
					data = append(data, extData)
				}
				if preRelease {
					printTable([]string{"Unique ID", "Latest Version", "Latest Release", "Pre-release", "Installs"}, data)
				} else {
					printTable([]string{"Unique ID", "Latest Version", "Latest Release", "Installs"}, data)
				}
			}
		}
		slog.Debug("done", "elapsedTime", time.Since(start).Round(time.Millisecond), argGrp)
	},
}

func printTable(headers []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
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
