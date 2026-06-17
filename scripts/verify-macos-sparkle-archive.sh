#!/usr/bin/env bash
# Verify a Sparkle .tar.xz update archive extracts to a valid app bundle.
set -euo pipefail

ARCHIVE="${1:?path to pkg-forge-macos-universal.tar.xz required}"
MIN_VERSION="${2:-1.3.1}"

TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

tar -xJf "$ARCHIVE" -C "$TMP"
APP="$(find "$TMP" -maxdepth 2 -name '*.app' -print -quit)"
if [[ -z "$APP" || ! -d "$APP" ]]; then
  echo "verify-macos-sparkle-archive: no .app in archive" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
bash "$SCRIPT_DIR/verify-macos-app.sh" "$APP" "$MIN_VERSION"
echo "verify-macos-sparkle-archive: OK"
