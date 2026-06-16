#!/usr/bin/env bash
# Ad-hoc codesign after Info.plist or framework changes (unsigned CI / local builds).
set -euo pipefail

APP="${1:?path to .app bundle required}"

sign_adhoc() {
  codesign -f -s - -o runtime "$@"
}

FW="$APP/Contents/Frameworks/Sparkle.framework"
if [[ -d "$FW" ]]; then
  VB="$FW/Versions/B"
  for target in \
    "$VB/XPCServices/Installer.xpc" \
    "$VB/XPCServices/Downloader.xpc" \
    "$VB/Autoupdate" \
    "$VB/Updater.app"; do
    if [[ -e "$target" ]]; then
      sign_adhoc "$target"
    fi
  done
  sign_adhoc "$FW"
fi

sign_adhoc "$APP"
echo "resign-macos-adhoc: ad-hoc signed $APP"
