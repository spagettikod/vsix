FROM golang:1.17.3 AS test
LABEL vsix_intermediate=true
WORKDIR /vsix
COPY ./ .
RUN CGO_ENABLED=0 go test ./...

FROM golang:1.17.3 AS build
LABEL vsix_intermediate=true
ARG VERSION
WORKDIR /vsix
COPY ./ .
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=$VERSION" vsix.go

FROM scratch AS package
COPY --from=build /vsix/vsix /
ENTRYPOINT [ "/vsix" ]