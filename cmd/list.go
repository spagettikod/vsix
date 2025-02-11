package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spagettikod/vsix/storage"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

func init() {
	listCmd.Flags().StringVarP(&dbPath, "data", "d", ".", "path where downloaded extensions are stored [VSIX_DB_PATH]")
	listCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "only print unique identifier")
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List extensions from the local storage",
	Long: `List extensions from the local storage.
	
Command will list all extension with their unique identifier.`,
	Example:               "  $ vsix list --data extensions",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		argGrp := slog.Group("args", "cmd", "list", "data", dbPath)
		start := time.Now()
		db, err := storage.OpenFs(dbPath)
		if err != nil {
			slog.Error("could not open database, exiting", "error", err, argGrp)
			os.Exit(1)
		}

		if quiet {
			uids := db.ListUniqueIDs()
			for _, uid := range uids {
				fmt.Printf("%s\n", uid.String())
			}
		} else {
			exts := db.List()
			printTable(exts)
		}

		slog.Debug("done", "elapsedTime", time.Since(start).Round(time.Millisecond), argGrp)
	},
}

func printTable(exts []vscode.Extension) {
	data := [][]string{}
	for _, ext := range exts {
		for _, v := range ext.Versions {
			extData := []string{}
			extData = append(extData, ext.UniqueID().String())

			extData = append(extData, v.Version)

			extData = append(extData, v.TargetPlatform())

			extData = append(extData, fmt.Sprintf("%v", v.IsPreRelease()))

			data = append(data, extData)
		}
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
