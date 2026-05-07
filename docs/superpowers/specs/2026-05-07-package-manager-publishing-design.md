# Package Manager Publishing via GoReleaser

**Date:** 2026-05-07
**Status:** Approved
**Scope:** Scoop (Windows) and Homebrew (macOS/Linux) distribution of the CLI binary

## Problem

Users who download `sap-devs.exe` directly from GitHub Releases encounter Windows SmartScreen warnings. Beyond that, there's no automatic update mechanism — users must manually check for new versions. Package managers solve both: trusted install paths bypass SmartScreen, and updates are handled by the package manager.

## Solution

Add `scoops:` and `homebrew_casks:` sections to `.goreleaser.yml`. On each tagged release, GoReleaser automatically generates and commits manifest files to the same repository. No companion repos needed.

## Design Decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Manifest location | Same repo (`sap-devs-cli`) | Simpler management; no extra repos to create or maintain |
| Package scope | CLI only (`sap-devs`) | Tray is experimental and managed via `sap-devs tray install` |
| Post-install hooks | None | Standard for developer CLIs; user runs `sap-devs init` manually |
| Implementation | GoReleaser native sections | Built-in support, zero custom scripting, proven reliability |
| Homebrew key | `homebrew_casks:` (not deprecated `brews:`) | `brews:` deprecated in GoReleaser v2.10; casks work for CLI tools too |
| Scoop directory | `bucket/` | Scoop convention; Scoop checks root and `bucket/` automatically |

## GoReleaser Configuration

### Scoop Section

```yaml
scoops:
  - repository:
      owner: SAP-samples
      name: sap-devs-cli
      branch: main
      token: "{{ .Env.GITHUB_TOKEN }}"
    directory: bucket
    homepage: https://github.com/SAP-samples/sap-devs-cli
    description: SAP developer context CLI — inject SAP knowledge into AI coding tools
    license: Apache-2.0
```

GoReleaser generates `bucket/sap-devs.json` containing:
- Version, download URL for the Windows amd64 zip
- SHA256 checksum (extracted from `checksums.txt`)
- `bin` entry pointing to `sap-devs.exe`

**Note:** `wrap_in_directory: false` in the archives config means the zip contains `sap-devs.exe` at root. The Scoop manifest's `bin` entry depends on this — changes to archive structure would break Scoop installs.

### Homebrew Section

```yaml
homebrew_casks:
  - repository:
      owner: SAP-samples
      name: sap-devs-cli
      branch: main
      token: "{{ .Env.GITHUB_TOKEN }}"
    directory: Casks
    homepage: https://github.com/SAP-samples/sap-devs-cli
    description: SAP developer context CLI — inject SAP knowledge into AI coding tools
    name: sap-devs
```

GoReleaser generates `Casks/sap-devs.rb` as a Homebrew cask containing:

- Version, download URLs per platform (linux amd64/arm64, darwin amd64/arm64)
- SHA256 checksums per platform
- Binary stanza pointing to `sap-devs`

**Migration from `brews:`:** GoReleaser v2.10 deprecated `brews:` (which generated "hacky" formulas installing pre-compiled binaries). `homebrew_casks:` is the modern replacement and works equally well for CLI tools. Casks install pre-compiled binaries directly — the same behavior we want.

## Install Experience

### Scoop (Windows)

```powershell
scoop bucket add sap-devs https://github.com/SAP-samples/sap-devs-cli
scoop install sap-devs
```

Updates: `scoop update sap-devs`

### Homebrew (macOS/Linux)

```bash
brew tap SAP-samples/sap-devs-cli https://github.com/SAP-samples/sap-devs-cli
brew install SAP-samples/sap-devs-cli/sap-devs
```

Updates: `brew upgrade sap-devs`

## Token & Permissions

The release workflow already has `permissions: contents: write` and uses `GITHUB_TOKEN`. GoReleaser uses this token to push manifest commits to the same repo. The token is explicitly passed via `token: "{{ .Env.GITHUB_TOKEN }}"` in both repository blocks.

**Branch protection:** The `GITHUB_TOKEN` in GitHub Actions acts as `github-actions[bot]`. If `main` has branch protection rules that block direct pushes, GoReleaser's manifest commit will fail silently (release still succeeds, but manifests won't update). **Before the first release with this config:**

- Verify that `github-actions[bot]` is allowed to push to `main`, OR
- Add a bypass rule for `github-actions[bot]` in branch protection settings, OR
- Use a PAT with `repo` scope in a separate secret (e.g., `GH_PAT`) and reference it in the token field

