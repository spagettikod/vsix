VERSION=5.0.0
OUTPUT=_pkg
PWD=$(shell pwd)
DATE=$(shell date -u -Iseconds)
.PHONY: build_linux build_macos pkg_linux pkg_macos all default clean setup docker test

default: help

help:
	@echo "Build targets for vsix $(VERSION)"
	@grep -E -h '\s##\s' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

clean:					## Clean build artifacts
	@rm -rf $(OUTPUT)

build: setup			## Build vsix for arm64 and amd64 through Docker
	@docker buildx build --output type=local,dest=$(OUTPUT) --platform=linux/amd64,linux/arm64 -t spagettikod/vsix:$(VERSION) --build-arg VERSION=$(VERSION) --build-arg DATE=$(DATE) .

docker:					## Build Docker container for arm64 and amd64
	@docker buildx build --output type=image --platform=linux/amd64,linux/arm64 -t spagettikod/vsix:$(VERSION) --build-arg VERSION=$(VERSION) --build-arg DATE=$(DATE) .

docker-dev:				## Build and push Docker container to development registry for arm64 and amd64
	@docker buildx build --output type=registry --platform=linux/amd64,linux/arm64 -t registry.spagettikod.se:8443/vsix:$(VERSION) --build-arg VERSION=$(VERSION) --build-arg DATE=$(DATE) .

down:					## Shut down services used for development
	@-docker stop minio
	@-docker rm minio
	@-docker network rm vsixminionet

install: 				## Install vsix locally
	@CGO_ENABLED=1 go install -tags fts5 -ldflags "-X main.version=$(VERSION) -X main.buildDate=$(DATE)" vsix.go

package: build			## Build and package Linux (amd64) and MacOS executables
	@tar -C $(OUTPUT_LINUX) -czf $(OUTPUT)/vsix$(VERSION).linux-amd64.tar.gz vsix
	@tar -C $(OUTPUT_MACOS) -czf $(OUTPUT)/vsix$(VERSION).macos-arm64.tar.gz vsix	

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
