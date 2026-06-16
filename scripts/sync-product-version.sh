#!/usr/bin/env bash
# Sync wails.json info.productVersion from a release tag (e.g. v1.2.0 → 1.2.0).
# Prints the normalized version (without v prefix) on stdout.
set -euo pipefail

TAG="${1:?release tag required (e.g. v1.2.0)}"
VERSION="${TAG#v}"

tmp="$(mktemp)"
jq --arg v "$VERSION" '
  .info = (.info // {}) |
  .info.productVersion = $v |
  .info.productName = (.info.productName // .name // "PKG Forge") |
  .info.companyName = (.info.companyName // "Ketlark")
' wails.json > "$tmp"
mv "$tmp" wails.json

echo "$VERSION"
