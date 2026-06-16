#!/usr/bin/env bash
# Optionally notarize a zipped .app (requires Apple notarization secrets).
set -euo pipefail

ZIP="${1:?path to .zip required}"

if [[ -z "${APPLE_SIGNING_IDENTITY:-}" || -z "${APPLE_ID:-}" || -z "${APPLE_PASSWORD:-}" || -z "${APPLE_TEAM_ID:-}" ]]; then
  echo "notarize-macos-app: notarization secrets not configured, skipping"
  exit 0
fi

if ! xcrun notarytool submit "$ZIP" \
  --apple-id "$APPLE_ID" \
  --password "$APPLE_PASSWORD" \
  --team-id "$APPLE_TEAM_ID" \
  --wait; then
  echo "notarize-macos-app: notarization failed" >&2
  exit 1
fi

TMP="$(mktemp -d)"
ditto -x -k "$ZIP" "$TMP"
APP="$(find "$TMP" -maxdepth 2 -name '*.app' -print -quit)"
if [[ -n "$APP" ]]; then
  xcrun stapler staple "$APP"
  rm -f "$ZIP"
  ditto -c -k --sequesterRsrc --keepParent "$APP" "$ZIP"
fi
rm -rf "$TMP"

echo "notarize-macos-app: stapled $ZIP"
