#!/usr/bin/env bash
# Optionally sign PKG Forge.app and embedded Sparkle.framework for distribution.
# Skipped when APPLE_SIGNING_IDENTITY is unset (local/unsigned builds).
set -euo pipefail

APP="${1:?path to .app bundle required}"
IDENTITY="${APPLE_SIGNING_IDENTITY:-}"

if [[ -z "$IDENTITY" ]]; then
  echo "sign-macos-app: APPLE_SIGNING_IDENTITY not set, skipping codesign"
  exit 0
fi

sign() {
  codesign -f -s "$IDENTITY" -o runtime --timestamp "$@"
}

FW="$APP/Contents/Frameworks/Sparkle.framework"
if [[ -d "$FW" ]]; then
  VA="$FW/Versions/A"
  for target in \
    "$VA/XPCServices/Installer.xpc" \
    "$VA/XPCServices/Downloader.xpc" \
    "$VA/Autoupdate" \
    "$VA/Updater.app"; do
    if [[ -e "$target" ]]; then
      sign "$target"
    fi
  done
  sign "$FW"
fi

sign "$APP"
codesign -dv --verbose=4 "$APP" 2>&1 | head -20
echo "sign-macos-app: signed $APP"
