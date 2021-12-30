# vsix
vsix is a CLI for Visual Studio Code Extension Marketplace. The tool can be useful to keep an off line stash of extension, for example in an air gapped environment.

You can keep a folder in sync with the Marketplace by specifying a list of extensions in a text file. These files can then be served internally using the `serve` command.

## Features
* [get](#get) a VSIX package from the Marketplace to install locally in VS Code
* [search](#search) for extensions by name
* keep folder in [sync](#sync) with the marketplace
* [serve](#serve) downloaded extensions locally
* list available extension [versions](#versions)
* display extension [information](#info) 

## Installation
vsix is distributed as a single binary file. The current release support the operating systems
and architectures below.

### Docker
There is a Docker image available with `vsix`.

```docker
docker run --rm -it spagettikod/vsix info golang.Go
```

You can bind mount folders to `/data` in the container when syncing extensions.

```docker
docker run --rm -it spagettikod/vsix info golang.Go
```

### macOS
This will install the latest version on macOS running on Intel.

```
curl -OL https://github.com/spagettikod/vsix/releases/download/v1.0.0/vsix1.0.0.macos-amd64.tar.gz
sudo tar -C /usr/local/bin -xvf vsix1.0.0.macos-amd64.tar.gz
```

### Linux
This will install the latest version on many Linux distros as long as you have curl installed.

```
curl -OL https://github.com/spagettikod/vsix/releases/download/v1.0.0/vsix1.0.0.linux-amd64.tar.gz
sudo tar -C /usr/local/bin -xvf vsix1.0.0.linux-amd64.tar.gz
```

## Usage
Most commands rely on knowing the "Unique Identifier" for a package. This identifier can be found in the "More Info" section on the Marketplace web page for an extension, for example [the Go extension](https://marketplace.visualstudio.com/items?itemName=golang.Go). It is also visible when running the `search` command.

There is build in documentation in the tool itself by running `vsix <command> --help`.

### `get`
Get will download the extension from the Marketplace. Extension identifier
can be found on the Visual Studio Code Marketplace web page for a given extension
where it's called "Unique Identifier". If the extension is an "Extension Pack",
which is a collection of extentions, all those extension will also be downloaded
as well.

If version is not specified the latest version will be downloaded. The extension is
downloaded to the current directory unless the output-flag is set. Download is skipped
if the extension already exists in the output directory.

The command will exit with a non zero value if the extension can not be found or the
given version does not exist.

#### Examples
Download the latest version of the golang.Go extension.
```bash
$ docker run --rm -it -v $(pwd):/data spagettikod/vsix get golang.Go
```

Download version 0.17.0 of the golang.Go extension to the current directory.
```
$ vsix get golang.Go 0.17.0
```

Download the latest version of the golang.Go extension to the `downloads` directory.
```
$ vsix get -o downloads golang.Go
```

### `info`
Display package information.

#### Example
```bash
$ docker run --rm -it spagettikod/vsix info golang.Go
Name:           Go
Publisher:      Go Team at Google
Latest version: 0.30.0
Released on:    2015-10-15 17:20 UTC
Last updated:   2021-12-16 17:03 UTC
Extension pack: 

Rich Go language support for Visual Studio Code
```

### `search`
Search for extensions that matches query.

#### Example
```
$ docker run --rm -it spagettikod/vsix search golang
UNIQUE ID                               NAME                            PUBLISHER                       LATEST VERSION  LAST UPDATED            INSTALLS        RATING     
golang.Go                               Go                              Go Team at Google               0.30.0          2021-12-16T17:03:11Z    6068389         4.34 (221)
kiteco.kite                             Kite AutoComplete AI Code: ...  Kite                            0.147.0         2021-06-03T15:27:45Z    3165991         3.20 (122)
TabNine.tabnine-vscode                  Tabnine AI Autocomplete for...  TabNine                         3.5.11          2021-12-19T09:41:40Z    3000445         4.34 (359)
golang.go-nightly                       Go Nightly                      Go Team at Google               2021.12.2121    2021-12-22T15:25:33Z    113193          5.00 (2)  
mikegleasonjr.theme-go                  Go Themes (playground & src)    Mike Gleason jr Couturier       0.0.3           2016-11-03T19:58:42Z    57469           5.00 (3)  
casualjim.gotemplate                    gotemplate-syntax               casualjim                       0.4.0           2020-05-08T19:04:29Z    56782           4.18 (11) 
aldijav.golangwithdidi                  Golang                          aldijav                         0.0.1           2021-01-23T10:27:33Z    38530           5.00 (1)  
joaoacdias.golang-tdd                   Golang TDD                      joaoacdias                      0.0.9           2017-05-09T13:54:17Z    31920           2.50 (2)  
yokoe.vscode-postfix-go                 Golang postfix code completion  yokoe                           0.0.12          2018-11-03T16:38:36Z    30868           5.00 (2)  
neverik.go-critic                       Go Critic                       neverik                         0.1.0           2018-12-06T10:13:12Z    28879           -         
HCLTechnologies.hclappscancodesweep     HCL AppScan CodeSweep           HCL Technologies                1.2.0           2021-11-30T14:50:21Z    18204           4.67 (9)  
zsh.go-snippets                         go snippets                     zsh                             0.0.4           2020-08-05T10:00:29Z    16783           5.00 (1)  
myax.appidocsnippets                    ApiDoc Snippets                 Miguel Yax                      0.1.20          2020-03-07T13:45:07Z    16320           5.00 (3)  
carolynvs.dep                           dep                             Carolyn Van Slyck               0.1.0           2018-02-04T17:46:21Z    15188           5.00 (1)  
doggy8088.go-extension-pack             Go Extension Pack               Will 保哥                       0.12.3          2021-12-16T07:44:30Z    13247           5.00 (2)  
skip1.go-swagger                        go-swagger                      skip1                           2.0.1           2019-09-27T09:55:44Z    9580            -         
vitorsalgado.vscode-glide               VS Code Glide                   Vitor Hugo Salgado              1.0.2           2016-08-10T03:11:06Z    9141            -         
ethan-reesor.vscode-go-test-adapter     Go Test Explorer                Ethan Reesor                    0.1.6           2021-07-21T22:21:02Z    8953            5.00 (1)  
xmtt.go-mod-grapher                     Go Mod Grapher                  xmtt                            1.1.1           2019-09-03T18:33:38Z    8777            5.00 (1)  
JFrog.jfrog-vscode-extension            JFrog                           JFrog                           1.9.1           2021-12-05T13:38:43Z    8539            5.00 (4)  
```
### `serve`
This command will start a HTTPS server that is compatible with Visual Studio Code. When setup you can browse, search and install extensions previously downloaded using the sync command. If sync is run and new extensions are downloaded while the server is running it will automatically update with the newly downloaded extensions. 

To enable Visual Studio Code integration you must change the tag serviceUrl in the file project.json in your Visual Studio Code installation. On MacOS, for example, the file is located at /Applications/Visual Studio Code.app/Contents/Resources/app/product.json. Set the URL to your server, for example https://vsix.example.com:8080.

#### Example
```bash
$ vsix serve --data _data https://www.example.com/vsix myserver.crt myserver.key
```

```bash
$ docker run -d -p 8443:8443 \
      -v $(pwd):/data \
      -v myserver.crt:/myserver.crt:ro \
      -v myserver.key:/myserver.key:ro \
      spagettikod/vsix serve /myserver.crt /myserver.key
```

### `sync`
Sync will download all the extensions specified in a text file. If a directory is given as input all text files in that directory (and its sub directories) will be parsed in search of extensions to download.

Input file example:
```bash
  # This is a comment
  # This will download the latest version 
  golang.Go
  # This will download version 0.17.0 of the golang extension
  golang.Go 0.17.0
```

Extensions are downloaded to the current folder unless the output-flag is set.

The command will exit with exit code 78 if one of the extensions can not be found or a given version does not exist. These errors will be logged to stderr output but the execution will not stop.

#### Example
```bash
$ docker run -d \
      -v $(pwd):/data \
      -v my_extensions_to_sync:/my_extensions_to_sync
      spagettikod/vsix sync /my_extensions_to_sync
```

### `versions`
List avilable versions for an extension.

```bash
$ docker run --rm -it spagettikod/vsix versions golang.Go
```
```
$ vsix version golang.Go
```