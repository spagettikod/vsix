VERSION=4.0.1
OUTPUT=_pkg
PWD=$(shell pwd)
.PHONY: build_linux build_macos pkg_linux pkg_macos all default clean setup docker test

default: help

help:
	@echo "Build targets for vsix $(VERSION)\n"
	@egrep -h '\s##\s' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

clean:					## Clean build artifacts
	@rm -rf $(OUTPUT)

build_linux: setup		## Build Linux executable for amd64
	@env GOOS=linux GOARCH=amd64 go build -o $(OUTPUT) -ldflags "-X main.version=$(VERSION)" vsix.go

build_macos: setup		## Build macOS executable for arm64
	@env GOOS=darwin GOARCH=arm64 go build -o $(OUTPUT) -ldflags "-X main.version=$(VERSION)" vsix.go

down:				## Shut down services used for development
	@-docker stop minio
	@-docker rm minio
	@-docker network rm vsixminionet

pkg_docker:				## Build and push Docker container for arm64 and amd64
	@docker buildx build --push --platform=linux/amd64,linux/arm64 -t spagettikod/vsix:$(VERSION) --build-arg VERSION=$(VERSION) .

pkg_linux: build_linux	## Build and package Linux executable for amd64
	@tar -C $(OUTPUT) -czf $(OUTPUT)/vsix$(VERSION).linux-amd64.tar.gz vsix

pkg_macos: build_macos	## Build and package macOS executable for arm64
	@tar -C $(OUTPUT) -czf $(OUTPUT)/vsix$(VERSION).macos-arm64.tar.gz vsix

setup:					## Setup and prepare for build
	@mkdir -p $(OUTPUT)

test:					## Run tests
	@docker build --target test . && docker rmi `docker image ls --filter label=vsix_intermediate=true -q`

up:						## Start services used for development
	@mkdir -p $(PWD)/_local/minio
	@mkdir -p $(PWD)/_local/.mc
	@docker network create vsixminionet
	@docker run -d --name minio --network vsixminionet -p 9000:9000 -p 9001:9001 -e MC_CONFIG_DIR=/root/.mc -e MINIO_ROOT_USER=vsixdev -e MINIO_ROOT_PASSWORD=vsixdevpw -v $(PWD)/_local/.mc:/root/.mc -v $(PWD)/_local/minio:/data quay.io/minio/minio server /data --console-address ":9001"
	@sleep 1
	@docker exec -it minio mc alias set vsixminio http://minio:9000 vsixdev vsixdevpw
	@docker exec -it minio mc mb vsixminio/exts
	@echo "Created MinIO alias called vsixminio, run . ./mcfunc.sh to get a bash function for the mc command with correct setup."
	@echo "Test the setup with: mc admin info vsixminio"

all: clean test pkg_linux pkg_macos pkg_docker
