package cmd

import (
	"errors"
	"os"

	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

func init() {
	getCmd.Flags().StringVarP(&out, "output", "o", ".", "Output directory for downloaded files")
	rootCmd.AddCommand(getCmd)
}

var getCmd = &cobra.Command{
	Use:                   "get [flags] <package> [version]",
	Short:                 "Download package",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		pe := parsedExtension{UniqueID: args[0]}
		if len(args) == 2 {
			pe.Version = args[1]
		}
		get(pe)
	},
}

func get(pe parsedExtension) {
	if err := download(pe, out); err != nil {
		errLog.Fatalf("%s: %s", pe, err)
	}
}

func download(pe parsedExtension, outputPath string) error {
	if exists, err := outputPathExists(outputPath); !exists {
		if err == nil {
			return errOutputPathNotFound
		}
		return err
	}
	verboseLog.Printf("%s: searching for extension at Marketplace", pe)
	ext, err := vscode.NewExtension(pe.UniqueID)
	if err != nil {
		return err
	}
	if ext.IsExtensionPack() {
		verboseLog.Printf("%s: is extension pack, getting pack contents", pe)
		for _, pack := range ext.ExtensionPack() {
			err := download(parsedExtension{UniqueID: pack}, outputPath)
			if err != nil {
				return err
			}
		}
	}
	if pe.Version == "" {
		pe.Version = ext.LatestVersion()
	}
	if !ext.HasVersion(pe.Version) {
		return errVersionNotFound
	}
	verboseLog.Printf("%s: found version %s", pe, pe.Version)
	if exists, err := ext.FileExists(pe.Version, outputPath); exists || err != nil {
		if exists {
			verboseLog.Printf("%s: skipping download, version already exist at output path\n", pe)
			return nil
		}
		return err
	}
	verboseLog.Printf("%s: downloading to %s", pe, outputPath)
	return ext.Download(pe.Version, outputPath)
}

func outputPathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
