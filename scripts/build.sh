#!/bin/sh
# Builds the single Designer binary: the frontend first (it is embedded), then Go.
set -eu
cd "$(dirname "$0")/.."
(cd web && npm ci && npm run check-icons && npm run build)
go vet ./...
go test ./...
mkdir -p bin
VERSION="${VERSION:-$(tr -d ' \r\n' < VERSION)-dev}"
go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o bin/designer$(go env GOEXE) ./cmd/designer
echo "built bin/designer$(go env GOEXE) $VERSION"
