# Auto-update and release signing

PKG Forge uses **two update mechanisms** depending on the platform:

| Platform | Mechanism | User experience |
|----------|-----------|-----------------|
| **macOS** (release build) | [Sparkle](https://sparkle-project.org/) via [go-sparkle](https://github.com/abemedia/go-sparkle) | Native update dialog; full `.app` bundle replaced |
| **Windows / Linux** | Built-in Go updater | Check, download, progress bar, restart from About page |
| **macOS** (local dev) | Built-in Go updater | Same as Windows/Linux (`wails dev` without `-tags sparkle`) |

See [ADR 0018](adr/0018-cross-platform-auto-update.md) for the rationale.

---

## User-facing behaviour

- **Settings → Updates**: toggle *Check for updates on startup*
- **About → Software updates**: manual check; on macOS (Sparkle) this opens the native Sparkle UI
- **Windows / Linux**: if an update is available, download + install in-app, then restart
- **Dev builds** (`Version=dev`): no update checks

---

## Release assets

Each GitHub Release (`v*`) publishes:

| Asset | Purpose |
|-------|---------|
| `pkg-forge-windows-amd64.exe` | Windows binary |
| `pkg-forge-macos-universal.zip` | macOS `.app` (Sparkle update archive) |
| `pkg-forge-linux-amd64` | Linux x86_64 |
| `pkg-forge-linux-arm64` | Linux ARM64 |
| `SHA256SUMS.txt` | Integrity checks for Win/Linux built-in updater |
| `appcast.xml` | Sparkle feed (macOS); resolved via `/releases/latest/download/appcast.xml` |

### macOS release archives

| File | Purpose |
|------|---------|
| `pkg-forge-macos-universal.zip` | Manual download from GitHub Releases |
| `pkg-forge-macos-universal.tar.xz` | Sparkle auto-update feed (`appcast.xml` enclosure) |

The zip is created with:

```bash
ditto -c -k --rsrc --sequesterRsrc --keepParent "PKG Forge.app" pkg-forge-macos-universal.zip
```

Sparkle updates use **tar.xz** instead of zip — zip extraction on macOS can drop embedded frameworks (`Sparkle.framework`) intermittently after in-app updates ([Sparkle #311](https://github.com/sparkle-project/Sparkle/issues/311)). CI verifies both archives before publish.

```bash
COPYFILE_DISABLE=1 tar -cJf pkg-forge-macos-universal.tar.xz -C build/bin "PKG Forge.app"
```

---

## Version alignment (critical for Sparkle)

These must all match for a given release (e.g. tag `v1.2.0` → version `1.2.0`):

| Source | Set by |
|--------|--------|
| `wails.json` → `info.productVersion` | `scripts/sync-product-version.sh` in CI |
| `CFBundleShortVersionString` / `CFBundleVersion` | Wails from `info.productVersion` |
| `main.Version` (About page, Win/Linux updater) | `-ldflags "-X main.Version=…"` in CI |
| `sparkle:version` in `appcast.xml` | `generate-appcast.sh` (signs the `.tar.xz` update archive) |

CI runs `scripts/sync-product-version.sh` before every platform build.

---

## Sparkle signing keys (one-time setup)

Keys are **not** on GitHub. Generate them once on a Mac:

### 1. Download Sparkle tools

```bash
curl -fsSL -o /tmp/sparkle.tar.xz \
  "https://github.com/sparkle-project/Sparkle/releases/download/2.6.4/Sparkle-2.6.4.tar.xz"
tar xf /tmp/sparkle.tar.xz -C /tmp
```

### 2. Generate or display the public key

```bash
/tmp/bin/generate_keys
```

macOS may prompt for Keychain access. Output includes:

```xml
<key>SUPublicEDKey</key>
<string>YOUR_PUBLIC_KEY_BASE64=</string>
```

If a key already exists:

```bash
/tmp/bin/generate_keys -p
```

### 3. Export the private key for CI

```bash
/tmp/bin/generate_keys -x ~/sparkle-eddsa-private.key
```

**Never commit this file.** Store its contents as a GitHub secret.

### 4. Add GitHub repository secrets

Repository → **Settings** → **Secrets and variables** → **Actions** → **New repository secret**:

| Secret | Value |
|--------|--------|
| `SPARKLE_PUBLIC_ED_KEY` | Base64 string from step 2 (without XML tags) |
| `SPARKLE_EDDSA_PRIVATE_KEY` | Full contents of `~/sparkle-eddsa-private.key` |

The release workflow injects the public key into the built `.app` (`SUPublicEDKey`) and signs `appcast.xml` with the private key.

Without these secrets, releases still publish an **unsigned** `appcast.xml` (logged as a warning). Sparkle **rejects** unsigned updates in production.

---

## Apple codesigning and notarization (optional, recommended for macOS)

For distribution outside your own machine, configure these **additional** secrets:

| Secret | Description |
|--------|-------------|
| `APPLE_CERTIFICATE` | Base64-encoded `.p12` (Developer ID Application) |
| `APPLE_CERTIFICATE_PASSWORD` | Password for the `.p12` |
| `APPLE_SIGNING_IDENTITY` | e.g. `Developer ID Application: Your Name (TEAMID)` |
| `KEYCHAIN_PASSWORD` | Any strong random string (temporary CI keychain) |
| `APPLE_ID` | Apple ID email |
| `APPLE_PASSWORD` | App-specific password |
| `APPLE_TEAM_ID` | 10-character Team ID |

When present, CI:

1. Signs the `.app` and embedded Sparkle framework (`scripts/sign-macos-app.sh`)
2. Notarizes the zip (`scripts/notarize-macos-app.sh`)

If secrets are missing, these steps are skipped automatically.

---

## Maintainer commands

### Cut a release

```bash
git tag -a v1.2.0 -m "Release v1.2.0"
git push origin v1.2.0
```

The `release.yml` workflow builds all platforms and publishes the GitHub Release.

### Local macOS build with Sparkle (matches CI)

```bash
scripts/sync-product-version.sh v1.2.0   # optional locally
CGO_ENABLED=1 CGO_LDFLAGS='-Wl,-rpath,@loader_path/../Frameworks' \
  wails build -platform darwin/universal -tags sparkle -clean
```

The `postBuildHooks` entry in `wails.json` runs `scripts/embed-sparkle.sh` after the build.

### Local macOS / Windows / Linux dev build (no Sparkle)

```bash
wails dev
# or
wails build
```

---

## Scripts reference

| Script | Role |
|--------|------|
| `scripts/sync-product-version.sh` | Sync `wails.json` `info.productVersion` from tag |
| `scripts/embed-sparkle.sh` | Copy official Sparkle `Sparkle.framework` into `.app` (post-build hook) |
| `scripts/sign-macos-app.sh` | Codesign app + Sparkle nested binaries |
| `scripts/notarize-macos-app.sh` | Notarize and staple release zip |
| `scripts/generate-appcast.sh` | Build signed `appcast.xml` via Sparkle `sign_update` (+ self-verify) |
| `scripts/derive_sparkle_public_key.go` | Derive `SUPublicEDKey` from exported private key (CI key-pair check) |
| `scripts/resign-macos-adhoc.sh` | Ad-hoc re-sign `.app` after `Info.plist` / Sparkle edits |
| `scripts/verify-macos-app.sh` | Shared bundle checks (Sparkle.framework present) |
| `scripts/verify-macos-zip.sh` | CI gate for manual-download zip |
| `scripts/package-macos-sparkle-archive.sh` | Build Sparkle `.tar.xz` update archive |
| `scripts/verify-macos-sparkle-archive.sh` | CI gate: tar.xz extracts to app with `Sparkle.framework` |

---

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| Sparkle never offers updates | Missing/wrong `SPARKLE_*` secrets; unsigned appcast; `SUPublicEDKey` empty in installed app |
| **“The update is improperly signed…”** | Installed app has empty/wrong `SUPublicEDKey` (common with local `wails build`); or `SPARKLE_PUBLIC_ED_KEY` / `SPARKLE_EDDSA_PRIVATE_KEY` mismatch in CI; or testing with a dev-signed build against ad-hoc CI releases |
| Feed URL 404 | `appcast.xml` not attached to the latest release, or wrong URL (`releases/latest/download/`, not `releases/download/latest/`) |
| macOS app crashes at launch (`Sparkle.framework` missing) | **v1.3.0 macOS zip is broken** (no embedded framework). Delete the app and install **v1.3.1+** manually from [Releases](https://github.com/Ketlark/PKGForge/releases). Do not use the v1.3.0 macOS asset. Later releases embed Sparkle correctly. |
| Sparkle update ends on v1.3.0 / same crash after update | The installed bundle is the broken v1.3.0 build (check `CFBundleShortVersionString` in crash log). Remove `PKG Forge.app` and install **v1.3.3+** manually from GitHub before retrying auto-update. |
| Sparkle update from v1.3.1+ still crashes (`Sparkle.framework` missing) | Known zip extraction issue during in-app updates; fixed from **v1.3.4** by shipping `.tar.xz` in the appcast. Install v1.3.4+ manually once, then auto-update should work. |
| Update offered but install fails | App not signed/notarized; quarantined download; Gatekeeper block |
| Win/Linux says up to date but GitHub has newer tag | `main.Version` is `dev`; or semver pre-release tag not parsed |
| macOS dev build behaves differently from release | Dev builds lack `-tags sparkle`; use built-in updater instead |

**Diagnose Sparkle signature errors on macOS:**

```bash
# Installed app must expose the same public key as CI (non-empty)
/usr/libexec/PlistBuddy -c 'Print :SUPublicEDKey' '/Applications/PKG Forge.app/Contents/Info.plist'

# Sparkle logs (reproduce the failed update first)
log show --last 5m --predicate 'process CONTAINS[c] "Sparkle" OR process CONTAINS[c] "pkg-forge"' | tail -80
```

For update testing, start from a **GitHub release** build (e.g. v1.3.1 zip), not an unsigned local `wails build`, unless you also build with `-tags sparkle` after these plist fixes.

---

## Code map

```
core/
  update_common.go      # Shared types, semver, constants
  update_builtin.go     # GitHub updater (!darwin || !sparkle)
  update_sparkle.go     # Sparkle backend (darwin && sparkle)
frontend/src/lib/stores/update.ts
frontend/src/lib/components/AboutPage.svelte
frontend/src/lib/components/SettingsPage.svelte
.github/workflows/release.yml
```
