#!/usr/bin/env bash
# Embeds Sparkle.framework into the macOS .app bundle after wails build.
# Skips when the binary was not built with -tags sparkle.
set -euo pipefail

BIN_PATH="${1:?binary path required}"

if [[ "$BIN_PATH" == *.app ]]; then
  APP_BUNDLE="$BIN_PATH"
elif [[ "$BIN_PATH" == *".app/Contents/MacOS/"* ]]; then
  APP_BUNDLE="${BIN_PATH%%/Contents/MacOS/*}"
else
  APP_BUNDLE="$(find "$(dirname "$BIN_PATH")" -maxdepth 1 -name '*.app' -print -quit)"
fi

if [[ -z "$APP_BUNDLE" || ! -d "$APP_BUNDLE" ]]; then
  echo "embed-sparkle: no .app bundle found near $BIN_PATH" >&2
  exit 1
fi

BIN="$(find "$APP_BUNDLE/Contents/MacOS" -maxdepth 1 -type f -print -quit)"
if [[ -z "$BIN" || ! -f "$BIN" ]]; then
  echo "embed-sparkle: no executable in $APP_BUNDLE/Contents/MacOS" >&2
  exit 1
fi
if ! otool -L "$BIN" 2>/dev/null | grep -q 'Sparkle.framework'; then
  echo "embed-sparkle: binary not linked against Sparkle (build without -tags sparkle?), skipping"
  exit 0
fi

resolve_sparkle_framework() {
  if [[ -n "${SPARKLE_FRAMEWORK_PATH:-}" && -d "${SPARKLE_FRAMEWORK_PATH}/Versions/B/Sparkle" ]]; then
    echo "$SPARKLE_FRAMEWORK_PATH"
    return
  fi

  local version="${SPARKLE_VERSION:-2.6.4}"
  local cache="${XDG_CACHE_HOME:-$HOME/.cache}/pkg-forge/sparkle-${version}"
  local fw="$cache/Sparkle.framework"

  if [[ ! -d "$fw/Versions/B/Sparkle" ]]; then
    mkdir -p "$cache"
    local tar="$cache/sparkle.tar.xz"
    echo "embed-sparkle: downloading Sparkle ${version} framework..." >&2
    curl -fsSL -o "$tar" \
      "https://github.com/sparkle-project/Sparkle/releases/download/${version}/Sparkle-${version}.tar.xz"
    tar xf "$tar" -C "$cache" ./Sparkle.framework
    rm -f "$tar"
  fi

  echo "$fw"
}

SRC_FW="$(resolve_sparkle_framework)"
DEST_DIR="$APP_BUNDLE/Contents/Frameworks"

mkdir -p "$DEST_DIR"
ditto "$SRC_FW" "$DEST_DIR/Sparkle.framework"

if [[ -n "${APPLE_SIGNING_IDENTITY:-}" ]]; then
  bash "$(dirname "$0")/sign-macos-app.sh" "$APP_BUNDLE"
else
  codesign -f -s - "$DEST_DIR/Sparkle.framework" 2>/dev/null || true
fi

echo "embed-sparkle: installed Sparkle.framework into $APP_BUNDLE"
