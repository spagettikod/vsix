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