**Important:** GoReleaser commits directly to `main`. This is standard for package manager manifests — the commits are small (single JSON/Ruby file) and fully automated.

## Signing Interaction

The signing workflow (`sign-windows.yml`) runs *after* GoReleaser and re-uploads signed Windows archives with `--clobber`. This changes the binary content at the release asset URL. The Scoop manifest (committed by GoReleaser) contains the SHA256 of the **unsigned** zip. After signing completes, the zip at that URL has a different SHA256 → **Scoop installs will permanently fail** until the manifest is updated.

### Required Fix: Update Scoop Manifest After Signing

The signing workflow must update `bucket/sap-devs.json` after re-uploading signed artifacts. Add a step to `sign-windows.yml` that:

1. Downloads the current `bucket/sap-devs.json` from the repo
2. Replaces the `hash` field with the SHA256 of the newly-signed zip
3. Commits and pushes the updated manifest

```bash
# In sign-windows.yml, after the checksum regeneration step:
NEW_HASH=$(sha256sum "upload/sap-devs_${VERSION}_windows_amd64.zip" | awk '{print $1}')
git clone --depth 1 https://x-access-token:${GITHUB_TOKEN}@github.com/SAP-samples/sap-devs-cli.git repo
cd repo
# Update the hash in the Scoop manifest
jq --arg hash "$NEW_HASH" '.architecture["64bit"].hash = $hash' bucket/sap-devs.json > tmp.json && mv tmp.json bucket/sap-devs.json
git add bucket/sap-devs.json
git commit -m "chore: update Scoop manifest hash after signing"
git push
```

**Why this is required (not optional):** Scoop verifies SHA256 on install. The manifest is committed once by GoReleaser with the unsigned hash. The signing workflow then replaces the zip. Without updating the manifest, every Scoop install after signing will fail permanently. This is not a brief window — it is permanent until the next release.

**Homebrew:** The cask references macOS/Linux archives which are NOT signed (only Windows binaries are signed). Homebrew hashes remain valid — no update needed.

## File Changes

| File | Action |
| --- | --- |
| `.goreleaser.yml` | Add `scoops:` and `homebrew_casks:` sections |
| `bucket/.gitkeep` | Create directory placeholder (GoReleaser populates on first release) |
| `Casks/.gitkeep` | Create directory placeholder (GoReleaser populates on first release) |
| `.github/workflows/sign-windows.yml` | Add Scoop manifest hash update step after signing |
| `CLAUDE.md` | Update Release section with package manager info |
| `docs/developer/developer-guide.md` | Add installation via package managers section |
| `TODO.md` | Mark package manager publishing as done |

## Release Flow

```
v* tag push
  → Release workflow (GoReleaser)
    → builds all platform binaries
    → creates GitHub Release with archives + checksums
    → commits bucket/sap-devs.json (Scoop manifest, unsigned hash)
    → commits Casks/sap-devs.rb (Homebrew cask)
  → release:published event
    → Release Tray Binary workflow
    → Sign Windows Binaries workflow
      → signs .exe files
      → re-uploads signed archives
      → regenerates checksums
      → updates bucket/sap-devs.json with signed hash ← NEW
```

## Edge Cases

- **First release after adding config:** GoReleaser creates the manifest files for the first time. The `.gitkeep` files are overwritten.
- **Failed manifest push (branch protection):** GoReleaser logs a warning but the release still succeeds. The manifest won't update. See Token & Permissions section for resolution.
- **Windows arm64:** Excluded in the GoReleaser build matrix. Scoop manifest only references amd64, which is correct.
- **`scoop bucket list` cosmetic issue:** Scoop's `bucket list` command may show 0 manifests when they're in `bucket/` rather than root. This is cosmetic — `scoop install sap-devs` still works. If this becomes a user issue, move manifests to root.
- **Archive structure coupling:** Scoop's `bin` entry assumes `sap-devs.exe` is at the zip root (`wrap_in_directory: false`). Any change to archive structure must be reflected in the Scoop config.

## Future Considerations

- **winget:** Higher friction (PR to microsoft/winget-pkgs). Defer until Scoop/Homebrew usage is validated.
- **Version pinning:** GoReleaser handles this automatically — each release overwrites the manifest with the latest version.
- **Tap migration file:** If users previously installed via some other method, a `tap_migrations.json` could redirect them to the cask.
