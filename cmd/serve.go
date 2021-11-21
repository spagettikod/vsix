package cmd

import (
	"log"
	"net/http"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:     "serve <root>",
	Short:   "Serve downloaded extensions",
	Example: "vsix serve",
	// Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		http.HandleFunc("/_apis/public/gallery/extensionquery", handle)
		if err := http.ListenAndServeTLS(":8080", "/home/roland/lego/certificates/lisa.spagettikod.se.crt", "/home/roland/lego/certificates/lisa.spagettikod.se.key", nil); err != nil {
			log.Fatalln(err)
		}
	},
}
