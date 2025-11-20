# vsix
Host your private [Marketplace](https://marketplace.visualstudio.com/) by downloading and serving extensions to Visual Studio Code in your own environment.

## Installation
vsix is distributed as a single binary file and a container image. The current release support the operating systems and architectures below.

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
Start by looking for some extensions you might be interested in. The query is the same as the search box in Visual Studio Code.

```
vsix search golang
```

Let's say we would like to add the Go extension from the Go Team at Google to our marketplace. We do this by finding the unique identifier and the `add` command.

```
vsix add golang.Go
```

> [!TIP]
> You can add multiple extensions at once, `vsix add golang.Go ms-python.python`. Or combine commands like `vsix add $(vsix search --sort install --limit 10 --quiet)`, the example will att the 10 most installed extensions.

When our extensions are added to our marketplace we can run `list` to see what we have.

```
vsix list
```

> [!TIP]
> By default only the latest version is showed, regardless of platform, to list all, non pre-relase version across all platforms run `vsix info --all`. To get a complete list also add `--pre-release` to also include any versions that are marked as pre-release.

Now that we have a local copy of an extension we can start our own marketplace.

```
vsix serve
```

If you don't see any errors you now have your own marketplace running at `http://localhost:8080`. To try it out and see if it responds as the official marketplace we can use the search command and point it at our own server.

```
vsix search --endpoint http://localhost:8080
```

You should see a list of all the extension you added, just like you would have search the official marketplace.

From time to time you will want to update your local extensions. To see and download updates to your local marketplace run the update command.

```
vsix update
```

Read below to further to customize your setup and how to use Visual Studio Code with your server.

## Use your own marketplace from Visual Studio Code
There are two ways you can make use of vsix as your local marketplace.

1. Edit your local Visual Studio Code configuration. The downside of this option each update will required you to make the changes again.
1. Spoof `marketplace.visualstudio.com` in your local network.

### Edit your local Visual Studio Code configuration
To make ou can modify `product.json` in your Visual Studio Code installation. The former has the benefit of not having to modify your local Visual Studio Code installation while the latter one needs to be reapplied after each update of Visual Studio Code.

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

### Spoofing the official marketplace
Spoofing the official marketplace gives each client seamless access to your marketplace and does not require modifying settings after each update of Visual Studio Code.

This option can be done in two ways, both require you to distribute and add trust to a local CA that can issue a local certificate for `marketplace.visualstudio.com`.

You can then either:
1. modify your local DNS to point `marketplace.visualstudio.com` at your local vsix server or
1. each client can add your server to their local `hosts` file.

The latter one is easier if you want each client to have the option of switching between the real marketplace and the local one.

## Configure vsix
To configure vsix you use environment variables. These can be set at each execution or added to a `.env` file vsix can read at startup. Run `vsix info` to see where environment files will be read from and to see what settings has been applied.

|Name|Description|
|---|---|
|**VSIX_BACKEND**|Backend to use when storing extensions. Valid options are `fs` (default) or `s3`.|
|**VSIX_CACHE_FILE**|Folder where the extension cache files (SQLite database) is stored. When not running in a container it defaults to a location in the user's home folder depending on your platform.|
|**VSIX_FS_DIR**|Folder where extensions are stored when stored on the file system, `VSIX_BACKEND` is set to `fs`. When not running in a container it defaults to a location in the user's home folder depending on your platform.|
|**VSIX_LOG_DEBUG**|Output debug information when running vsix. Valid values are  `true` or `false`. Default is `false`.|
|**VSIX_LOG_VERBOSE**|Output logs and suppress other forms of output like progress bars when running vsix. Valid values are  `true` or `false`. Default is `false`.|
|**VSIX_PLATFORMS**|Limit which platforms to download when running the add-command. This is a space separated list of values within square brackets. For example *[universal linux-x64 win32-x64]*. Only the specified platforms will be downloaded. Default is a empty list and all platforms will be downloaded.|
|**VSIX_S3_BUCKET**|S3 bucket where extensions will be stored when `VSIX_BACKEND` is set to `s3`. Default value is empty, value is required when using S3.|
|**VSIX_S3_CREDENTIALS**|Location of the credentials file used when `VSIX_BACKEND` is set to `s3`. The expected format is the same used by AWS. Default value is empty, value is required when using S3.|
|**VSIX_S3_PREFIX**|Prefix to be added when using S3 as backend. If not set files will be stored at bucket root. Default value is empty.|
|**VSIX_S3_PROFILE**|Name of the profile to use in the credentials file (see `VSIX_S3_CREDENTIALS`) when using S3 as backend. Default value is `default`.|
|**VSIX_S3_URL**|URL to the S3 server when using S3 as backend. Default value is `http://localhost:9000`.|
|**VSIX_SERVE_ADDR**|Address where the serve-command will listen for connections. Default value is `0.0.0.0:8080`.|
|**VSIX_SERVE_URL**|URL to the vsix server. This value is used when rewriting extension URL's so Visual Studio Code finds its way back to your marketplace when installing extensions. When [spoofing](#spoofing-the-official-marketplace) the official marketplace this should be set to `https://marketplace.visualstudio.com`. Default value is `http://localhost:8080`.|