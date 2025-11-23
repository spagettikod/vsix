#!/bin/sh

vsix() {
    docker run --rm -it \
        --user 1000:1000 \
        spagettikod/vsix:5.0.0-beta "$@"
}
