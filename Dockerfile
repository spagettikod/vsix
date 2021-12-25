FROM golang:1.17.3 AS test
LABEL vsix_intermediate=true
WORKDIR /vsix
COPY ./ .
RUN CGO_ENABLED=0 go test -ldflags="-extldflags=-static" ./...

FROM --platform=$BUILDPLATFORM golang:1.17.3 AS build
LABEL vsix_intermediate=true
ARG VERSION
ARG TARGETOS TARGETARCH
WORKDIR /vsix
COPY ./ .
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -ldflags="-extldflags=-static" -ldflags "-X main.version=$VERSION" vsix.go

FROM scratch AS package
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /vsix/vsix /
WORKDIR /data
VOLUME [ "/data" ]
ENTRYPOINT [ "/vsix" ]