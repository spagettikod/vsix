package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spf13/cobra"
)

func init() {
	searchCmd.Flags().IntVarP(&limit, "limit", "l", 20, "limit number of results")
	searchCmd.Flags().StringVarP(&sortByFlag, "sort", "s", "install", "sort critera, valid values are: none, install")
	searchCmd.Flags().BoolVar(&preRelease, "pre-release", false, "include pre-release versions")
	rootCmd.AddCommand(searchCmd)
}

var searchCmd = &cobra.Command{
	Use:                   "search <query>",
	Short:                 "Search for extensions that matches query",
	Example:               "  $ vsix search docker",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		q := args[0]
		if len(q) < 3 {
			fmt.Println("query parameter must cotain atleast 3 characters")
			os.Exit(1)
		}
		log.Info().Str("query", q).Msg("looking up extension at Marketplace")
		sortCritera, err := parseSortCriteria(sortByFlag)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		eqr, err := marketplace.QueryLastestVersionByText(q, limit, sortCritera).Run()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		exts := eqr.Results[0].Extensions
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
			extData = append(extData, ext.LatestVersion(preRelease))
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

func parseSortCriteria(sortBy string) (marketplace.SortCriteria, error) {
	switch sortBy {
	case "install":
		return marketplace.ByInstallCount, nil
	case "none":
		return marketplace.ByNone, nil
	}
	return marketplace.ByNone, fmt.Errorf("%s is not a valid sort critera", sortBy)
}
