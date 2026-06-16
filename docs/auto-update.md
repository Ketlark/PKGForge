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

### macOS zip format

The macOS zip is created with:

```bash
ditto -c -k --sequesterRsrc --keepParent "PKG Forge.app" pkg-forge-macos-universal.zip
```

`--sequesterRsrc` preserves framework symlinks and code signatures as required by [Sparkle](https://sparkle-project.org/documentation/publishing/).

---

## Version alignment (critical for Sparkle)

These must all match for a given release (e.g. tag `v1.2.0` → version `1.2.0`):

| Source | Set by |
|--------|--------|
| `wails.json` → `info.productVersion` | `scripts/sync-product-version.sh` in CI |
| `CFBundleShortVersionString` / `CFBundleVersion` | Wails from `info.productVersion` |
| `main.Version` (About page, Win/Linux updater) | `-ldflags "-X main.Version=…"` in CI |
| `sparkle:version` in `appcast.xml` | `generate_appcast` (reads version from `.app` inside zip) |

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
| `scripts/embed-sparkle.sh` | Copy `Sparkle.framework` into `.app` (post-build hook) |
| `scripts/sign-macos-app.sh` | Codesign app + Sparkle nested binaries |
| `scripts/notarize-macos-app.sh` | Notarize and staple release zip |
| `scripts/generate-appcast.sh` | Build signed `appcast.xml` via Sparkle `generate_appcast` |

---

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| Sparkle never offers updates | Missing/wrong `SPARKLE_*` secrets; unsigned appcast; `SUPublicEDKey` empty in installed app |
| Feed URL 404 | `appcast.xml` not attached to the latest release, or wrong URL (`releases/latest/download/`, not `releases/download/latest/`) |
| Update offered but install fails | App not signed/notarized; quarantined download; Gatekeeper block |
| Win/Linux says up to date but GitHub has newer tag | `main.Version` is `dev`; or semver pre-release tag not parsed |
| macOS dev build behaves differently from release | Dev builds lack `-tags sparkle`; use built-in updater instead |

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
