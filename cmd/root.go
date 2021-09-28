package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "vsix",
		Short: "Command line interface tool for Visual Studio Code Extension Marketplace.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				VerboseLog.SetOutput(os.Stdout)
			}
		},
	}
	verbose            bool
	out                string // used by sub-commands
	limit              int    // used by sub-commands
	sortByFlag         string // used by sub-commands
	forceget           bool   // used by sub-commands
	ErrFileExists      error  = errors.New("extension has already been downloaded")
	ErrVersionNotFound error  = errors.New("could not find version at Marketplace")
	ErrOutDirNotFound  error  = errors.New("output dir does not exist")

	ErrLog     = log.New(os.Stderr, "", 0)
	InfLog     = log.New(os.Stdout, "", 0)
	VerboseLog = log.New(ioutil.Discard, "", 0)
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

// Execute TODO
func Execute(version string) {
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
