#!/usr/bin/env bash
# Package a .app into a Sparkle update archive (.tar.xz).
# tar.xz extracts more reliably than zip for Sparkle updates (symlinks, frameworks).
set -euo pipefail

APP="${1:?path to .app bundle required}"
OUT="${2:?output .tar.xz path required}"

APP_DIR="$(cd "$(dirname "$APP")" && pwd)"
APP_NAME="$(basename "$APP")"

rm -f "$OUT"
COPYFILE_DISABLE=1 tar -cJf "$OUT" -C "$APP_DIR" "$APP_NAME"
echo "package-macos-sparkle-archive: wrote $OUT"
