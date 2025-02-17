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
	listCmd.Flags().BoolVarP(&all, "all", "a", false, "print version details")
	listCmd.Flags().BoolVar(&installs, "installs", false, "print version details")
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

		fexts := filterExtensions(db.List(), targetPlatforms, preRelease, prefix)
		if installs {
			slices.SortFunc(fexts, vscode.SortFuncExtensionByInstallCount)
		}
		if all {
			ftags := filterVersions(fexts, targetPlatforms, preRelease)
			if quiet {
				for _, tag := range ftags {
					fmt.Printf("%s\n", tag.String())
				}
			} else {
				data := [][]string{}
				for _, tag := range ftags {
					tagData := []string{}
					tagData = append(tagData, tag.UniqueID.String())
					tagData = append(tagData, tag.Version)
					tagData = append(tagData, tag.TargetPlatform)
					tagData = append(tagData, fmt.Sprintf("%v", tag.PreRelease))
					data = append(data, tagData)
				}
				printTable([]string{"Unique ID", "Version", "Target Platform", "Pre-release"}, data)
			}
		} else {
			if quiet {
				for _, ext := range fexts {
					fmt.Printf("%s\n", ext.UniqueID().String())
				}
			} else {
				data := [][]string{}
				for _, ext := range fexts {
					extData := []string{}
					extData = append(extData, ext.UniqueID().String())
					extData = append(extData, ext.LatestVersion(preRelease))
					extData = append(extData, ext.LastUpdated.Format("2006-01-02 15:04"))
					extData = append(extData, fmt.Sprintf("%v", ext.InstallCount()))
					data = append(data, extData)
				}
				printTable([]string{"Unique ID", "Latest Version", "Latest Release", "Installs"}, data)
			}
		}

		slog.Debug("done", "elapsedTime", time.Since(start).Round(time.Millisecond), argGrp)
	},
}

// filterExtensions filter extenions in the database. If no filter is applied all extenions are listed.
func filterExtensions(exts []vscode.Extension, targetPlatforms []string, preRelease bool, prefix string) []vscode.Extension {
	fexts := slices.Clone(exts)
	// filter out target platforms
	if len(targetPlatforms) > 0 {
		fexts = slices.DeleteFunc(fexts, func(e vscode.Extension) bool {
			return !e.MatchesTargetPlatforms(targetPlatforms...)
		})
	}

	// filter out all that are not pre-release
	if preRelease {
		fexts = slices.DeleteFunc(fexts, func(e vscode.Extension) bool {
			return !e.HasPreRelease()
		})
	}

	// filter out anything not starting with the given prefix
	if prefix != "" {
		fexts = slices.DeleteFunc(fexts, func(e vscode.Extension) bool {
			return !strings.Contains(e.UniqueID().String(), prefix)
		})
	}

	slices.SortFunc(fexts, func(e1, e2 vscode.Extension) int {
		return strings.Compare(strings.ToLower(e1.UniqueID().String()), strings.ToLower(e2.UniqueID().String()))
	})
	return fexts
}

// filterVersions apply filters and return VersionTags matching the filters. If not filters are applied all VersionTags are returned.
func filterVersions(exts []vscode.Extension, targetPlatforms []string, preRelease bool) []vscode.VersionTag {
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
