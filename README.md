# vsix
Host your private [Marketplace](https://marketplace.visualstudio.com/) by downloading and serving extensions to Visual Studio Code in your own environment. The tool can be useful when working in air gapped environments but you still want access to Visual Studio Code extensions.

## Installation
vsix is distributed as a single binary file. The current release support the operating systems and architectures below.

<details>
<summary>macOS (Apple Silicon)</summary>

```shell
curl -OL https://github.com/spagettikod/vsix/releases/download/v2.2.0/vsix2.2.0.macos-arm64.tar.gz
sudo tar -C /usr/local/bin -xvf vsix2.2.0.macos-arm64.tar.gz
```
</details>

<details>
<summary>Linux</summary>

```shell
curl -OL https://github.com/spagettikod/vsix/releases/download/v2.2.0/vsix2.2.0.linux-amd64.tar.gz
sudo tar -C /usr/local/bin -xvf vsix2.2.0.linux-amd64.tar.gz
```
</details>

<details>
<summary>Docker</summary>

```docker
docker run --rm -it spagettikod/vsix info golang.Go
```

You can bind mount folders to `/data` in the container when working with downloaded extensions.

```docker
docker run --rm -it \
      -v $(pwd):/data \
      spagettikod/vsix update /my_extensions_to_sync
```
</details>

## Getting Started
Create a folder where you store downloaded extensions.

```
mkdir extensions
```

Add a few extensions you would like to have available off-line.

```
vsix add --data extensions golang.Go gruntfuggly.todo-tree
```

Start your own marketplace serving the extensions you added above.

```
vsix serve --data extensions https://localhost:8080_apis/public/gallery
```

If you run your own DNS you can hijack `marketplace.visualstudio.com` and point it to your local server

1. Open `product.json`. On macOS (if Visual Studio Code is installed in the Applications folder) this file is located at `/Applications/Visual Studio Code.app/Contents/Resources/app/product.json`.
1. Find the `extensionGallery` block and edit the `serviceUrl` to point to your server. For example, using the example above `https://vsix.myserver.com/extensions`:
      ```json
      ...
      "extensionsGallery": {
		"serviceUrl": "https://marketplace.visualstudio.com/_apis/public/gallery",
	},
      ...
      ```
1. Restart Visual Studio Code and start using extensions from your own marketplace.

## Usage
Most commands rely on knowing the "Unique Identifier" for a package. This identifier can be found in the "More Info" section on the Marketplace web page for an extension, for example [the Go extension](https://marketplace.visualstudio.com/items?itemName=golang.Go). It is also visible when running the `search` command.

There is build in documentation in the tool itself by running `vsix <command> --help`.

### `add`
Add extensions to your local storage. This will download the latest version of the extension and create necessary metadata to later serve the extension locally.

```shell
vsix --data extensions add golang.Go
```

You can add multiple extensions at once:
```shell
vsix --data extensions add golang.Go ms-python.python
```

#### Multiple platforms
Some extensions support multiple platforms. If you don't have or use all platforms you can limit which platforms you want to add. When you run the `update`-command it will only update those platforms that were added. If you want to add a platform later on you can add it by running the `add` command again.

This example will add an extension limiting it to the `darwin-arm64` platform.
```shell
vsix --data extensions add --platforms darwin-arm64 redhat.java
```

When adding multiple extensions and limiting to certain platforms you must remember to include the `universal` platform, which most extensions belong to. Otherwise these will be added. A good rule of thumb is to always add all platforms you want to support.
```shell
vsix --data extensions add --platforms universal,darwin-arm64,linux-amd64,win32-arm64 golang.Go redhat.java
```

#### Pre-release
```shell
vsix --data extensions add golang.Go ms-python.python
```


## Running your local Marketplace
By syncing extensions from the official maketplace and hosting your own marketplace you can run a local proxy with extensions. To achieve this we will do the following:

1. Setup sync with the official marketplace
1. Start a local marketplace server
1. Point Visual Studio Code to your server

All examples will use Docker but you can easily replace this with your specific platform build.

### Setup sync with the official marketplace
To have something to serve from your local marketplace we first need to fetch some extensions from the official marketplace.

1. Start by creating a folder somewhere where the "database" is stored.
      ```
      mkdir data
      ```
1. Create a file with extensions to be synced, let's call it `extensions` and add some extensions. Here we fetch the latest versions of the Go and Java extensions, where Java is actually an extension pack with multiple extensions.
      ```
      golang.Go
      vscjava.vscode-java-pack
      ```
1. Synchronize your "database" with the marketplace. In this example we will run it once but in a real setup you'll want to use cron to run at regular intervals to always have the latest versions. \
The example below won't load pre-release versions of extensions, add the `--pre-release` flag to the command to fetch pre-release versions.
      ```
      docker run \
            -v $(pwd)/data:/data
            -v $(pwd)/extensions:/extensions
            spagettikod/vsix -o /data --platforms linux-x86,darwin-arm64 /extensions
      ```

### Start a local marketplace server
Now that you've setup synchronization and have some nice extensions in your "database" you'll want to serve them to your Visual Studio Code editor.

Although you can serve a marketplace without TLS I recommend fetching a server certificate from [Let's Encrypt](https://letsencrypt.org/). There's a nice client called [Lego](https://github.com/go-acme/lego) I like to use.

The vsix Docker image will automatically run the `serve` sub command if no other parameters are given. It will run `serve` pointing the "database" to `/data` and the server certificate and key to files named `/server.crt` and `/server.key`. So all you have to do is bind mount those files.

You will also need to specify the endpoint where extensions and assets can be downloaded from, for example `https://vsix.myserver.com/extensions`. If you run your own DNS and hijack the offical domain this should say `https://marketplace.visualstudio.com/_apis/public/gallery`.

```
docker run -d \
      -v $(pwd)/data:/data
      -v <PATH TO YOUR CERTIFICATE>:/server.cert:ro
      -v <PATH TO YOUR KEY>:/server.key:ro
      -e VSIX_EXTERNAL_URL=<YOUR SERVER ENDPOINT>
      spagettikod/vsix
```

### Point Visual Studio Code to your server
If you run your own DNS and hijack `marketplace.visualstudio.com` you are set to go and won't need to modify Visual Studio Code to use your own marketplace proxy.

If you don't run your own DNS you will have to modify your Visual Studio Code configuration.
1. Open `product.json`. On macOS (if Visual Studio Code is installed in the Applications folder) this file is located at `/Applications/Visual Studio Code.app/Contents/Resources/app/product.json`.
1. Find the `extensionGallery` block and edit the `serviceUrl` to point to your server. For example, using the example above `https://vsix.myserver.com/extensions`:
      ```json
      ...
      "extensionsGallery": {
		"serviceUrl": "https://marketplace.visualstudio.com/_apis/public/gallery",
	},
      ...
      ```
1. Restart Visual Studio Code and start using extensions from your own marketplace.

## Commands
Besides serving your offline marketplace vsix has some useful commands to interact with the offical Visual Studo Marketplace and to interact with your marketplace "database".

### `dump`
Prints all extensions found in the "database" at the given path.

```bash
$ docker run --rm -it -v $(pwd):/data spagettikod/vsix get golang.Go
```

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
$ vsix serve --data _data --cert myserver.crt --key myserver.key https://www.example.com/vsix
```

```bash
$ docker run -d -p 8443:8443 \
      -v $(pwd):/data \
      -v myserver.crt:/myserver.crt:ro \
      -v myserver.key:/myserver.key:ro \
      spagettikod/vsix serve https://my.vsixserver.net:8443
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
