package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(infoCmd)
}

var infoCmd = &cobra.Command{
	Use:                   "info",
	Short:                 "Prints information about the current vsix setup",
	Long:                  `Prints information about the current vsix setup.`,
	Example:               "  $ vsix info",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	PersistentPreRunE:     nil,
	Run: func(cmd *cobra.Command, args []string) {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalln(err)
		}
		// about the configuration file
		fmt.Println("General")
		fmt.Println("-------")
		fmt.Println("  Version:        ", rootCmd.Version)
		fmt.Println("  Build:          ", buildDate)
		fmt.Println("  In container:   ", runtimeDocker())
		fmt.Println("  Config paths:   ", filepath.Join(configPaths[0], configFilename))
		if !runtimeDocker() {
			fmt.Println("                  ", filepath.Join(wd, configFilename))
		}
		filename := viper.ConfigFileUsed()
		if filename == "" {
			filename = "<no file found>"
		}
		fmt.Println("  Config in use:  ", filename)
		fmt.Println("")

		// current configuration
		fmt.Println("Current configuration")
		fmt.Println("---------------------")
		fmt.Print("The * means the value has been modifed from the default value.\n\n")
		for _, key := range sortDefaultKeys() {
			changed := false
			var value any
			switch key {
			case "VSIX_PLATFORMS":
				changed = len(viper.GetStringSlice(key)) != 0
				value = viper.GetStringSlice(key)
			case "VSIX_LOG_DEBUG":
				changed = viper.GetBool(key) != defaults[key]
				value = viper.GetBool(key)
			case "VSIX_LOG_VERBOSE":
				changed = viper.GetBool(key) != defaults[key]
				value = viper.GetBool(key)
			default:
				changed = viper.GetString(key) != defaults[key]
				value = viper.Get(key)
			}
			if changed {
				fmt.Print("* ")
			} else {
				fmt.Print("  ")
			}

			fmt.Println(key, spaces(key), value)
		}
		fmt.Println("")

		// Cache
		fmt.Println("Cache")
		fmt.Println("-----")
		if err := setupCache(); err != nil {
			fmt.Printf("<cache could not be read: %s>\n", err)
		} else {
			defer cache.Close()
			stats, err := cache.Stats()
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println("  Filename:         ", cache.Filename)
			fmt.Println("  Extensions:       ", stats.ExtensionCount)
			fmt.Println("  Versions:         ", stats.VersionCount)
			fmt.Println("  Latest update:    ", stats.LastUpdated.Format(time.DateTime))
			fmt.Println("  Target platforms: ", strings.Join(strings.Split(stats.Platforms, ","), "\n                    "))
		}
	},
}

func sortDefaultKeys() []string {
	keys := []string{}
	for k := range defaults {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func spaces(key string) string {
	longestKey := 0
	for k := range defaults {
		if len(k) > longestKey {
			longestKey = len(k)
		}
	}
	return strings.Repeat(" ", longestKey-len(key))
}
