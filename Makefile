VERSION=1.0.0
OUTPUT=_pkg
.PHONY: build_linux build_macos pkg_linux pkg_macos all default clean setup

default: all

clean:
	@rm -rf $(OUTPUT)

setup:
	@mkdir -p $(OUTPUT)

build_linux: setup
	@env GOOS=linux GOARCH=amd64 go build -o $(OUTPUT) -ldflags "-X main.version=$(VERSION)" vsix.go

pkg_linux: build_linux
	@tar -C $(OUTPUT) -czf $(OUTPUT)/vsix$(VERSION).linux-amd64.tar.gz vsix

build_macos: setup
	@env GOOS=darwin GOARCH=amd64 go build -o $(OUTPUT) -ldflags "-X main.version=$(VERSION)" vsix.go

pkg_macos: build_macos
	@tar -C $(OUTPUT) -czf $(OUTPUT)/vsix$(VERSION).macos-amd64.tar.gz vsix

all: clean pkg_linux pkg_macos