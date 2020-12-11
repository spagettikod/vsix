# vsix
vsix is a CLI for Visual Studio Code Extension Marketplace. The tool can be useful to keep an off line stash of extension, for example in an air gapped environment.
Since the exact version does not need to be specified the tool can be run regularly to always download the latest version.

Features
* list available extension versions
* display extension information
* download extension, either a specific or the latest version
* extension pack sub-packages are also downloaded
* batch download multiple extensions specified in a text file or files in a sub directory

## Installation
vsix is distributed as a single binary file. The current release support the operating systems
and architectures below.

### macOS
This will install the latest version on macOS running on Intel.

```
curl -OL https://github.com/spagettikod/vsix/releases/download/v0.9.0/vsix0.9.0.macos-amd64.tar.gz
sudo tar -C /usr/local/bin -xvf vsix0.9.0.macos-amd64.tar.gz
```

### Linux
This will install the latest version on many Linux distros as long as you have curl installed.

```
curl -OL https://github.com/spagettikod/vsix/releases/download/v0.9.0/vsix0.9.0.linux-amd64.tar.gz
sudo tar -C /usr/local/bin -xvf vsix0.9.0.linux-amd64.tar.gz
```

## Usage
Most commands rely on knowing the "Unique Identifier" for a package. This identifier can be found in the "More Info" section on the Marketplace web page for an extension, for example [the Go extension](https://marketplace.visualstudio.com/items?itemName=golang.Go).
 
### `get`
```
Get will download the extension from the Marketplace. Extension identifier
can be found on the Visual Studio Code Marketplace web page for a given extension
where it's called "Unique Identifier". If the extension is a "Extension Pack",
which is a collection of extentions, all those extension will also be downloaded
as well.

If version is not specified the latest version will be downloaded. The extension is
downloaded to the current directory unless the output-flag is set. Download is skipped
if the extension already exists in the output directory.

The command will exit with a non zero value if the extension can not be found or the
given version does not exist.

Usage:
  vsix get [flags] <identifier> [version]

Examples:
  vsix get golang.Go
  vsix get golang.Go 0.17.0
  vsix get -o downloads golang.Go

Flags:
  -h, --help            help for get
  -o, --output string   Output directory for downloaded files (default ".")

Global Flags:
  -v, --verbose   verbose output
```

### `batch`
```
Batch will download all the extensions specified in a text file. If a directory is
given as input all text files in that directory (and its sub directories) will be parsed
in search of extensions to download.

Input file example:
  # This is a comment
  # This will download the latest version 
  golang.Go
  # This will download version 0.17.0 of the golang extension
  golang.Go 0.17.0

Extensions are downloaded to the current folder unless the output-flag is set.

The command will exit with a non zero value if one of the extensions can not be found
or a given version does not exist. These errors will be logged to standard error
output but the execution will not stop.

Usage:
  vsix batch <file|dir>

Examples:
  vsix batch my_extensions.txt
  vsix batch -o downloads my_extensions.txt

Flags:
  -h, --help            help for batch
  -o, --output string   Output directory for downloaded files (default ".")

Global Flags:
  -v, --verbose   verbose output
```