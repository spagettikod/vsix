package cmd

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve [flags] <external URL>",
	Short: "Run your own marketplace and serve the extensions from your database",
	Long: `Run your own marketplace and serve the extensions in your database.

This command will start a server that is compatible with Visual Studio Code.
When setup you can browse, search and install extensions previously downloaded
using the add command. 

Since URL's of extensions are modified serve needs to know where it can be found.
For example setting the external URL to 'https://my.server.com/vsix' will generate
URL's starting with 'https://my.server.com/vsix'.

Serve only listens on http but Visual Studio Code requires https-endpoints. Use
a proxy like, Traefik or nginx, to terminate TLS when serving extensions.
`,
	Example:               `  $ vsix serve https://www.example.com/vsix`,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		argGrp := slog.Group("args", "cmd", "serve")

		// get the path for the external URL to build the actual path
		extURL, err := url.Parse(viper.GetString("VSIX_SERVE_URL"))
		if err != nil {
			slog.Error("could not parse external url, exiting", "error", err, argGrp)
			os.Exit(1)
		}

		http.HandleFunc(fmt.Sprintf("OPTIONS /asset/%s", vscode.AssetURLPattern), assetOptionsHandler(argGrp))
		http.HandleFunc(fmt.Sprintf("GET /asset/%s", vscode.AssetURLPattern), assetGetHandler(argGrp))
		http.HandleFunc("OPTIONS /_gallery/{publisher}/{name}/latest", latestOptionsHandler(argGrp))
		http.HandleFunc("GET /_gallery/{publisher}/{name}/latest", latestGetHandler(extURL.String()+"/asset", argGrp))
		http.HandleFunc("OPTIONS /_apis/public/gallery/extensionquery", queryOptionsHandler(argGrp))
		http.HandleFunc("POST /_apis/public/gallery/extensionquery", queryPostHandler(extURL.String()+"/asset", argGrp))

		slog.Info("starting VSIX Server", "addr", viper.GetString("VSIX_SERVE_ADDR"), "vsixServeUrl", viper.GetString("VSIX_SERVE_URL"), argGrp)
		if err := http.ListenAndServe(viper.GetString("VSIX_SERVE_ADDR"), nil); err != nil {
			slog.Error("error while serving, exiting", "error", err, argGrp)
			os.Exit(1)
		}
	},
}

func assetOptionsHandler(argGrp slog.Attr) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))

		// extract asset identifiers from url
		_, _, err := vscode.ParseAssetURL(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			slog.Error("error parsing asset url", "error", err, requestGroup(r), argGrp)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		slog.Debug("request complete", "elapsedTime", time.Since(start).Round(time.Millisecond), requestGroup(r), argGrp)
	})
}

func assetGetHandler(argGrp slog.Attr) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// extract asset identifiers from url
		tag, assetType, err := vscode.ParseAssetURL(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			slog.Error("error parsing asset url", "error", err, requestGroup(r), argGrp)
			return
		}

		// load the asset from database
		f, err := backend.LoadAsset(tag, assetType)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				slog.Error("asset not found", "error", err, requestGroup(r), argGrp)
				return
			}
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			slog.Error("error opening asset", "error", err, requestGroup(r), argGrp)
			return
		}
		defer f.Close()

		// determine content type and set headers
		contentType, err := backend.DetectAssetContentType(tag, assetType)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			slog.Error("error detecting asset content type", "error", err, requestGroup(r), argGrp)
			return
		}
		w.Header().Set("Content-Type", contentType)

		// gzip the response and write file in response
		gw := gzip.NewWriter(w)
		defer gw.Close()
		w.Header().Set("Content-Encoding", "gzip")
		if _, err = io.Copy(gw, f); err != nil {
			slog.Error("error sending asset file", "error", err, requestGroup(r), argGrp)
			return
		}
		slog.Debug("request complete", "elapsedTime", time.Since(start).Round(time.Millisecond), requestGroup(r), argGrp)
	})
}

func latestOptionsHandler(argGrp slog.Attr) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))

		// extract unique identifier from URL pattern
		_, ok := vscode.Parse(fmt.Sprintf("%s.%s", r.PathValue("publisher"), r.PathValue("name")))
		if !ok {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			slog.Error("error parsing unique id", requestGroup(r), argGrp)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		slog.Debug("request complete", "elapsedTime", time.Since(start).Round(time.Millisecond), requestGroup(r), argGrp)
	})
}

func latestGetHandler(assetURLPrefix string, argGrp slog.Attr) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Security-Policy", "connect-src 'self' http://localhost:8080;")

		// extract unique identifier from URL pattern
		uid, ok := vscode.Parse(fmt.Sprintf("%s.%s", r.PathValue("publisher"), r.PathValue("name")))
		if !ok {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			slog.Error("error parsing unique id", requestGroup(r), argGrp)
			return
		}

		ext, err := cache.FindByUniqueID(uid)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			slog.Info("extension not found", requestGroup(r), argGrp)
			return
		}

		// convert all external asset URL's to point to this VSIX serve instance instead of externally
		ext = ext.RewriteAssetURL(assetURLPrefix)

		bites, err := json.Marshal(ext)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			slog.Error("error marshaling extension metadata to json", "error", err, requestGroup(r), argGrp)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(bites); err != nil {
			slog.Error("error sending extension metadata response", "error", err, requestGroup(r), argGrp)
		}

		slog.Debug("request complete", "elapsedTime", time.Since(start).Round(time.Millisecond), requestGroup(r), argGrp)
	})
}

func queryOptionsHandler(argGrp slog.Attr) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))

		w.WriteHeader(http.StatusNoContent)
		slog.Debug("request complete", "elapsedTime", time.Since(start).Round(time.Millisecond), requestGroup(r), argGrp)
	})
}

func queryPostHandler(assetURLPrefix string, argGrp slog.Attr) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("Access-Control-Allow-Origin", "*")

		bites, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			slog.Error("error reading body", "error", err, requestGroup(r), argGrp)
			return
		}
		defer r.Body.Close()

		query := marketplace.Query{}
		err = json.Unmarshal(bites, &query)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			slog.Error("error unmarshaling query json", "error", err, requestGroup(r), argGrp)
			return
		}

		results, err := cache.Run(query)
		if err != nil {
			if err == marketplace.ErrInvalidQuery {
				slog.Error("query contained in the request is not valid", "error", err, "query", string(bites), requestGroup(r), argGrp)
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			} else {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				slog.Error("error running query", "error", err, "query", string(bites), requestGroup(r), argGrp)
			}
			return
		}
		results.RewriteAssetURL(assetURLPrefix)

		bites, err = json.Marshal(results)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			slog.Error("error marshaling query reponse json", "error", err, requestGroup(r), argGrp)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if _, err = io.Copy(w, bytes.NewBuffer(bites)); err != nil {
			slog.Error("error sending query reponse", "error", err, requestGroup(r), argGrp)
			return
		}

		slog.Debug("request complete", "elapsedTime", time.Since(start).Round(time.Millisecond), requestGroup(r), argGrp)
	})
}

func requestGroup(r *http.Request) slog.Attr {
	return slog.Group("request", "method", r.Method, "url", r.URL)
}
