package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "vsix",
		Short: "Visual Studio Code Extension Marketplace command line interface tool.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			slog.SetLogLoggerLevel(slog.LevelWarn)
			dbPath = EnvOrFlag("VSIX_DB_PATH", dbPath)
			if !EnvOrFlagBool("VSIX_LOG_JSON", jsonLog) {
				log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
			}
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
			if EnvOrFlagBool("VSIX_LOG_VERBOSE", verbose) {
				slog.SetLogLoggerLevel(slog.LevelInfo)
				zerolog.SetGlobalLevel(zerolog.InfoLevel)
			}
			if EnvOrFlagBool("VSIX_LOG_DEBUG", debug) {
				slog.SetLogLoggerLevel(slog.LevelDebug)
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}
		},
	}
	verbose bool
	debug   bool
	jsonLog bool
	// out                          string   // used by sub-commands
	limit                        int      // used by sub-commands
	sortByFlag                   string   // used by sub-commands
	dbPath                       string   // used by sub-commands
	serveAddr                    string   // used by sub-commands
	serveCert                    string   // used by sub-commands
	serveKey                     string   // used by sub-commands
	targetPlatforms              []string // used by sub-commands
	preRelease                   bool     // used by sub-commands
	force                        bool     // used by sub-commands
	quiet                        bool     // used by sub-commands (search)
	nolimit                      bool     // used by sub-commands (search)
	keep                         int      // used by sub-commands
	threads                      int      // used by sub-commands
	ErrFileExists                error    = errors.New("extension has already been downloaded")
	ErrVersionNotFound           error    = errors.New("could not find version at Marketplace")
	ErrOutDirNotFound            error    = errors.New("output dir does not exist")
	ErrMultiplatformNotSupported error    = errors.New("multi-platform extensions are not supported yet")
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "turn on debug logging [VSIX_LOG_DEBUG]")
	rootCmd.PersistentFlags().BoolVar(&jsonLog, "json", false, "log output as JSON [VSIX_LOG_JSON]")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "turn on verbose logging [VSIX_LOG_VERBOSE]")
}

// Execute TODO
func Execute(version string) {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(`{{printf "%s" .Version}}
`)
	log.Logger = log.With().Str("vsix_version", rootCmd.Version).Logger()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func EnvOrFlag(env, flag string) string {
	if val, found := os.LookupEnv(env); found {
		return val
	}
	return flag
}

func EnvOrFlagBool(env string, flag bool) bool {
	if _, found := os.LookupEnv(env); found {
		return true
	}
	return flag
}
