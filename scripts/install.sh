#!/bin/sh
# Installs Designer from a GitHub release (macOS, Linux).
#
#   curl -fsSL https://raw.githubusercontent.com/vinaayakh/designer/main/scripts/install.sh | sh
#
# Environment:
#   VERSION      version to install, e.g. 0.2.0 (default: the latest release)
#   INSTALL_DIR  where the binary goes (default: ~/.local/bin)
#   DESIGNER_REPO  owner/repo to download from (default: vinaayakh/designer)
#   DESIGNER_DOWNLOAD_URL  a mirror holding the release files (needs VERSION)
set -eu

REPO="${DESIGNER_REPO:-vinaayakh/designer}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-latest}"

say() { printf ' %s\n' "$1"; }
die() { printf ' error: %s\n' "$1" >&2; exit 1; }

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported OS $(uname -s); on Windows use install.ps1" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) die "unsupported architecture $(uname -m)" ;;
esac

if command -v curl >/dev/null 2>&1; then
  get() { curl -fsSL --retry 2 "$1"; }
  save() { curl -fsSL --retry 2 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  get() { wget -qO- "$1"; }
  save() { wget -qO "$2" "$1"; }
else
  die "curl or wget is needed"
fi

if [ "$VERSION" = latest ]; then
  VERSION="$(get "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n 1)"
  [ -n "$VERSION" ] || die "couldn't find the latest release of $REPO"
fi
VERSION="${VERSION#v}"

name="designer_${VERSION}_${os}_${arch}"
base="${DESIGNER_DOWNLOAD_URL:-https://github.com/$REPO/releases/download/v$VERSION}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

say "Downloading Designer $VERSION ($os/$arch)"
save "$base/$name.tar.gz" "$tmp/$name.tar.gz" || die "download failed: $base/$name.tar.gz"
save "$base/checksums.txt" "$tmp/checksums.txt" || die "download failed: $base/checksums.txt"

want="$(grep " \*\{0,1\}$name.tar.gz\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
[ -n "$want" ] || die "$name.tar.gz is not in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  have="$(sha256sum "$tmp/$name.tar.gz" | cut -d' ' -f1)"
else
  have="$(shasum -a 256 "$tmp/$name.tar.gz" | cut -d' ' -f1)"
fi
[ "$want" = "$have" ] || die "checksum mismatch for $name.tar.gz"
say "Checksum verified"

tar -xzf "$tmp/$name.tar.gz" -C "$tmp"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/$name/designer" "$INSTALL_DIR/designer" 2>/dev/null || { cp "$tmp/$name/designer" "$INSTALL_DIR/designer" && chmod 0755 "$INSTALL_DIR/designer"; }
say "Installed $INSTALL_DIR/designer"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) say "Add $INSTALL_DIR to your PATH, e.g.: export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac
command -v claude >/dev/null 2>&1 || say "Designer runs its AI work in Claude Code. Install it and log in: https://claude.com/claude-code"
say "Start it with: designer ~/my-system"
