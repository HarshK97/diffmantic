#!/bin/sh
set -e

# Diffmantic Installer Script
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/HarshK97/diffmantic/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/HarshK97/diffmantic/main/install.sh | sh -s -- --dir=$HOME/.local/bin

OWNER="HarshK97"
REPO="diffmantic"
BINARY_NAME="diffm"

# Parse arguments
INSTALL_DIR="$HOME/.local/bin"
VERSION=""

while [ $# -gt 0 ]; do
  case "$1" in
    --dir=*)
      INSTALL_DIR="${1#*=}"
      ;;
    --version=*)
      VERSION="${1#*=}"
      ;;
    *)
      echo "Error: Unrecognized option '$1'"
      echo "Usage: $0 [--dir=<path>] [--version=<version>]"
      exit 1
      ;;
  esac
  shift
done

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin)  TARGET_OS="darwin" ;;
  linux)   TARGET_OS="linux" ;;
  msys*|mingw*|cygwin*) TARGET_OS="windows" ;;
  *)
    echo "Error: Unsupported operating system: $OS"
    exit 1
    ;;
esac

# Detect Architecture
ARCH=$(uname -m | tr '[:upper:]' '[:lower:]')
case "$ARCH" in
  x86_64|amd64) TARGET_ARCH="amd64" ;;
  arm64|aarch64) TARGET_ARCH="arm64" ;;
  *)
    echo "Error: Unsupported CPU architecture: $ARCH"
    exit 1
    ;;
esac

# Resolve Version
if [ -z "$VERSION" ] || [ "$VERSION" = "latest" ]; then
  echo "Checking the latest release version..."
  RELEASE_URL="https://api.github.com/repos/$OWNER/$REPO/releases/latest"
  # Fetch latest tag name from GitHub API
  VERSION=$(curl -sfL "$RELEASE_URL" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
  if [ -z "$VERSION" ]; then
    echo "Error: Failed to fetch the latest release version from GitHub."
    exit 1
  fi
fi

# Clean 'v' prefix if present for binary matching, but keep it if the asset URL uses it
VERSION_CLEAN=$(echo "$VERSION" | sed 's/^v//')

# Set Extension and Real Binary Name
FORMAT="tar.gz"
REAL_BINARY_NAME="$BINARY_NAME"
if [ "$TARGET_OS" = "windows" ]; then
  FORMAT="zip"
  REAL_BINARY_NAME="${BINARY_NAME}.exe"
fi

# Build Asset Name (Must match .goreleaser.yaml name_template)
# Template: diffmantic_{Version}_{Os}_{Arch}.tar.gz
ASSET_NAME="diffmantic_${VERSION_CLEAN}_${TARGET_OS}_${TARGET_ARCH}.${FORMAT}"
DOWNLOAD_URL="https://github.com/$OWNER/$REPO/releases/download/$VERSION/$ASSET_NAME"
CHECKSUMS_URL="https://github.com/$OWNER/$REPO/releases/download/$VERSION/checksums.txt"

echo "Resolved target: OS=$TARGET_OS, ARCH=$TARGET_ARCH, VERSION=$VERSION"
echo "Downloading $DOWNLOAD_URL ..."

# Create install directory
mkdir -p "$INSTALL_DIR"

# Download Asset
TEMP_DIR=$(mktemp -d)
TEMP_FILE="$TEMP_DIR/$ASSET_NAME"

if ! curl -sfL -o "$TEMP_FILE" "$DOWNLOAD_URL"; then
  echo "Error: Failed to download release asset: $ASSET_NAME"
  rm -rf "$TEMP_DIR"
  exit 1
fi

# Checksum Verification
echo "Verifying checksum..."
CHECKSUM_FILE="$TEMP_DIR/checksums.txt"
if curl -sfL -o "$CHECKSUM_FILE" "$CHECKSUMS_URL"; then
  EXPECTED_HASH=$(grep "$ASSET_NAME" "$CHECKSUM_FILE" | sed 's/[[:space:]].*//')
  if [ -n "$EXPECTED_HASH" ]; then
    ACTUAL_HASH=""
    if command -v sha256sum >/dev/null 2>&1; then
      ACTUAL_HASH=$(sha256sum "$TEMP_FILE" | sed 's/[[:space:]].*//')
    elif command -v shasum >/dev/null 2>&1; then
      ACTUAL_HASH=$(shasum -a 256 "$TEMP_FILE" | sed 's/[[:space:]].*//')
    else
      echo "Warning: No sha256sum or shasum tool found. Skipping checksum verification."
    fi

    if [ -n "$ACTUAL_HASH" ]; then
      if [ "$ACTUAL_HASH" != "$EXPECTED_HASH" ]; then
        echo "Error: Checksum verification failed for $ASSET_NAME"
        echo "Expected: $EXPECTED_HASH"
        echo "Actual:   $ACTUAL_HASH"
        rm -rf "$TEMP_DIR"
        exit 1
      fi
      echo "Checksum verified successfully."
    fi
  else
    echo "Warning: Checksum for $ASSET_NAME not found in checksums.txt. Skipping verification."
  fi
else
  echo "Warning: Failed to download checksums.txt. Skipping checksum verification."
fi

echo "Extracting binary..."
if [ "$FORMAT" = "zip" ]; then
  unzip -q -o "$TEMP_FILE" -d "$TEMP_DIR"
else
  tar -xzf "$TEMP_FILE" -C "$TEMP_DIR"
fi

# Move binary to target location
mv "$TEMP_DIR/$REAL_BINARY_NAME" "$INSTALL_DIR/$REAL_BINARY_NAME"
chmod +x "$INSTALL_DIR/$REAL_BINARY_NAME"

# Cleanup temp files
rm -rf "$TEMP_DIR"

echo "Successfully installed $REAL_BINARY_NAME to $INSTALL_DIR/$REAL_BINARY_NAME"

# Check if INSTALL_DIR is on PATH
case ":$PATH:" in
  *:"$INSTALL_DIR":*) ;;
  *)
    echo ""
    echo "Warning: $INSTALL_DIR is not on your PATH."
    echo "You can add it by appending this line to your shell profile (e.g., ~/.bashrc or ~/.zshrc):"
    echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
    ;;
esac
