package cmd

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	"github.com/spagettikod/vsix/db"
	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

const (
	assetURLPath = "assets/"
)

func init() {
	serveCmd.Flags().StringVarP(&serveDBRoot, "data", "d", ".", "directory where downloaded extensions are stored")
	serveCmd.Flags().StringVarP(&serveAddr, "addr", "a", "0.0.0.0:8080", "address where the server listens for connections")
	serveCmd.Flags().StringVarP(&serveCert, "cert", "c", "", "certificate file if serving with TLS [VSIX_CERT_FILE]")
	serveCmd.Flags().StringVarP(&serveKey, "key", "k", "", "certificate key file if serving with TLS [VSIX_KEY_FILE]")
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve [flags] <external URL>",
	Short: "Serve downloaded extensions to Visual Studio Code",
	Long: `This command will start a HTTPS server that is compatible with Visual Studio Code.
When setup you can browse, search and install extensions previously downloaded
using the sync command. If sync is run and new extensions are downloaded
while the server is running it will automatically update with the newly
downloaded extensions. 

To enable Visual Studio Code integration you must change the tag serviceUrl in
the file project.json in your Visual Studio Code installation. On MacOS, for
example, the file is located at
/Applications/Visual Studio Code.app/Contents/Resources/app/product.json. Set
the URL to your server, for example https://vsix.example.com:8080, see Examples
below.
`,
	Example: `  $ vsix serve --data _data https://www.example.com/vsix myserver.crt myserver.key

  $ docker run -d -p 8443:8080 \
	-v $(pwd):/data \
	-v myserver.crt:/server.crt:ro \
	-v myserver.key:/server.key:ro \
	spagettikod/vsix serve https://www.example.com/vsix:8443`,
	// Args:                  cobra.MinimumNArgs(3),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		externalURL := EnvOrArg("VSIX_EXTERNAL_URL", args, 0)
		// setup URLs and server root
		server, apiRoot, assetRoot, err := parseEndpoints(externalURL)
		if err != nil {
			fmt.Printf("external URL %s is not a valid URL\n", externalURL)
			os.Exit(1)
		}

		// load database of extensions
		root := "."
		if len(serveDBRoot) > 0 {
			root = serveDBRoot
		}
		db, err := db.New(root, server+assetRoot)
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
		http.Handle(assetRoot, stack.Then(assetHandler(db)))
		http.Handle(apiRoot, stack.Then(queryHandler(db)))

		log.Info().Msgf("Use this server in Visual Studio Code by setting \"serviceUrl\" in the file product.json to \"%s\"", server+apiRoot[:strings.LastIndex(apiRoot, "/")])
		log.Debug().Msgf("assets are served from %s", server+assetRoot)

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
	if externalURL[:5] != "https" {
		err = fmt.Errorf("external URL must use protocol 'https'")
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

func assetHandler(db *db.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		switch r.Method {
		case http.MethodOptions:
			w.Header().Set("Access-Control-Allow-Headers", "x-market-user-id,x-market-client-id")
		case http.MethodGet:
			hlog.FromRequest(r).Debug().Msgf("extracting filename from path: %s", r.URL.Path)
			// assemble filename from request URL
			pathParts := strings.Split(r.URL.Path, "/")
			// FIXME this panics if the requested path does not follow the expected layout
			filename := path.Join(db.Root, pathParts[len(pathParts)-4], pathParts[len(pathParts)-3], pathParts[len(pathParts)-2], pathParts[len(pathParts)-1])

			// set content type top json if returned file is a manifest
			if strings.Contains(filename, "Manifest") {
				hlog.FromRequest(r).Debug().Str("path", filename).Msg("requested file is a manifest setting content type to application/json")
				w.Header().Set("Content-Type", "application/json")
			}

			// open the file from local storage
			hlog.FromRequest(r).Debug().Str("path", filename).Msg("opening file")
			file, err := os.Open(filename)
			if err != nil {
				serverError(w, r, fmt.Errorf("error opening file: %v", err))
				return
			}
			defer file.Close()

			// return file as gzip
			w.Header().Set("Content-Encoding", "gzip")
			gw := gzip.NewWriter(w)
			defer gw.Close()
			hlog.FromRequest(r).Debug().Str("path", filename).Msg("sending file")
			_, err = io.Copy(gw, file)
			if err != nil {
				serverError(w, r, fmt.Errorf("error transmitting file: %v", err))
				return
			}
		}
	})
}

