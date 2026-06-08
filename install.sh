#!/usr/bin/env bash
set -euo pipefail

# Install fox-control binary from GitHub Releases.
# Usage: curl -fsSL https://raw.githubusercontent.com/fox-in-the-box-ai/fox-fleet/main/install.sh | bash
#   FOX_VERSION=1.0.0 curl ... | bash   # pin a specific version

REPO="fox-in-the-box-ai/fox-fleet"
INSTALL_DIR="${FOX_INSTALL_DIR:-/usr/local/bin}"

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux" ;;
    Darwin*) echo "darwin" ;;
    *)       echo "unsupported" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)             echo "unsupported" ;;
  esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

if [ "$OS" = "unsupported" ] || [ "$ARCH" = "unsupported" ]; then
  echo "Error: unsupported platform $(uname -s)/$(uname -m)" >&2
  exit 1
fi

if [ -n "${FOX_VERSION:-}" ]; then
  VERSION="$FOX_VERSION"
  TAG="v${VERSION}"
else
  TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
  VERSION="$TAG"
  TAG="v${VERSION}"
fi

ARTIFACT="fox-control-${TAG}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARTIFACT}.tar.gz"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading fox-control ${TAG} for ${OS}/${ARCH}..."
curl -fsSL "$URL" -o "${TMPDIR}/${ARTIFACT}.tar.gz"

echo "Extracting..."
tar xzf "${TMPDIR}/${ARTIFACT}.tar.gz" -C "$TMPDIR"

if [ ! -w "$INSTALL_DIR" ]; then
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo install -m 755 "${TMPDIR}/${ARTIFACT}/fox-control" "${INSTALL_DIR}/fox-control"
else
  install -m 755 "${TMPDIR}/${ARTIFACT}/fox-control" "${INSTALL_DIR}/fox-control"
fi

echo "fox-control ${TAG} installed to ${INSTALL_DIR}/fox-control"
fox-control version
