FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.8.0 AS xx

# FROM --platform=$BUILDPLATFORM registry.spagettikod.se:8443/macossdk:26 AS sdk

FROM --platform=$BUILDPLATFORM golang:1.25.4-alpine AS build
COPY --from=xx / /
LABEL vsix_intermediate=true
ARG VERSION
ARG DATE
ARG TARGETOS TARGETARCH
WORKDIR /vsix
COPY ./ .
RUN apk add clang lld
RUN xx-apk add musl-dev gcc binutils
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
ENV CGO_ENABLED=1
# RUN --mount=type=cache,target=/go/pkg/mod \
#     --mount=from=sdk,target=/xx-sdk,src=/xx-sdk \
#     xx-go build -tags fts5 -ldflags="-extldflags=-static -linkmode external -X main.version=$VERSION -X main.buildDate=$DATE" -o vsix . && \
#     xx-verify --static vsix
RUN --mount=type=cache,target=/go/pkg/mod \
    xx-go build -tags fts5 -ldflags="-extldflags=-static -linkmode external -X main.version=$VERSION -X main.buildDate=$DATE" -o vsix . && \
    xx-verify --static vsix

FROM scratch AS package
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
VOLUME [ "/data", "/cache" ]
WORKDIR /data
ENV VSIX_FS_DIR=/data
ENV VSIX_CACHE_FILE=/cache/vsix.sqlite
COPY --from=build /vsix/vsix /
ENTRYPOINT [ "/vsix" ]
