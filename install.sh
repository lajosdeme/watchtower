#!/bin/sh
# install.sh — universal installer for watchtower
# Usage: curl -fsSL https://raw.githubusercontent.com/lajosdeme/watchtower/main/install.sh | sh
# Or with a version: VERSION=v1.2.0 ... | sh

set -e

REPO="lajosdeme/watchtower"
BINARY="watchtower"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# --- helpers -----------------------------------------------------------------

say() { printf "\033[1m%s\033[0m\n" "$*"; }
err() { printf "\033[31merror:\033[0m %s\n" "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || err "required tool not found: $1"; }

need curl
need tar

# --- detect OS and arch ------------------------------------------------------

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
  *) err "unsupported OS: $OS" ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) err "unsupported architecture: $ARCH" ;;
esac

# --- resolve version ---------------------------------------------------------

if [ -z "$VERSION" ]; then
  say "Fetching latest release version..."
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
fi

[ -z "$VERSION" ] && err "could not determine version"
say "Installing ${BINARY} ${VERSION}..."

# --- build download URL ------------------------------------------------------

EXT="tar.gz"
[ "$OS" = "windows" ] && EXT="zip"

# Matches GoReleaser's default archive name template
FILENAME="${BINARY}_${VERSION#v}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

# --- download ----------------------------------------------------------------

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

say "Downloading ${URL}..."
curl -fsSL "$URL" -o "${TMP}/${FILENAME}"

# --- verify checksum ---------------------------------------------------------

say "Verifying checksum..."
curl -fsSL "$CHECKSUMS_URL" -o "${TMP}/checksums.txt"

# use sha256sum on linux, shasum on macos
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$TMP" && grep "$FILENAME" checksums.txt | sha256sum --check --status) \
    || err "checksum verification failed!"
elif command -v shasum >/dev/null 2>&1; then
  (cd "$TMP" && grep "$FILENAME" checksums.txt | shasum -a 256 --check --status) \
    || err "checksum verification failed!"
else
  say "Warning: no sha256 tool found, skipping checksum verification"
fi

# --- extract and install -----------------------------------------------------

say "Extracting..."
if [ "$EXT" = "tar.gz" ]; then
  tar -xzf "${TMP}/${FILENAME}" -C "$TMP"
else
  need unzip
  unzip -q "${TMP}/${FILENAME}" -d "$TMP"
fi

BINARY_PATH="${TMP}/${BINARY}"
[ "$OS" = "windows" ] && BINARY_PATH="${BINARY_PATH}.exe"

chmod +x "$BINARY_PATH"

# --- install (with sudo fallback) --------------------------------------------

if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY_PATH" "${INSTALL_DIR}/${BINARY}"
else
  say "Need sudo to write to ${INSTALL_DIR}..."
  sudo mv "$BINARY_PATH" "${INSTALL_DIR}/${BINARY}"
fi

say "✓ ${BINARY} ${VERSION} installed to ${INSTALL_DIR}/${BINARY}"
"${INSTALL_DIR}/${BINARY}" --version 2>/dev/null || true