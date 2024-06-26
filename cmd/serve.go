package cmd

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/justinas/alice"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
	"github.com/spagettikod/vsix/database"
	"github.com/spagettikod/vsix/marketplace"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

const (
	assetURLPath = "assets/"
)

func init() {
	serveCmd.Flags().StringVarP(&dbPath, "data", "d", ".", "path where downloaded extensions are stored [VSIX_DB_PATH]")
	serveCmd.Flags().StringVar(&serveAddr, "addr", "0.0.0.0:8080", "address where the server listens for connections")
	serveCmd.Flags().StringVar(&serveCert, "cert", "", "certificate file if serving with TLS [VSIX_CERT_FILE]")
	serveCmd.Flags().StringVar(&serveKey, "key", "", "certificate key file if serving with TLS [VSIX_KEY_FILE]")
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve [flags] <external URL>",
	Short: "Run your own marketplace and serve the extensions in your local storage",
	Long: `Run your own marketplace and serve the extensions in your local storage.

This command will start a HTTPS server that is compatible with Visual Studio Code.
When setup you can browse, search and install extensions previously downloaded
using the add command. If the update-command is run and new extensions are
downloaded while the serve-command is running it will automatically update with
the newly downloaded extensions. 

To enable Visual Studio Code integration you must change the tag serviceUrl in
the file project.json in your Visual Studio Code installation. On MacOS, for
example, the file is located at
/Applications/Visual Studio Code.app/Contents/Resources/app/product.json. Set
the URL to your server, for example https://vsix.example.com:8080, see examples
below.
`,
	Example:               `  $ vsix serve --data extensions --cert myserver.crt --key myserver.key https://www.example.com/vsix`,
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		externalURL := EnvOrArg("VSIX_EXTERNAL_URL", args, 0)
		// setup URLs and server root
		server, apiRoot, assetRoot, err := parseEndpoints(externalURL)
		if err != nil {
			fmt.Printf("given URL is not valid: %s\n", externalURL)
			os.Exit(1)
		}

		// load database of extensions
		root := "."
		if len(dbPath) > 0 {
			root = dbPath
		}
		db, err := database.OpenFs(root, true)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		stack := alice.New(
			hlog.NewHandler(log.Logger),
			hlog.RequestIDHandler("request_id", ""),
			hlog.MethodHandler("method"),
			hlog.URLHandler("url"),
			hlog.RemoteAddrHandler("remote_addr"),
			hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
				hlog.FromRequest(r).Info().
					Int("status", status).
					Int64("request_size", r.ContentLength).
					Int("response_size", size).
					Dur("duration", time.Duration(duration.Microseconds())).
					Msg("ACCESS")
			}),
		)

		// setup and start server
		http.Handle(assetRoot, stack.Then(assetHandler(db, "/"+assetRoot)))
		http.Handle(apiRoot, stack.Then(queryHandler(db, server, assetRoot)))

		log.Info().Msgf("Use this server in Visual Studio Code by setting \"serviceUrl\" in the file product.json to \"%s\"", server+apiRoot[:strings.LastIndex(apiRoot, "/")])
		log.Debug().Msgf("assets are served from %s", server+assetRoot)
		log.Debug().Msgf("API served from %s", server+apiRoot)

		serveCert = EnvOrFlag("VSIX_CERT_FILE", serveCert)
		serveKey = EnvOrFlag("VSIX_KEY_FILE", serveKey)

		if serveCert == "" || serveKey == "" {
			log.Info().
				Str("cert", serveCert).
				Str("key", serveKey).
				Msg("Certificiate and key were not given, starting without TLS")
			if err := http.ListenAndServe(serveAddr, nil); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		} else {
			log.Info().
				Str("cert", serveCert).
				Str("key", serveKey).
				Msg("Certificiate and key were specified, starting with TLS")
			if err := http.ListenAndServeTLS(serveAddr, serveCert, serveKey, nil); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
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

func parseEndpoints(externalURL string) (server string, apiRoot string, assetRoot string, err error) {
	if len(externalURL) < 5 {
		err = fmt.Errorf("invalid URL")
		return
	}
	if externalURL[:5] != "https" && externalURL[:4] != "http" {
		err = fmt.Errorf("URL is missing protocol")
		return
	}
	u, err := url.Parse(externalURL)
	if err != nil {
		return
	}
	server = u.Scheme + "://" + u.Host
	externalURL = u.Path
	if externalURL == "" {
		externalURL = "/"
	}

	if externalURL[len(externalURL)-1:] == "/" {
		apiRoot = externalURL + "extensionquery"
		assetRoot = externalURL + assetURLPath
	} else {
		apiRoot = externalURL + "/extensionquery"
		assetRoot = externalURL + "/" + assetURLPath
	}
	return
}

func assetHandler(db *database.DB, assetURLPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		switch r.Method {
		case http.MethodOptions:
			w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
		case http.MethodGet:
			hlog.FromRequest(r).Debug().Msgf("extracting filename from path: %s", r.URL.Path)
			// assemble filename from request URL
			filePath := path.Join(db.Root(), r.URL.Path[len(assetURLPath)-1:])

			// set content type top json if returned file path is a manifest
			if strings.Contains(filePath, "Manifest") {
				hlog.FromRequest(r).Debug().Str("filePath", filePath).Msg("requested file is a manifest setting content type to application/json")
				w.Header().Set("Content-Type", "application/json")
			}

			// open the file from local storage
			hlog.FromRequest(r).Debug().Str("filePath", filePath).Msg("opening file")
			file, err := os.Open(filePath)
			if err != nil {
				serverError(w, r, fmt.Errorf("error opening file: %v", err))
				return
			}
			defer file.Close()

			// return file as gzip
			w.Header().Set("Content-Encoding", "gzip")
			gw := gzip.NewWriter(w)
			defer gw.Close()
			hlog.FromRequest(r).Debug().Str("filePath", filePath).Msg("sending file")
			_, err = io.Copy(gw, file)
			if err != nil {
				serverError(w, r, fmt.Errorf("error transmitting file: %v", err))
				return
			}
		}
	})
}

