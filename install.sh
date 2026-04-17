#!/bin/sh
# Install the agiler CLI.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/agilercloud/cli/main/install.sh | sh
#
# Environment overrides:
#   AGILER_VERSION      version tag to install (default: latest)
#   AGILER_INSTALL_DIR  destination for the binary (default: $HOME/.local/bin)

set -eu

REPO="agilercloud/cli"
BINARY="agiler"
VERSION="${AGILER_VERSION:-latest}"
INSTALL_DIR="${AGILER_INSTALL_DIR:-${HOME}/.local/bin}"

err() { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*"; }

need() { command -v "$1" >/dev/null 2>&1 || err "required command not found: $1"; }
need curl
need tar
need uname

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
	darwin|linux) ;;
	*) err "unsupported OS: $os" ;;
esac

arch=$(uname -m)
case "$arch" in
	x86_64|amd64)  arch=x86_64 ;;
	arm64|aarch64) arch=arm64 ;;
	*) err "unsupported architecture: $arch" ;;
esac

if [ "$VERSION" = "latest" ]; then
	VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
		| sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
		| head -n1)
	[ -n "$VERSION" ] || err "failed to resolve latest version"
fi

archive="${BINARY}_${VERSION#v}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${VERSION}/${archive}"
checksums="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT TERM

info "downloading ${archive} (${VERSION})"
curl -fsSL -o "${tmpdir}/${archive}" "$url" || err "download failed: $url"
curl -fsSL -o "${tmpdir}/checksums.txt" "$checksums" || err "checksum download failed"

expected=$(grep " ${archive}\$" "${tmpdir}/checksums.txt" | awk '{print $1}' | head -n1)
[ -n "$expected" ] || err "checksum for ${archive} not found in checksums.txt"

if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "${tmpdir}/${archive}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "${tmpdir}/${archive}" | awk '{print $1}')
else
	err "no sha256 tool available (install coreutils or libressl)"
fi

[ "$expected" = "$actual" ] || err "checksum mismatch: expected $expected, got $actual"

tar -xzf "${tmpdir}/${archive}" -C "${tmpdir}"
[ -f "${tmpdir}/${BINARY}" ] || err "archive did not contain ${BINARY}"

mkdir -p "$INSTALL_DIR"
mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"

if [ "$os" = "darwin" ]; then
	xattr -d com.apple.quarantine "${INSTALL_DIR}/${BINARY}" 2>/dev/null || true
fi

info "installed ${BINARY} ${VERSION} -> ${INSTALL_DIR}/${BINARY}"

case ":${PATH}:" in
	*":${INSTALL_DIR}:"*) ;;
	*) info "note: ${INSTALL_DIR} is not in your PATH — add it to your shell rc to run 'agiler' without a full path" ;;
esac
