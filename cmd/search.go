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
	searchCmd.Flags().StringVarP(&sortByFlag, "sort", "s", "install", "sort critera, valid values are: none, install, rating, date")
	searchCmd.Flags().BoolVar(&preRelease, "pre-release", false, "include pre-release versions")
	searchCmd.Flags().BoolVar(&nolimit, "nolimit", false, "disables the result limit, all matching results are shown")
	searchCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "only print unique identifier")
	rootCmd.AddCommand(searchCmd)
}

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Query Marketplace for extensions.",
	Long: `Search for extensions at Marketplace.

Without any parameters the command lists extensions at Marketplace sorted by install count.
By default it limits the result to 20 items. Sort order and limits can be controlled
by flags.`,
	Example:               "  $ vsix search docker",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		q := ""
		if len(args) == 1 {
			q = args[0]
		}
		if nolimit {
			limit = 0
		}
		log.Info().Str("query", q).Msg("looking up extension at Marketplace")
		sortCritera, err := parseSortCriteria(sortByFlag)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		query := marketplace.QueryNoCritera(sortCritera)
		if q != "" {
			query = marketplace.QueryLastestVersionByText(q, sortCritera)
		}

		exts, err := query.RunAll(limit)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		data := [][]string{}
		for _, ext := range exts {
			if quiet {
				fmt.Println(ext.UniqueID())
				continue
			}
			extData := []string{}
			extData = append(extData, ext.UniqueID().String())
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

		if quiet {
			os.Exit(0)
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
	case "rating":
		return marketplace.ByRating, nil
	case "date":
		return marketplace.ByPublishedDate, nil
	case "none":
		return marketplace.ByNone, nil
	}
	return marketplace.ByNone, fmt.Errorf("%s is not a valid sort critera", sortBy)
}