func queryHandler(db *db.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		switch r.Method {
		case http.MethodOptions:
			w.Header().Set("Access-Control-Allow-Headers", "x-market-user-id,x-market-client-id,content-type")
			return
		case http.MethodPost:
			if strings.Index(r.Header.Get("Content-Type"), "application/json") == 0 {
				hlog.FromRequest(r).Debug().Msg("reading HTTP body")
				b, err := ioutil.ReadAll(r.Body)
				if err != nil {
					serverError(w, r, fmt.Errorf("error while reading request body: %v", err))
					return
				}
				query := vscode.Query{}
				hlog.FromRequest(r).Debug().Msg("unmarshaling JSON")
				err = json.Unmarshal(b, &query)
				if err != nil {
					serverError(w, r, fmt.Errorf("error while unmarshaling request: %v", err))
					return
				}

				// debug print requested query
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

				results := vscode.NewResults([]vscode.Extension{})

				uniqueIDs := query.CriteriaValues(vscode.FilterTypeExtensionName)
				if len(uniqueIDs) > 0 {
					hlog.FromRequest(r).Debug().Msgf("found array of extension names in query: %v", uniqueIDs)
					extensions := db.FindByUniqueID(query.Flags == vscode.FlagLatestVersion, uniqueIDs...)
					hlog.FromRequest(r).Debug().Msgf("extension name database query found %v extension", len(extensions))
					results = vscode.NewResults(extensions)
				}

				searchValues := query.CriteriaValues(vscode.FilterTypeSearchText)
				if len(searchValues) > 0 {
					hlog.FromRequest(r).Debug().Msgf("found text searches in query: %v", searchValues)
					extensions := db.Search(query.Flags == vscode.FlagLatestVersion, searchValues...)
					hlog.FromRequest(r).Debug().Msgf("free text database query found %v extension", len(extensions))
					results = vscode.NewResults(extensions)
				}

				// remove extensions found in both queries
				results.Deduplicate()

				hlog.FromRequest(r).Debug().Msg("marshaling results to JSON")
				b, err = json.Marshal(results)
				if err != nil {
					serverError(w, r, fmt.Errorf("error while marshaling results: %v", err))
					return
				}

				// w.Header().Set("cache-control", "no-cache, no-store, must-revalidate")
				// w.Header().Set("content-type", "application/json; charset=utf-8")
				// w.Header().Set("request-context", "appId=cid-v1:8314cee3-137b-4671-85f0-0ebbe0d65ec2")
				// w.Header().Set("access-control-allow-origin", "*")
				// w.Header().Set("access-control-max-age", "3600")
				// w.Header().Set("access-control-allow-methods", "OPTIONS,GET,POST,PATCH,PUT,DELETE")
				// w.Header().Set("access-control-expose-headers", "ActivityId,Request-Context")
				// w.Header().Set("strict-transport-security", "max-age=2592000")
				// w.Header().Set("x-content-type-options", "nosniff")
				// w.Header().Set("activityid", "9caac648-7cbf-44e7-be1d-addd8b1850d9")
				// w.Header().Set("x-powered-by", "ASP.NET")
				// w.Header().Set("region", "CUS")
				// w.Header().Set("x-cache", "CONFIG_NOCACHE")
				// w.Header().Set("x-geo", "VSCodeSearchQueryGlobal_ExtensionQuery")
				// w.Header().Set("x-msedge-ref", "Ref A: 6DD684E9EE864289ACDEE17AF4D25F7E Ref B: CPH30EDGE0921 Ref C: 2022-02-05T15:02:08Z")
				// w.Header().Set("date", "Sat, 05 Feb 2022 15:02:08 GMT")

				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				hlog.FromRequest(r).Debug().Msg("sending response")
				if _, err = io.Copy(w, bytes.NewBuffer(b)); err != nil {
					serverError(w, r, fmt.Errorf("error while sending results: %v", err))
					return
				}

				// debug print query results
				if hlog.FromRequest(r).GetLevel() <= zerolog.DebugLevel {
					b, err := json.MarshalIndent(results, "", "  ")
					if err != nil {
						log.Error().Err(err).Msg("error while logging query response")
						return
					}
					hlog.FromRequest(r).Debug().Msg(string(b))
				}
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
