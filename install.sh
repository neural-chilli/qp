#!/usr/bin/env sh
set -eu

REPO="neural-chilli/qp"
BIN_NAME="qp"

require() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

require curl
require tar
require uname
require mktemp

if command -v sha256sum >/dev/null 2>&1; then
  SHA_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  SHA_CMD="shasum -a 256"
else
  SHA_CMD=""
fi

detect_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)
      echo "unsupported OS: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "unsupported architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

resolve_version() {
  if [ -n "${VERSION:-}" ]; then
    echo "$VERSION"
    return
  fi
  latest="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  if [ -z "$latest" ]; then
    echo "failed to resolve latest release tag" >&2
    exit 1
  fi
  echo "$latest"
}

OS="$(detect_os)"
ARCH="$(detect_arch)"
VERSION="$(resolve_version)"
TAG="${VERSION#v}"

ARCHIVE="${BIN_NAME}_v${TAG}_${OS}_${ARCH}.tar.gz"
CHECKSUMS_URL="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"
ARCHIVE_URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TMP_DIR="$(mktemp -d)"
ARCHIVE_PATH="$TMP_DIR/$ARCHIVE"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

echo "Downloading $ARCHIVE_URL"
curl -fsSL "$ARCHIVE_URL" -o "$ARCHIVE_PATH"

if [ -n "$SHA_CMD" ]; then
  echo "Verifying checksum"
  expected="$(curl -fsSL "$CHECKSUMS_URL" | awk "/$ARCHIVE\$/ {print \$1}")"
  if [ -z "$expected" ]; then
    echo "could not find checksum for $ARCHIVE" >&2
    exit 1
  fi
  actual="$($SHA_CMD "$ARCHIVE_PATH" | awk '{print $1}')"
  if [ "$expected" != "$actual" ]; then
    echo "checksum mismatch for $ARCHIVE" >&2
    exit 1
  fi
fi

echo "Installing to $INSTALL_DIR/$BIN_NAME"
tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"
mkdir -p "$INSTALL_DIR"
install "$TMP_DIR/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"

echo "$BIN_NAME $VERSION installed to $INSTALL_DIR/$BIN_NAME"
