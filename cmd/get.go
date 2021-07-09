package cmd

import (
	"errors"
	"os"
	"path"

	"github.com/spagettikod/vsix/vscode"
	"github.com/spf13/cobra"
)

func init() {
	getCmd.Flags().StringVarP(&out, "output", "o", ".", "Output directory for downloaded files")
	getCmd.Flags().BoolVarP(&forceget, "force", "f", false, "Output directory for downloaded files")
	rootCmd.AddCommand(getCmd)
}

var getCmd = &cobra.Command{
	Use:   "get [flags] <identifier> [version]",
	Short: "Download extension",
	Long: `Get will download the extension from the Marketplace. Extension identifier
can be found on the Visual Studio Code Marketplace web page for a given extension
where it's called "Unique Identifier". If the extension is a "Extension Pack",
which is a collection of extentions, all those extension will also be downloaded
as well.

If version is not specified the latest version will be downloaded. The extension is
downloaded to the current directory unless the output-flag is set. Download is skipped
if the extension already exists in the output directory.

The command will exit with a non zero value if the extension can not be found or the
given version does not exist.`,
	Example: `  vsix get golang.Go
  vsix get golang.Go 0.17.0
  vsix get -o downloads golang.Go`,
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
	if exists, err := ext.FileExists(pe.Version, outputPath); !forceget && (exists || err != nil) {
		if exists {
			verboseLog.Printf("%s: skipping download, version already exist at output path\n", pe)
			return nil
		}
		return err
	}
	verboseLog.Printf("%s: downloading to %s", pe, path.Join(outputPath, ext.VsixFilename(pe.Version)))
	err = ext.Download(pe.Version, outputPath)
	if err != nil {
		return err
	}
	verboseLog.Printf("%s: saving metadata to %s", pe, path.Join(outputPath, ext.MetaFilename(pe.Version)))
	return ext.SaveMetadata(pe.Version, outputPath)
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
