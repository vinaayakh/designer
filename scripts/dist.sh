#!/bin/sh
# Builds release archives for every supported platform into dist/:
#   designer_<version>_<os>_<arch>.tar.gz (.zip on Windows), each with the binary, LICENSE,
#   NOTICE, THIRD_PARTY_NOTICES.md and README.md, plus checksums.txt (SHA-256).
#
#   scripts/dist.sh             # version from VERSION
#   VERSION=0.2.0 scripts/dist.sh
#   SKIP_WEB=1 scripts/dist.sh  # reuse an existing web/dist
set -eu
cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(tr -d ' \r\n' < VERSION)}"
VERSION="${VERSION#v}"
TARGETS="${TARGETS:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64}"

if [ -z "${SKIP_WEB:-}" ]; then
  (cd web && npm ci && npm run check-icons && npm run build)
fi
[ -f web/dist/index.html ] || { echo "web/dist is missing: build the frontend first" >&2; exit 1; }

rm -rf dist
mkdir -p dist/stage
for t in $TARGETS; do
  os=${t%/*}
  arch=${t#*/}
  ext=""
  [ "$os" = windows ] && ext=".exe"
  name="designer_${VERSION}_${os}_${arch}"
  stage="dist/stage/$name"
  mkdir -p "$stage"
  printf "building %-36s" "$name"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$stage/designer$ext" ./cmd/designer
  cp LICENSE NOTICE THIRD_PARTY_NOTICES.md README.md "$stage/"
  if [ "$os" = windows ]; then
    if command -v zip >/dev/null 2>&1; then
      (cd dist/stage && zip -qr "../$name.zip" "$name")
    elif [ -x "${SYSTEMROOT:-/c/Windows}/System32/tar.exe" ]; then # Windows' own bsdtar writes zips (Git Bash's GNU tar doesn't)
      (cd dist/stage && "${SYSTEMROOT:-/c/Windows}/System32/tar.exe" -a -cf "../$name.zip" "$name")
    else
      echo "zip is needed to package $name" >&2
      exit 1
    fi
  else
    tar -C dist/stage -czf "dist/$name.tar.gz" "$name"
  fi
  echo " $(du -h "$stage/designer$ext" | cut -f1)"
done
rm -rf dist/stage

cd dist
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum designer_* > checksums.txt
else
  shasum -a 256 designer_* > checksums.txt
fi
cat checksums.txt
