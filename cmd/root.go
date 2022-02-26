package cmd

import (
	"errors"
	"fmt"
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
			if !EnvOrFlagBool("VSIX_LOG_JSON", jsonLog) {
				log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
			}
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
			if EnvOrFlagBool("VSIX_LOG_VERBOSE", verbose) {
				zerolog.SetGlobalLevel(zerolog.InfoLevel)
			}
			if EnvOrFlagBool("VSIX_LOG_DEBUG", debug) {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}
		},
	}
	verbose                      bool
	debug                        bool
	jsonLog                      bool
	out                          string // used by sub-commands
	limit                        int    // used by sub-commands
	sortByFlag                   string // used by sub-commands
	serveDBRoot                  string // used by sub-commands
	serveAddr                    string // used by sub-commands
	serveCert                    string // used by sub-commands
	serveKey                     string // used by sub-commands
	ErrFileExists                error  = errors.New("extension has already been downloaded")
	ErrVersionNotFound           error  = errors.New("could not find version at Marketplace")
	ErrOutDirNotFound            error  = errors.New("output dir does not exist")
	ErrMultiplatformNotSupported error  = errors.New("multi-platform extensions are not supported yet")
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "1", false, "turn on debug logging [VSIX_LOG_DEBUG]")
	rootCmd.PersistentFlags().BoolVarP(&jsonLog, "json", "j", false, "output verbose and debug logs as JSON [VSIX_LOG_JSON]")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "turn on verbose logging [VSIX_LOG_VERBOSE]")
}

// Execute TODO
func Execute(version string) {
	rootCmd.Version = version
	log.Logger = log.With().Str("version", rootCmd.Version).Logger()
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
