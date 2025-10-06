package cmd

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spagettikod/vsix/storage"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	verbose bool
	debug   bool
	backend storage.Backend
	cache   storage.Cache

	// used by sub-commands
	targetPlatforms []string // used by sub-commands
	preRelease      bool     // used by sub-commands
	force           bool     // used by sub-commands
	quiet           bool     // used by sub-commands (search)

	ErrFileExists                error = errors.New("extension has already been downloaded")
	ErrVersionNotFound           error = errors.New("could not find version at Marketplace")
	ErrOutDirNotFound            error = errors.New("output dir does not exist")
	ErrMultiplatformNotSupported error = errors.New("multi-platform extensions are not supported yet")

	// paths where to look for configuration files
	configFilename string         = ".env"
	configPaths    []string       = []string{}
	defaults       map[string]any = map[string]any{
		"VSIX_BACKEND":        "fs",
		"VSIX_CACHE_FILE":     "",
		"VSIX_FS_DIR":         "",
		"VSIX_LOG_DEBUG":      false,
		"VSIX_LOG_VERBOSE":    false,
		"VSIX_PLATFORMS":      []string{},
		"VSIX_S3_CREDENTIALS": "~/.aws/credentials",
		"VSIX_S3_PROFILE":     "default",
		"VSIX_S3_URL":         "http://localhost:9000",
		"VSIX_S3_BUCKET":      "",
		"VSIX_SERVE_ADDR":     "0.0.0.0:8080",
		"VSIX_SERVE_URL":      "http://example.com:8080/vsix",
	}

	rootCmd = &cobra.Command{
		Use:                "vsix",
		Short:              "Visual Studio Code Extension Marketplace command line interface tool.",
		PersistentPreRunE:  persistentPreRunE,
		PersistentPostRunE: persistentPostRunE,
	}
)

func init() {
	viper.SetConfigName(configFilename)
	viper.SetConfigType("env")

	// setup configuration paths
	// if err == nil {
	// 	dataDir = filepath.Join(dataDir, "vsix")
	// 	extDir := filepath.Join(dataDir, storage.FSBackendDir)
	// 	if err := os.MkdirAll(extDir, 0750); err != nil {
	// 		if !os.IsExist(err) {
	// 			log.Fatalln(err)
	// 		}
	// 	}
	// } else {
	// 	log.Fatalln(err)
	// }
	configDir, err := os.UserConfigDir()
	if err == nil {
		configDir = filepath.Join(configDir, "vsix")
		if err := os.Mkdir(configDir, 0750); err != nil {
			if !os.IsExist(err) {
				log.Fatalln(err)
			}
		}
		configPaths = append(configPaths, configDir)
	} else {
		log.Fatalln(err)
	}
	configPaths = append(configPaths, ".")
	for _, v := range configPaths {
		viper.AddConfigPath(v)
	}

	// reading configuration, if found
	if err := viper.ReadInConfig(); err != nil {
		// fall back to environtment variables if not found
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Fatalln(err)
		}
	}
	viper.AutomaticEnv()

	rootCmd.PersistentFlags().BoolVar(&debug, "debug", defaults["VSIX_LOG_DEBUG"].(bool), "turn on debug logging [VSIX_LOG_DEBUG]")
	if err := viper.BindPFlag("VSIX_LOG_DEBUG", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		log.Fatalln(err)
	}

	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", defaults["VSIX_LOG_VERBOSE"].(bool), "turn on verbose logging [VSIX_LOG_VERBOSE]")
	if err := viper.BindPFlag("VSIX_LOG_VERBOSE", rootCmd.PersistentFlags().Lookup("verbose")); err != nil {
		log.Fatalln(err)
	}

	// these defaults are caculated and are not hard coded in the map
	dataDir, err := storage.UserDataDir()
	if err != nil {
		log.Fatalln(err)
	}
	defaults["VSIX_CACHE_FILE"] = filepath.Join(dataDir, "vsix", storage.CacheFilename)
	defaults["VSIX_FS_DIR"] = filepath.Join(dataDir, "vsix", storage.FSBackendDir)
	// set defaults from the default map
	for k, v := range defaults {
		viper.SetDefault(k, v)
	}
}

func Execute(version string) {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(`{{printf "%s" .Version}}
`)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func persistentPreRunE(cmd *cobra.Command, args []string) error {
	slog.SetLogLoggerLevel(slog.LevelWarn)

	if viper.GetBool("VSIX_LOG_VERBOSE") {
		slog.SetLogLoggerLevel(slog.LevelInfo)
	}
	if viper.GetBool("VSIX_LOG_DEBUG") {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	if viper.ConfigFileUsed() != "" {
		slog.Info("read configuration", "file", viper.ConfigFileUsed())
	} else {
		slog.Info("no configuration file used, using defaults")
	}

	if cmd.Name() != "info" && cmd.Name() != "search" {
		if err := setupBackend(); err != nil {
			return err
		}
		if err := setupCache(); err != nil {
			return err
		}
	}

	return nil
}

func persistentPostRunE(cmd *cobra.Command, args []string) error {
	return cache.Close()
}

func setupBackend() error {
	backendType := storage.BackendType(viper.GetString("VSIX_BACKEND"))

	var err error

	switch backendType {
	case storage.BackendTypeFS:
		backend, err = storage.NewFSBackend(viper.GetString("VSIX_FS_DIR"))
		if err != nil {
			return err
		}
	case storage.BackendTypeS3:
		s3cfg, err := storage.NewS3Config(
			viper.GetString("VSIX_S3_URL"),
			viper.GetString("VSIX_S3_BUCKET"),
			viper.GetString("VSIX_S3_CREDENTIALS"),
			viper.GetString("VSIX_S3_PROFILE"),
		)
		if err != nil {
			return err
		}
		backend, err = storage.NewS3Backend(s3cfg)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown backend, must be fs or s3")
	}

	return nil
}

func setupCache() error {
	var err error
	cache, err = storage.OpenCache(viper.GetString("VSIX_CACHE_FILE"))
	if err != nil {
		return fmt.Errorf("error opening cache: %w", err)
	}
	return nil
}
