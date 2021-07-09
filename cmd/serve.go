package cmd

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(serveCmd)
}

func handle(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method)
	for k, v := range r.Header {
		log.Printf("%s: %v\n", k, v)
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatalln(err)
	}
	if len(b) > 0 {
		var out bytes.Buffer
		json.Indent(&out, b, "", "  ")
		log.Println(out.String())
	}
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "*")
	w.Header().Add("Access-Control-Allow-Methods", "*")
	w.Header().Add("Access-Control-Expose-Headers", "x-ms-request-id,Server,x-ms-version,Content-Type,Content-Encoding,Cache-Control,Last-Modified,ETag,x-ms-lease-status,x-ms-blob-type,Content-Length,Date,Transfer-Encoding")
}

var serveCmd = &cobra.Command{
	Use:     "serve",
	Short:   "Serve downloaded extensions",
	Example: "vsix serve",
	// Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		http.HandleFunc("/extensionquery", handle)
		if err := http.ListenAndServeTLS(":8080", "/home/roland/lego/certificates/lisa.spagettikod.se.crt", "/home/roland/lego/certificates/lisa.spagettikod.se.key", nil); err != nil {
			log.Fatalln(err)
		}
	},
}
