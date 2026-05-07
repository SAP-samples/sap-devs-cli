# Windows EXE Code Signing with SignPath.io

**Date:** 2026-05-07
**Status:** Approved
**Scope:** Both CLI (`sap-devs.exe`) and tray (`sap-devs-tray.exe`) binaries

## Problem

Windows SmartScreen blocks or warns users when they download unsigned `.exe` files from the internet. Both `sap-devs` and `sap-devs-tray` are distributed as unsigned Windows binaries, causing friction for Windows users.

## Solution

Use [SignPath.io](https://signpath.io) (free for OSS) to Authenticode-sign all Windows `.exe` artifacts as a post-release step in GitHub Actions. Signing is best-effort — releases proceed even if signing fails.

## Prerequisites & SignPath Setup

1. **Register for SignPath OSS program** — apply at https://signpath.io/open-source with the SAP-samples/sap-devs-cli repo
2. **Install SignPath GitHub App** — grants SignPath access to download release artifacts and receive webhook notifications
3. **Create a signing policy** — configure an Authenticode signing policy in the SignPath dashboard targeting `.exe` files
4. **Add repository secrets:**
   - `SIGNPATH_API_TOKEN` — API token from SignPath dashboard
   - `SIGNPATH_ORGANIZATION_ID` — org ID from SignPath dashboard
   - `SIGNPATH_SIGNING_POLICY_SLUG` — slug of the signing policy (e.g., `release-signing`)
   - `SIGNPATH_PROJECT_SLUG` — project slug in SignPath (e.g., `sap-devs-cli`)
   - `SIGNPATH_ARTIFACT_CONFIGURATION_SLUG` — artifact configuration slug defining which files to sign (e.g., `exe-signing`)

## Workflow Architecture

### Trigger

The signing workflow uses `workflow_run` triggered after the "Release Tray Binary" workflow completes successfully. This guarantees all Windows artifacts (both CLI and tray) exist on the release before signing begins:

```
v* tag push
  → "Release" workflow (GoReleaser) → creates release with CLI .exe
  → release published event
    → "Release Tray Binary" workflow → uploads tray .exe
    → workflow completes successfully
      → "Sign Windows Binaries" workflow → signs both .exe files
```

### Why after tray (not after GoReleaser)

GoReleaser finishes in ~2 minutes. The tray matrix build takes ~8 minutes (5 platforms including Windows). Triggering after the tray workflow ensures both Windows artifacts are present.

### Extracting the release tag

`workflow_run` events do **not** propagate `GITHUB_REF_NAME` from the triggering workflow. The signing workflow uses a dual-strategy extraction with fail-fast:

1. **Job-level guard:** `if: github.event.workflow_run.conclusion == 'success'` — only run when the tray workflow succeeded
2. **Primary:** Read `github.event.workflow_run.head_branch` and validate it matches `^v[0-9]+\.[0-9]+\.[0-9]+`
3. **Fallback:** If primary fails validation, query `gh release list --limit 1 --json tagName -q '.[0].tagName'`
4. **Fail-fast:** If neither yields a valid semver tag, emit `::error::` and `exit 1` — this step does NOT use `continue-on-error`

```bash
CANDIDATE="${{ github.event.workflow_run.head_branch }}"
if [[ ! "$CANDIDATE" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  CANDIDATE=$(gh release list --limit 1 --json tagName -q '.[0].tagName')
fi
if [[ ! "$CANDIDATE" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  echo "::error::Could not determine release tag"
  exit 1
fi
echo "TAG=$CANDIDATE" >> "$GITHUB_ENV"
echo "VERSION=${CANDIDATE#v}" >> "$GITHUB_ENV"
```

Additionally, a non-tag `workflow_run` invocation (e.g., from a branch push re-running the tray workflow) is guarded by requiring the tag regex to pass — the workflow exits cleanly with an error annotation if no valid tag is found.

## Signing Flow

The workflow (`.github/workflows/sign-windows.yml`) runs on `ubuntu-latest` with `permissions: contents: write`:

1. **Extract release tag** from `github.event.workflow_run.head_branch` and set `TAG` / `VERSION` env vars

2. **Download Windows artifacts** from the release (via `gh release download`):
   - `sap-devs_<version>_windows_amd64.zip` (CLI archive containing `sap-devs.exe`)
   - `sap-devs_<version>_windows_amd64.exe` (CLI bare binary)
   - `sap-devs-tray_<version>_windows_amd64.zip` (tray archive containing `sap-devs-tray.exe`)

3. **Extract `.exe` files** from zip archives into a staging directory

4. **Submit to SignPath** for Authenticode signing:
   - Uses `SignPath/github-action-submit-signing-request` action (note: capital S/P in org name)
   - Packages all `.exe` files into a single zip artifact for submission (SignPath signs matching files inside the container)
   - Waits for signing to complete (the action polls SignPath internally)
   - Downloads the signed artifact zip containing the signed `.exe` files

5. **Repackage signed binaries:**
   - Re-zip CLI archive with signed `sap-devs.exe`
   - Re-zip tray archive with signed `sap-devs-tray.exe`
   - Replace bare binary (`sap-devs_<version>_windows_amd64.exe`) with signed version

6. **Re-upload to release** (with `--clobber`):
   - Overwrites unsigned archives/binaries with signed versions

7. **Regenerate checksums:**
   - Download existing `checksums.txt` from the release
   - Recalculate SHA256 for the 2 modified CLI artifacts (zip + bare .exe)
   - Replace only the Windows lines in `checksums.txt` (identified by `_windows_` in filename), preserving all other platform entries
   - Recalculate SHA256 for the tray zip and update `sap-devs-tray_<version>_windows_amd64.zip.sha256`
   - Regenerate `tray-checksums.txt` by re-downloading all per-platform `.sha256` files and concatenating
   - Re-upload all modified checksum files with `--clobber`

## Error Handling

- **Best-effort semantics:** Every signing step uses `continue-on-error: true`
- **Visibility:** Failed signing emits `::warning::` annotations in the workflow run summary listing which artifacts couldn't be signed
- **No release blocking:** If SignPath is unavailable or rejects the request, the release ships with unsigned binaries (same as today)
- **Workflow output:** Sets `signed: true/false` output for potential future badge/annotation use

## Verification

After downloading a signed release binary:

```powershell
Get-AuthenticodeSignature .\sap-devs.exe | Select Status, SignerCertificate
# Expected: Status=Valid, SignerCertificate shows SignPath-issued cert
```

Windows SmartScreen behavior changes:
- **Unsigned:** "Windows protected your PC — Unknown publisher"
- **Signed:** Shows publisher name; warning reduces/disappears as reputation builds

## Checksum Integrity

Signing modifies the binary content. The workflow regenerates all affected checksums after signing and re-uploads them. Users verifying checksums against release assets will always get matching hashes for the signed versions.

## Documentation Updates

After implementation:
- Mark TODO.md signing item as complete
- Add workflow to CLAUDE.md Release section
- Add "Windows Code Signing" subsection to docs/developer/developer-guide.md
