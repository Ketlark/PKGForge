#!/usr/bin/env bash
# Generate a signed Sparkle appcast.xml for the macOS update zip in dist/.
#
# Requires Sparkle sign_update on PATH (macOS Sparkle bin/).
# Env:
#   SPARKLE_EDDSA_PRIVATE_KEY  EdDSA private key PEM (GitHub secret)
#   RELEASE_VERSION            Tag, e.g. v1.2.0
set -euo pipefail

DIST_DIR="${1:-dist}"
ZIP_NAME="pkg-forge-macos-universal.zip"
ZIP_PATH="$DIST_DIR/$ZIP_NAME"
APPCAST="$DIST_DIR/appcast.xml"
TAG="${RELEASE_VERSION:?RELEASE_VERSION is required}"
VERSION="${TAG#v}"
KEY_FILE="$(mktemp)"

cleanup() {
  rm -f "$KEY_FILE"
}
trap cleanup EXIT

if [[ ! -f "$ZIP_PATH" ]]; then
  echo "generate-appcast: missing $ZIP_PATH" >&2
  exit 1
fi

DOWNLOAD_PREFIX="https://github.com/Ketlark/PKGForge/releases/download/${TAG}/"

if [[ -z "${SPARKLE_EDDSA_PRIVATE_KEY:-}" ]]; then
  echo "generate-appcast: SPARKLE_EDDSA_PRIVATE_KEY not set; writing unsigned feed (Sparkle will reject in production)" >&2
  cat > "$APPCAST" <<EOF
<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">
  <channel>
    <title>PKG Forge</title>
    <link>https://github.com/Ketlark/PKGForge</link>
    <description>PKG Forge updates</description>
    <language>en</language>
    <item>
      <title>Version ${VERSION}</title>
      <link>https://github.com/Ketlark/PKGForge/releases/tag/${TAG}</link>
      <sparkle:version>${VERSION}</sparkle:version>
      <sparkle:shortVersionString>${VERSION}</sparkle:shortVersionString>
      <pubDate>$(date -R)</pubDate>
      <enclosure
        url="${DOWNLOAD_PREFIX}${ZIP_NAME}"
        length="$(wc -c < "$ZIP_PATH" | tr -d ' ')"
        type="application/octet-stream"/>
    </item>
  </channel>
</rss>
EOF
  echo "generate-appcast: wrote unsigned appcast to $APPCAST"
  exit 0
fi

if ! command -v sign_update >/dev/null 2>&1; then
  echo "generate-appcast: sign_update not found on PATH" >&2
  exit 1
fi

printf '%s' "$SPARKLE_EDDSA_PRIVATE_KEY" > "$KEY_FILE"
SIGNATURE="$(sign_update --ed-key-file "$KEY_FILE" -p "$ZIP_PATH" | tr -d '\n\r')"
LENGTH="$(wc -c < "$ZIP_PATH" | tr -d ' ')"

if ! sign_update --ed-key-file "$KEY_FILE" --verify "$ZIP_PATH" "$SIGNATURE"; then
  echo "generate-appcast: signature self-check failed" >&2
  exit 1
fi

cat > "$APPCAST" <<EOF
<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">
  <channel>
    <title>PKG Forge</title>
    <link>https://github.com/Ketlark/PKGForge</link>
    <description>PKG Forge updates</description>
    <language>en</language>
    <item>
      <title>Version ${VERSION}</title>
      <link>https://github.com/Ketlark/PKGForge/releases/tag/${TAG}</link>
      <sparkle:version>${VERSION}</sparkle:version>
      <sparkle:shortVersionString>${VERSION}</sparkle:shortVersionString>
      <pubDate>$(date -R)</pubDate>
      <enclosure
        url="${DOWNLOAD_PREFIX}${ZIP_NAME}"
        sparkle:edSignature="${SIGNATURE}"
        length="${LENGTH}"
        type="application/octet-stream"/>
    </item>
  </channel>
</rss>
EOF

echo "generate-appcast: wrote signed appcast to $APPCAST (length=${LENGTH})"
