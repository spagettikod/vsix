#!/bin/bash

rm -rf _data
mkdir _data
VSIX_DEBUG=t go run vsix.go get -v -o _data -f golang.go 0.29.0
VSIX_DEBUG=t go run vsix.go get -v -o _data -f golang.go 0.28.0
VSIX_DEBUG=t go run vsix.go get -v -o _data -f vscodevim.vim 0.16.9
VSIX_DEBUG=t go run vsix.go get -v -o _data -f vscodevim.vim 0.0.10
VSIX_DEBUG=t go run vsix.go get -v -o _data -f vscodevim.vim 0.0.1
VSIX_DEBUG=t go run vsix.go get -v -o _data -f vscodevim.vim