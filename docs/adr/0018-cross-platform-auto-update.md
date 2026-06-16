# ADR 0018: Cross-platform auto-update (Sparkle on macOS, built-in on Windows/Linux)

Date: 2026-06-16

## Status

Accepted

## Context

PKG Forge is distributed as native binaries via GitHub Releases (Windows `.exe`,
Linux bare binaries, macOS `.app` in a `.zip`). Users need in-app updates without
manual re-downloads.

Wails v2 has no built-in auto-update. Options considered:

| Approach | macOS | Windows / Linux |
|----------|-------|-----------------|
| Custom Go updater (GitHub API + binary swap) | Replaces only the binary inside `.app`, not bundle resources | Works well |
| [go-selfupdate](https://github.com/creativeprojects/go-selfupdate) | Same binary-only limitation on `.app` | Battle-tested |
| [Sparkle](https://sparkle-project.org/) | Industry standard; replaces full `.app` bundle | macOS only |

macOS distribution as a `.app` bundle makes Sparkle the right choice for
production macOS builds: it swaps the entire bundle (Info.plist, icons,
frameworks) and handles relaunch.

Windows and Linux releases are single executables; a Go-based updater with
`SHA256SUMS.txt` verification is sufficient.

## Decision

Use a **split backend**:

- **macOS release builds** (`-tags sparkle`): [go-sparkle](https://github.com/abemedia/go-sparkle) + Sparkle 2 embedded in the `.app`. Feed URL:
  `https://github.com/Ketlark/PKGForge/releases/latest/download/appcast.xml`
- **Windows / Linux** (and macOS dev builds without `sparkle`): built-in updater in `core/update_builtin.go` — GitHub Releases API, semver compare, SHA256 verify, in-place binary replace
- **UI**: About page and Settings toggle; macOS Sparkle builds delegate check/install to native Sparkle dialogs

Release pipeline (`.github/workflows/release.yml`):

1. Sync `wails.json` `info.productVersion` from the git tag (must match `CFBundleVersion` for Sparkle)
2. Build platform artifacts; macOS with Sparkle + optional codesign/notarization
3. Publish `appcast.xml` (signed with EdDSA when `SPARKLE_EDDSA_PRIVATE_KEY` is set) alongside binaries and `SHA256SUMS.txt`

## Consequences

**Positive**

- macOS users get full-bundle updates via Sparkle
- Win/Linux keep a unified GitHub + SHA256 workflow
- Dev builds on macOS without Sparkle still work (`wails dev` / `wails build`)

**Negative / operational**

- Sparkle requires one-time EdDSA key generation and two GitHub secrets
- Production macOS updates need Apple Developer ID signing + notarization for a smooth Gatekeeper experience (optional in CI until secrets are configured)
- `go-sparkle` wraps deprecated `SUUpdater` APIs (still functional as a Sparkle 2 stub); may need a native bridge later

## References

- [docs/auto-update.md](../auto-update.md) — setup and secrets
- [Sparkle publishing](https://sparkle-project.org/documentation/publishing/)
- [marcus-crane/wails3-sparkle-poc](https://github.com/marcus-crane/wails3-sparkle-poc)
