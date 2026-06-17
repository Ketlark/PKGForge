#!/usr/bin/env bash
# Verify a .app bundle contains Sparkle.framework and links correctly.
set -euo pipefail

APP="${1:?path to .app bundle required}"
MIN_VERSION="${2:-1.3.1}"

BIN="$(find "$APP/Contents/MacOS" -maxdepth 1 -type f -print -quit)"
FW="$APP/Contents/Frameworks/Sparkle.framework/Versions/B/Sparkle"
if [[ ! -f "$FW" ]]; then
  echo "verify-macos-app: missing $FW" >&2
  exit 1
fi

if ! otool -L "$BIN" | grep -q 'Sparkle.framework'; then
  echo "verify-macos-app: binary not linked against Sparkle" >&2
  exit 1
fi

VERSION="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$APP/Contents/Info.plist")"
if ! printf '%s\n%s\n' "$MIN_VERSION" "$VERSION" | sort -V | tail -1 | grep -qx "$VERSION"; then
  echo "verify-macos-app: bundle version $VERSION is below minimum $MIN_VERSION" >&2
  exit 1
fi

echo "verify-macos-app: OK ($VERSION, Sparkle.framework present)"
