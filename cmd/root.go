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
			if !jsonLog {
				log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
			}
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
			if verbose {
				zerolog.SetGlobalLevel(zerolog.InfoLevel)
			}
			if debug {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}
		},
	}
	verbose            bool
	debug              bool
	jsonLog            bool
	out                string // used by sub-commands
	limit              int    // used by sub-commands
	sortByFlag         string // used by sub-commands
	serveDBRoot        string // used by sub-commands
	serveAddr          string // used by sub-commands
	ErrFileExists      error  = errors.New("extension has already been downloaded")
	ErrVersionNotFound error  = errors.New("could not find version at Marketplace")
	ErrOutDirNotFound  error  = errors.New("output dir does not exist")
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "1", false, "turn on debug logging")
	rootCmd.PersistentFlags().BoolVarP(&jsonLog, "json", "j", false, "output verbose and debug logs as JSON")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "turn on verbose logging")
}

// Execute TODO
func Execute(version string) {
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
