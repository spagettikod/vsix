VERSION=3.0.0
OUTPUT=_pkg
.PHONY: build_linux build_macos pkg_linux pkg_macos all default clean setup docker test

default: test

clean:
	@rm -rf $(OUTPUT)

test:
	@docker build --target test . && docker rmi `docker image ls --filter label=vsix_intermediate=true -q`

setup:
	@mkdir -p $(OUTPUT)

build_docker:
	@docker build -t spagettikod/vsix:v$(VERSION) -t spagettikod/vsix:latest --build-arg VERSION=$(VERSION) .

build_linux: setup
	@env GOOS=linux GOARCH=amd64 go build -o $(OUTPUT) -ldflags "-X main.version=$(VERSION)" vsix.go

build_macos: setup
	@env GOOS=darwin GOARCH=arm64 go build -o $(OUTPUT) -ldflags "-X main.version=$(VERSION)" vsix.go

pkg_linux: build_linux
	@tar -C $(OUTPUT) -czf $(OUTPUT)/vsix$(VERSION).linux-amd64.tar.gz vsix

pkg_macos: build_macos
	@tar -C $(OUTPUT) -czf $(OUTPUT)/vsix$(VERSION).macos-arm64.tar.gz vsix

pkg_docker_dev: test
	@docker buildx build --push --platform=linux/amd64,linux/arm64 -t registry.spagettikod.se:8443/vsix:$(VERSION)-dev --build-arg VERSION=$(VERSION) .

pkg_docker:
# @docker buildx create --use
	@docker buildx build --push --platform=linux/amd64,linux/arm64 -t spagettikod/vsix:$(VERSION) -t spagettikod/vsix:latest --build-arg VERSION=$(VERSION) .

all: clean test pkg_linux pkg_macos pkg_docker
