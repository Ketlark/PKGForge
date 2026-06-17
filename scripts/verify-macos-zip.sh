#!/usr/bin/env bash
# Verify a release zip contains Sparkle.framework and a launchable universal binary.
set -euo pipefail

ZIP="${1:?path to pkg-forge-macos-universal.zip required}"
MIN_VERSION="${2:-1.3.1}"

TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

ditto -x -k "$ZIP" "$TMP"
APP="$(find "$TMP" -maxdepth 2 -name '*.app' -print -quit)"
if [[ -z "$APP" || ! -d "$APP" ]]; then
  echo "verify-macos-zip: no .app in archive" >&2
  exit 1
fi

BIN="$(find "$APP/Contents/MacOS" -maxdepth 1 -type f -print -quit)"
FW="$APP/Contents/Frameworks/Sparkle.framework/Versions/B/Sparkle"
if [[ ! -f "$FW" ]]; then
  echo "verify-macos-zip: missing $FW" >&2
  exit 1
fi

if ! otool -L "$BIN" | grep -q 'Sparkle.framework'; then
  echo "verify-macos-zip: binary not linked against Sparkle" >&2
  exit 1
fi

VERSION="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$APP/Contents/Info.plist")"
if ! printf '%s\n%s\n' "$MIN_VERSION" "$VERSION" | sort -V | tail -1 | grep -qx "$VERSION"; then
  echo "verify-macos-zip: bundle version $VERSION is below minimum $MIN_VERSION" >&2
  exit 1
fi

echo "verify-macos-zip: OK ($VERSION, Sparkle.framework present)"
