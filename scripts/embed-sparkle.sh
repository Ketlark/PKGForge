#!/usr/bin/env bash
# Embeds Sparkle.framework into the macOS .app bundle after wails build.
# Skips when the binary was not built with -tags sparkle.
set -euo pipefail

BIN_PATH="${1:?binary path required}"

if [[ "$BIN_PATH" == *.app ]]; then
  APP_BUNDLE="$BIN_PATH"
else
  APP_BUNDLE="$(find "$(dirname "$BIN_PATH")" -maxdepth 1 -name '*.app' -print -quit)"
fi

if [[ -z "$APP_BUNDLE" || ! -d "$APP_BUNDLE" ]]; then
  echo "embed-sparkle: no .app bundle found near $BIN_PATH" >&2
  exit 1
fi

BIN="$APP_BUNDLE/Contents/MacOS/"*
if ! otool -L "$BIN" 2>/dev/null | grep -q 'Sparkle.framework'; then
  echo "embed-sparkle: binary not linked against Sparkle (build without -tags sparkle?), skipping"
  exit 0
fi

MOD_DIR="$(go list -f '{{.Dir}}' -m github.com/abemedia/go-sparkle)"
SRC_FW="$MOD_DIR/Sparkle.framework"
DEST_DIR="$APP_BUNDLE/Contents/Frameworks"

if [[ ! -d "$SRC_FW" ]]; then
  echo "embed-sparkle: Sparkle.framework not found in $MOD_DIR" >&2
  exit 1
fi

mkdir -p "$DEST_DIR"
ditto "$SRC_FW" "$DEST_DIR/Sparkle.framework"

if [[ -n "${APPLE_SIGNING_IDENTITY:-}" ]]; then
  bash "$(dirname "$0")/sign-macos-app.sh" "$APP_BUNDLE"
else
  codesign -f -s - "$DEST_DIR/Sparkle.framework" 2>/dev/null || true
fi

echo "embed-sparkle: installed Sparkle.framework into $APP_BUNDLE"
