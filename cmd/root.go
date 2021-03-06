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
				verboseLog.SetOutput(os.Stdout)
			}
		},
	}
	errLog                = log.New(os.Stderr, "", 0)
	infLog                = log.New(os.Stdout, "", 0)
	verboseLog            = log.New(ioutil.Discard, "", 0)
	verbose               bool
	out                   string // used by sub-commands
	limit                 int8   // used by sub-commands
	sortByFlag            string // used by sub-commands
	errFileExists         error  = errors.New("extension has already been downloaded")
	errVersionNotFound    error  = errors.New("could not find version at Marketplace")
	errOutputPathNotFound error  = errors.New("output path does not exist")
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
