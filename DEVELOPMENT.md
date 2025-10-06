# Development stuff

## Snippets
List all extensions from an extension query result:
```bash
curl -s -X POST http://localhost:8080/sub/_apis/public/gallery/extensionquery | jq '.results[].extensions[] | "\(.publisher.publisherName).\(.extensionName)"'
```

Print asset URLs from an extension query result:
```bash
curl -s -X POST http://localhost:8080/sub/_apis/public/gallery/extensionquery | jq '.results[].extensions[].versions[].files[] | "\(.assetType) -> \(.source)"'
```

## Setup MinIO
```bash
docker run -d --rm --name minio -v /mnt/big/volumes/minio:/data -p 9000:9000 quay.io/minio/minio:RELEASE.2025-06-13T11-33-47Z server /data
```

```bash
. ./mcfunc.sh
mc alias set homer http://homer.spagettikod.se:9000 minioadmin minioadmin
mc admin info homer
mc mb homer/exts
```
