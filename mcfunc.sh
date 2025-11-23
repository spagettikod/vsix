#!/bin/bash

mc() {
    docker run \
        --rm \
        -it \
        --network vsixminionet \
        --workdir /host \
        -v $(pwd):/host \
        -v $(pwd)/_local/.mc:/root/.mc \
        -e MC_CONFIG_DIR=/root/.mc \
        --entrypoint mc \
        quay.io/minio/minio "$@"
}
