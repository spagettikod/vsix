# FROM golang:1.23.5 AS test
# LABEL vsix_intermediate=true
# WORKDIR /vsix
# COPY ./ .
# RUN CGO_ENABLED=1 go test -tags fts5 -ldflags="-extldflags=-static" ./...

FROM golang:1.23.5 AS common-build
ARG VERSION
ARG TARGETOS TARGETARCH
WORKDIR /vsix
COPY ./ .
ENV CGO_ENABLED=1
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

FROM common-build AS backend-linux-amd64
RUN dpkg --add-architecture amd64 \
    && apt-get update \
    && apt-get install -y --no-install-recommends gcc-x86-64-linux-gnu libc6-dev-amd64-cross
RUN CC=x86_64-linux-gnu-gcc go build -tags fts5 -ldflags="-extldflags=-static -linkmode external -X main.version=$VERSION" -o vsix .

FROM common-build AS backend-linux-arm64
RUN apt-get update && \
    apt-get install -y --no-install-recommends gcc-aarch64-linux-gnu && \
    rm -rf /var/lib/apt/lists/*
RUN go build -tags fts5 -ldflags="-extldflags=-static -linkmode external -X main.version=$VERSION" -o vsix .

# FROM --platform=$BUILDPLATFORM golang:1.23.5 AS build
# LABEL vsix_intermediate=true
# ARG VERSION
# ARG TARGETOS TARGETARCH
# WORKDIR /vsix
# COPY ./ .
# RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=1 \
#     go build -tags fts5 -ldflags="-extldflags=-static -linkmode external -X main.version=$VERSION" -o vsix .

FROM scratch AS common-package
COPY --from=common-build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
VOLUME [ "/data", "/cache" ]
WORKDIR /data
ENV VSIX_FS_DIR=/data
ENV VSIX_CACHE_FILE=/cache/vsix.sqlite
ENTRYPOINT [ "/vsix" ]

FROM --platform=amd64 common-package
COPY --from=backend-linux-amd64 /vsix/vsix /

FROM --platform=arm64 common-package
COPY --from=backend-linux-arm64 /vsix/vsix /
