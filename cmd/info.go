package cmd

import (
	"fmt"
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
	Short:                 "Print information about your setup",
	Long:                  `Prints information about the current vsix setup.`,
	Example:               "  $ vsix info",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	PersistentPreRunE:     nil,
	Run: func(cmd *cobra.Command, args []string) {
		// about the configuration file
		fmt.Println("General")
		fmt.Println("-------")
		fmt.Println("  Version:        ", rootCmd.Version)
		fmt.Println("  Build:          ", buildDate)
		fmt.Println("  In container:   ", runtimeDocker())
		fmt.Println("  Config paths:   ", filepath.Join(configPaths[0], configFilename))
		for _, p := range configPaths[1:] {
			fmt.Printf("%s%s\n", strings.Repeat(" ", 19), filepath.Join(p, configFilename))
		}
		filename := viper.ConfigFileUsed()
		if filename == "" {
			filename = "<none>"
		}
		fmt.Println("  Config in use:  ", filename)
		fmt.Println("")

		// current configuration
		fmt.Println("Current configuration")
		fmt.Println("---------------------")
		fmt.Print("No prefix indicates default value (even though is might have been explicitly set).\n")
		fmt.Print("f = value differs from default, value read from configuration file\n")
		fmt.Print("e = value differs from default, value read from environment variable\n\n")
		for _, key := range sortDefaultKeys() {
			var value any
			switch key {
			case "VSIX_PLATFORMS":
				value = viper.GetStringSlice(key)
			case "VSIX_LOG_DEBUG":
				value = viper.GetBool(key)
			case "VSIX_LOG_VERBOSE":
				value = viper.GetBool(key)
			default:
				value = viper.Get(key)
			}
			fmt.Print(configValueSource(key))

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

// configValueSource returns a string where the configuration key value was set or if it equals the default value.
func configValueSource(key string) string {
	notDefault := false
	switch key {
	case "VSIX_PLATFORMS":
		p1 := viper.GetStringSlice(key)
		p2 := defaults[key].([]string)
		slices.Sort(p1)
		slices.Sort(p2)
		notDefault = !slices.Equal(p1, p2)
	default:
		notDefault = viper.Get(key) != defaults[key]
	}
	// check to see if value differs from default, we then know it has been changed
	if notDefault {
		_, isEnv := os.LookupEnv(key)
		if isEnv {
			return "e "
		} else if viper.InConfig(key) {
			return "f "
		}
	}
	return "  "
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
