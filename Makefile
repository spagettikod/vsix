VERSION=2.1.0
OUTPUT=_pkg
.PHONY: build_linux build_macos build_macos_intel pkg_linux pkg_macos pkg_macos_intel all default clean setup docker test

default: all

clean:
	@rm -rf $(OUTPUT)

pkg_docker_dev:
	@docker buildx build --push --platform=linux/amd64,linux/arm64 -t registry.spagettikod.se:8443/vsix:v$(VERSION)-dev --build-arg VERSION=$(VERSION) .

pkg_docker:
# @docker buildx create --use
	@docker buildx build --push --platform=linux/amd64,linux/arm64 -t spagettikod/vsix:v$(VERSION) -t spagettikod/vsix:latest --build-arg VERSION=$(VERSION) .

test:
	@docker build --target test . && docker rmi `docker image ls --filter label=vsix_intermediate=true -q`

setup:
	@mkdir -p $(OUTPUT)

build_docker:
	@docker build -t spagettikod/vsix:v$(VERSION) -t spagettikod/vsix:latest --build-arg VERSION=$(VERSION) .

build_linux: setup
	@env GOOS=linux GOARCH=amd64 go build -o $(OUTPUT) -ldflags "-X main.version=$(VERSION)" vsix.go

pkg_linux: build_linux
	@tar -C $(OUTPUT) -czf $(OUTPUT)/vsix$(VERSION).linux-amd64.tar.gz vsix

build_macos: setup
	@env GOOS=darwin GOARCH=arm64 go build -o $(OUTPUT) -ldflags "-X main.version=$(VERSION)" vsix.go

build_macos_intel: setup
	@env GOOS=darwin GOARCH=amd64 go build -o $(OUTPUT) -ldflags "-X main.version=$(VERSION)" vsix.go

pkg_macos: build_macos
	@tar -C $(OUTPUT) -czf $(OUTPUT)/vsix$(VERSION).macos-arm64.tar.gz vsix

pkg_macos_intel: build_macos_intel
	@tar -C $(OUTPUT) -czf $(OUTPUT)/vsix$(VERSION).macos-amd64.tar.gz vsix

all: clean test pkg_linux pkg_macos pkg_macos_intel