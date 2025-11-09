FROM --platform=$BUILDPLATFORM tonistiigi/xx AS xx

FROM --platform=$BUILDPLATFORM golang:1.25.4 AS build
COPY --from=xx / /
LABEL vsix_intermediate=true
ARG VERSION
ARG DATE
ARG TARGETOS TARGETARCH
RUN xx-apt-get install -y binutils gcc libc6-dev
WORKDIR /vsix
COPY ./ .
RUN --mount=type=cache,target=/go/pkg/mod GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=1 \
    xx-go build -tags fts5 -ldflags="-extldflags=-static -linkmode external -X main.version=$VERSION -X main.buildDate=$DATE" -o vsix .

FROM scratch AS package
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
VOLUME [ "/data", "/cache" ]
WORKDIR /data
ENV VSIX_FS_DIR=/data
ENV VSIX_CACHE_FILE=/cache/vsix.sqlite
COPY --from=build /vsix/vsix /
ENTRYPOINT [ "/vsix" ]
