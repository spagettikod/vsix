package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spagettikod/vsix/vscode"

	"github.com/spf13/cobra"
)

func init() {
	searchCmd.Flags().Int8VarP(&limit, "limit", "l", 20, "Limit number of results")
	searchCmd.Flags().StringVarP(&sortByFlag, "sort", "s", "install", "Sort critera, valid values are: none, install")
	rootCmd.AddCommand(searchCmd)
}

var searchCmd = &cobra.Command{
	Use:                   "search <query>",
	Short:                 "Search for extensions that matches query",
	Example:               "vsix search docker",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		q := args[0]
		if len(q) < 3 {
			errLog.Fatalln("query parameter must cotain atleast 3 characters")
		}
		verboseLog.Printf("%s: looking up extension at Marketplace", q)
		sortCritera, err := vscode.ParseSortCritera(sortByFlag)
		if err != nil {
			errLog.Fatalln(err)
		}
		exts, err := vscode.Search(q, limit, sortCritera)
		if err != nil {
			errLog.Fatalln(err)
		}
		data := [][]string{}
		for _, ext := range exts {
			extData := []string{}
			extData = append(extData, ext.UniqueID())
			if len(ext.DisplayName) > 30 {
				extData = append(extData, ext.DisplayName[:27]+"...")
			} else {
				extData = append(extData, ext.DisplayName)
			}
			extData = append(extData, ext.Publisher.DisplayName)
			extData = append(extData, ext.LatestVersion())
			extData = append(extData, ext.LastUpdated.Format(time.RFC3339))
			extData = append(extData, fmt.Sprint(ext.InstallCount()))
			avg := ext.AverageRating()
			if avg > -1 {
				extData = append(extData, fmt.Sprintf("%.2f (%v)", avg, ext.RatingCount()))
			} else {
				extData = append(extData, "-")
			}
			data = append(data, extData)
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Unique ID", "Name", "Publisher", "Latest Version", "Last Updated", "Installs", "Rating"})
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
	},
}
