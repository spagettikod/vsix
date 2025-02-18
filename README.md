# vsix
Host your private [Marketplace](https://marketplace.visualstudio.com/) by downloading and serving extensions to Visual Studio Code in your own environment.

## Installation
vsix is distributed as a single binary file. The current release support the operating systems and architectures below.

<details>
<summary>macOS (Apple Silicon)</summary>

```shell
curl -OL https://github.com/spagettikod/vsix/releases/download/4.0.0/vsix4.0.0.macos-arm64.tar.gz
sudo tar -C /usr/local/bin -xvf vsix4.0.0.macos-arm64.tar.gz
```
</details>

<details>
<summary>Linux</summary>

```shell
curl -OL https://github.com/spagettikod/vsix/releases/download/4.0.0/vsix4.0.0.linux-amd64.tar.gz
sudo tar -C /usr/local/bin -xvf vsix4.0.0.linux-amd64.tar.gz
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
      spagettikod/vsix update
```
</details>

## Getting Started
Create a folder where you store downloaded extensions.

```
mkdir my_extensions
```

Add a few extensions you would like to have available off-line.

```
vsix add --data my_extensions golang.Go gruntfuggly.todo-tree
```

Start your own marketplace serving the extensions you added above.

```
vsix serve --data my_extensions https://vsix.myserver.com:8443/_apis/public/gallery
```

If you run your own DNS you can hijack `marketplace.visualstudio.com` and point it to your local server or you can modify `product.json` in your Visual Studio Code installation. The former has the benefit of not having to modify your local Visual Studio Code installation while the latter one needs to be reapplied after each update of Visual Studio Code. 

1. Open `product.json`. On macOS (if Visual Studio Code is installed in the Applications folder) this file is located at `/Applications/Visual Studio Code.app/Contents/Resources/app/product.json`.
1. Find the `extensionGallery` block and edit the `serviceUrl` to point to your server. For example, using the example above `https://vsix.myserver.com:8443/_apis/public/gallery`:
      ```json
      ...
      "extensionsGallery": {
		"serviceUrl": "https://marketplace.visualstudio.com/_apis/public/gallery",
	},
      ...
      ```
1. Restart Visual Studio Code and start using extensions from your own marketplace.

## Update extensions
To update and fetch the latest version of the extensions on your local marketplace you run the update command.

```
vsix update --data extensions
```

This will check for newer versions and download the most current one of your extensions if needed. It will only download the latest version, not all versions inbetween your latest version and the latest version at Visual Studio Code Marketplace.

## Multiple platforms
Some extensions support multiple platforms. If you don't have or use all platforms you can limit which platforms you want to add. When you run the `update`-command it will only update those platforms that were added. If you want to add a platform later on you can add it by running the `add` command again.

This example will add an extension limiting it to the `darwin-arm64` platform.
```shell
vsix --data extensions add --platforms darwin-arm64 redhat.java
```

When adding multiple extensions and limiting to certain platforms you must remember to include the `universal` platform, which most extensions belong to. Otherwise these will not be added. A good rule of thumb is to always add all platforms you want to support, regardless of extension.
```shell
vsix --data extensions add --platforms universal,darwin-arm64,linux-amd64,win32-arm64 golang.Go redhat.java
```

You can use the `info` sub command to check which platforms are supported for a certain extension. 

## Pre-release
Some extensions provide pre-release versions. These can in some cases be updated quite frequently. By default pre-release versions will not be downloaded by the `add` and `update` command. If you want an update to include pre-releases  
```shell
vsix --data extensions --pre-release golang.Go ms-python.python
```