func queryHandler(db *database.DB, server, assetRoot string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		switch r.Method {
		case http.MethodOptions:
			w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
			return
		case http.MethodPost:
			if strings.Index(r.Header.Get("Content-Type"), "application/json") == 0 {
				hlog.FromRequest(r).Debug().Msg("reading HTTP body")
				b, err := io.ReadAll(r.Body)
				if err != nil {
					serverError(w, r, fmt.Errorf("error while reading request body: %v", err))
					return
				}
				query := marketplace.Query{}
				hlog.FromRequest(r).Debug().Msg("unmarshaling JSON")
				err = json.Unmarshal(b, &query)
				if err != nil {
					serverError(w, r, fmt.Errorf("error while unmarshaling request: %v", err))
					return
				}

				debugRequest(r, query)

				results, err := db.Run(query)
				if err != nil {
					if err == marketplace.ErrInvalidQuery {
						hlog.FromRequest(r).Info().Msg("query contained in the request is not valid")
						http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
					} else {
						serverError(w, r, fmt.Errorf("error while querying the local storage: %v", err))
					}
					return
				}
				results.SetAssetEndpoint(server + assetRoot)

				hlog.FromRequest(r).Debug().Msg("marshaling results to JSON")
				b, err = json.Marshal(results)
				if err != nil {
					serverError(w, r, fmt.Errorf("error while marshaling results: %v", err))
					return
				}

				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				hlog.FromRequest(r).Debug().Msg("sending response")
				if _, err = io.Copy(w, bytes.NewBuffer(b)); err != nil {
					serverError(w, r, fmt.Errorf("error while sending results: %v", err))
					return
				}

				debugResponse(r, results)

			} else {
				hlog.FromRequest(r).Debug().Msg("incoming request is not application/json, skipping this request")
			}
		}
	})
}

func serverError(w http.ResponseWriter, r *http.Request, err error) {
	hlog.FromRequest(r).Error().
		Err(err).
		Send()
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func debugRequest(r *http.Request, query marketplace.Query) {
	if hlog.FromRequest(r).GetLevel() <= zerolog.DebugLevel {
		headers := ""
		for k, v := range r.Header {
			headers = fmt.Sprintf("%s%s: %v\n", headers, k, strings.Join(v, ","))
		}
		hlog.FromRequest(r).Debug().Msg(headers)
		b, err := json.MarshalIndent(query, "", "  ")
		if err != nil {
			log.Error().Err(err).Msg("error while logging request query")
			return
		}
		hlog.FromRequest(r).Debug().Msg(string(b))
	}
}

func debugResponse(r *http.Request, results vscode.Results) {
	if hlog.FromRequest(r).GetLevel() <= zerolog.DebugLevel {
		b, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			log.Error().Err(err).Msg("error while logging query response")
			return
		}
		hlog.FromRequest(r).Debug().Msg(string(b))
	}
}
