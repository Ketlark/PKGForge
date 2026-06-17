#!/usr/bin/env bash
# Ad-hoc codesign after Info.plist or framework changes (unsigned CI / local builds).
set -euo pipefail

APP="${1:?path to .app bundle required}"

# Finder/Chrome quarantine and resource forks break nested Sparkle signatures on macOS 26+.
dot_clean -m "$APP" 2>/dev/null || true
xattr -cr "$APP" 2>/dev/null || true

sign_adhoc() {
  # Do not use -o runtime for ad-hoc builds: Tahoe rejects loading Sparkle when
  # the main binary and embedded framework were signed with mismatched flags.
  codesign -f -s - "$@"
}

FW="$APP/Contents/Frameworks/Sparkle.framework"
if [[ -d "$FW" ]]; then
  VB="$FW/Versions/B"
  if [[ ! -d "$VB" ]]; then
    VB="$FW/Versions/A"
  fi
  for target in \
    "$VB/XPCServices/Installer.xpc" \
    "$VB/XPCServices/Downloader.xpc" \
    "$VB/Autoupdate" \
    "$VB/Updater.app/Contents/MacOS/Updater" \
    "$VB/Updater.app"; do
    if [[ -e "$target" ]]; then
      sign_adhoc "$target"
    fi
  done
  if [[ -f "$VB/Sparkle" ]]; then
    sign_adhoc "$VB/Sparkle"
  fi
  sign_adhoc "$FW"
fi

sign_adhoc "$APP"
codesign --verify --deep --strict "$APP"
echo "resign-macos-adhoc: ad-hoc signed $APP"
