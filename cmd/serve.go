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
	"github.com/spagettikod/vsix/storage"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

const (
	assetURLPath = "assets/"
)

func init() {
	serveCmd.Flags().StringVarP(&dbPath, "data", "d", ".", "path where downloaded extensions are stored [VSIX_DB_PATH]")
	serveCmd.Flags().StringVar(&serveAddr, "addr", "0.0.0.0:8080", "address where the server listens for connections")
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve [flags] <external URL>",
	Short: "Run your own marketplace and serve the extensions from your local storage",
	Long: `Run your own marketplace and serve the extensions in your local storage.

This command will start a server that is compatible with Visual Studio Code.
When setup you can browse, search and install extensions previously downloaded
using the add command. 

Since URL's of extensions are modified serve needs to know where it can be found.
For example setting the external URL to 'https://my.server.com/vsix' will generate
URL's starting with 'https://my.server.com/vsix'.

Serve only listens on http but Visual Studio Code requires https-endpoints. Use
a proxy like, Traefik or nginx, to terminate TLS when serving extensions.
`,
	Example:               `  $ vsix serve --data extensions --cert myserver.crt --key myserver.key https://www.example.com/vsix`,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		argGrp := slog.Group("args", "cmd", "serve", "path", dbPath, "addr", serveAddr, "external_url", EnvOrArg("VSIX_EXTERNAL_URL", args, 0))

		// get the path for the external URL to build the actual path
		extURL, err := url.Parse(EnvOrArg("VSIX_EXTERNAL_URL", args, 0))
		if err != nil {
			slog.Error("could not parse external url, exiting", "error", err, argGrp)
			os.Exit(1)
		}

		db, err := storage.OpenFs(dbPath)
		if err != nil {
			slog.Error("could not open database, exiting", "error", err, argGrp)
			os.Exit(1)
		}

		http.HandleFunc(fmt.Sprintf("OPTIONS %s/asset/%s", extURL.Path, vscode.AssetURLPattern), assetOptionsHandler(argGrp))
		http.HandleFunc(fmt.Sprintf("GET %s/asset/%s", extURL.Path, vscode.AssetURLPattern), assetGetHandler(db, argGrp))
		http.HandleFunc(fmt.Sprintf("OPTIONS %s/_gallery/{publisher}/{name}/latest", extURL.Path), latestOptionsHandler(argGrp))
		http.HandleFunc(fmt.Sprintf("GET %s/_gallery/{publisher}/{name}/latest", extURL.Path), latestGetHandler(db, extURL.String()+"/asset", argGrp))
		http.HandleFunc(fmt.Sprintf("OPTIONS %s/_apis/public/gallery/extensionquery", extURL.Path), queryOptionsHandler(argGrp))
		http.HandleFunc(fmt.Sprintf("POST %s/_apis/public/gallery/extensionquery", extURL.Path), queryPostHandler(db, extURL.String()+"/asset", argGrp))

		slog.Info("starting VSIX Server", argGrp)
		if err := http.ListenAndServe(serveAddr, nil); err != nil {
			slog.Error("error while serving, exiting", "error", err, argGrp)
			os.Exit(1)
		}
	},
}

func EnvOrArg(env string, args []string, idx int) string {
	if val, found := os.LookupEnv(env); found {
		return val
	}
	if idx < len(args) {
		return args[idx]
	}
	fmt.Printf("%s: parameter or flag missing\n", env)
	os.Exit(1)
	return ""
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

func assetGetHandler(db *storage.Database, argGrp slog.Attr) http.HandlerFunc {
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
		f, err := db.LoadAsset(tag, assetType)
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
		contentType, err := db.DetectAssetContentType(tag, assetType)
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
		if _, err = io.Copy(w, f); err != nil {
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

func latestGetHandler(db *storage.Database, assetURLPrefix string, argGrp slog.Attr) http.HandlerFunc {
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

		ext, found := db.FindByUniqueID(uid)
		if !found {
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

func queryPostHandler(db *storage.Database, assetURLPrefix string, argGrp slog.Attr) http.HandlerFunc {
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

		results, err := db.Run(query)
		if err != nil {
			if err == marketplace.ErrInvalidQuery {
				slog.Error("query contained in the request is not valid", "error", err)
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			} else {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				slog.Error("error running query", "error", err, requestGroup(r), argGrp)
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
