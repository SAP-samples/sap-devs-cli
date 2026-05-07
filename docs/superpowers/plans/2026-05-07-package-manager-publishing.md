# Package Manager Publishing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Distribute `sap-devs` CLI via Scoop (Windows) and Homebrew (macOS/Linux) using GoReleaser's native support, with post-signing manifest correction.

**Architecture:** Add `scoops:` and `homebrew_casks:` sections to `.goreleaser.yml` so manifests are auto-generated and committed to the same repo on each tagged release. Extend the signing workflow to fix the Scoop manifest hash after re-uploading signed binaries.

**Tech Stack:** GoReleaser v2 (YAML config), GitHub Actions, jq, Scoop (JSON manifests), Homebrew (Ruby casks)

**Spec:** `docs/superpowers/specs/2026-05-07-package-manager-publishing-design.md`

---

### Task 1: Create Directory Placeholders

**Files:**
- Create: `bucket/.gitkeep`
- Create: `Casks/.gitkeep`

- [ ] **Step 1: Create bucket directory for Scoop manifests**

```bash
mkdir -p bucket && touch bucket/.gitkeep
```

- [ ] **Step 2: Create Casks directory for Homebrew casks**

```bash
mkdir -p Casks && touch Casks/.gitkeep
```

- [ ] **Step 3: Commit**

```bash
git add bucket/.gitkeep Casks/.gitkeep
git commit -m "chore: add placeholder directories for Scoop and Homebrew manifests"
```

---

### Task 2: Add Scoop and Homebrew Sections to GoReleaser

**Files:**
- Modify: `.goreleaser.yml` (append after `changelog:` section, ~line 51)

- [ ] **Step 1: Add `scoops:` section to `.goreleaser.yml`**

Append after the `changelog:` block:

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

This generates `bucket/sap-devs.json` on each release. It references the `archives` artifact (Windows amd64 zip) automatically since that's the only Windows archive.

- [ ] **Step 2: Add `homebrew_casks:` section to `.goreleaser.yml`**

Append after the `scoops:` block:

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
    license: Apache-2.0
    name: sap-devs
```

This generates `Casks/sap-devs.rb` on each release with per-platform download URLs and SHA256 hashes for macOS and Linux.

- [ ] **Step 3: Validate YAML syntax**

Run: `yq '.' .goreleaser.yml > /dev/null && echo "YAML valid"`

Expected: `YAML valid`

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yml
git commit -m "feat: add Scoop and Homebrew package manager publishing via GoReleaser"
```

---

### Task 3: Add Scoop Manifest Update to Signing Workflow

**Files:**
- Modify: `.github/workflows/sign-windows.yml` (insert new step between "Regenerate checksums" and "Set output")

The signing workflow re-uploads signed Windows archives with `--clobber`, changing the binary content. The Scoop manifest committed by GoReleaser has the **pre-signing** SHA256. Without this fix, Scoop installs permanently fail after signing.

- [ ] **Step 1: Add manifest update step to sign-windows.yml**

Insert this step after the "Regenerate checksums" step (id: `checksums`) and before the "Set output" step (id: `result`):

```yaml
      - name: Update Scoop manifest hash
        if: steps.checksums.outcome == 'success'
        continue-on-error: true
        id: scoop-manifest
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          # Calculate SHA256 of the signed Windows zip
          NEW_HASH=$(sha256sum "upload/sap-devs_${VERSION}_windows_amd64.zip" | awk '{print $1}')

          # Clone repo (shallow), update manifest, push
          git clone --depth 1 "https://x-access-token:${GH_TOKEN}@github.com/SAP-samples/sap-devs-cli.git" scoop-update
          cd scoop-update

          # Only update if the manifest exists (first release won't have it yet)
          if [ -f bucket/sap-devs.json ]; then
            git config user.email "github-actions[bot]@users.noreply.github.com"
            git config user.name "github-actions[bot]"
            jq --arg hash "$NEW_HASH" '.architecture["64bit"].hash = $hash' bucket/sap-devs.json > tmp.json && mv tmp.json bucket/sap-devs.json
            git add bucket/sap-devs.json
            git diff --cached --quiet || git commit -m "chore: update Scoop manifest hash after signing v${VERSION}"
            git push
          else
            echo "::notice::bucket/sap-devs.json not found — skipping (first release?)"
          fi
```

- [ ] **Step 2: Update the "Set output" step to include scoop-manifest status**

Change the warning line in the `result` step to also report the scoop-manifest outcome:

```yaml
      - name: Set output
        id: result
        run: |
          if [ "${{ steps.sign.outcome }}" = "success" ] && [ "${{ steps.upload.outcome }}" = "success" ]; then
            echo "signed=true" >> "$GITHUB_OUTPUT"
            echo "✅ Windows binaries signed and uploaded successfully"
            if [ "${{ steps.scoop-manifest.outcome }}" != "success" ]; then
              echo "::warning::Scoop manifest hash update failed — manual fix needed"
            fi
          else
            echo "signed=false" >> "$GITHUB_OUTPUT"
            echo "::warning::Signing incomplete — release ships with unsigned binaries"
            echo "::warning::Download: ${{ steps.download.outcome }}, Extract: ${{ steps.extract.outcome }}, Sign: ${{ steps.sign.outcome }}, Repackage: ${{ steps.repackage.outcome }}, Upload: ${{ steps.upload.outcome }}, Scoop: ${{ steps.scoop-manifest.outcome }}"
          fi
```

- [ ] **Step 3: Validate YAML syntax**

Run: `yq '.' .github/workflows/sign-windows.yml > /dev/null && echo "YAML valid"`

Expected: `YAML valid`

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/sign-windows.yml
git commit -m "fix: update Scoop manifest hash after signing to prevent checksum mismatch"
```

---

### Task 4: Update Documentation

**Files:**
- Modify: `CLAUDE.md` (~line 205, Release section)
- Modify: `docs/developer/developer-guide.md` (after the "Windows Code Signing" section)
- Modify: `TODO.md` (~line 15, package manager publishing section)

- [ ] **Step 1: Update CLAUDE.md Release section**

Replace the current Release section (line 205-207) with:

```markdown
### Release

Releases use GoReleaser triggered by `v*` tags. The binary is named `sap-devs`. Version is injected at build time via `-ldflags`. Windows `.exe` binaries (CLI and tray) are Authenticode-signed via SignPath.io as a best-effort post-release step (`.github/workflows/sign-windows.yml`), triggered after the tray workflow completes. Package manager manifests (Scoop `bucket/sap-devs.json`, Homebrew `Casks/sap-devs.rb`) are auto-generated and committed by GoReleaser on each release.
```

- [ ] **Step 2: Add package manager installation section to developer-guide.md**

Insert a new `### Installing via Package Managers` section in `docs/developer/developer-guide.md` before the "## Release Workflow" section (around line 427). Content:

```markdown
### Installing via Package Managers

#### Scoop (Windows)

```powershell
scoop bucket add sap-devs https://github.com/SAP-samples/sap-devs-cli
scoop install sap-devs
scoop update sap-devs   # to upgrade
```

#### Homebrew (macOS/Linux)

```bash
brew tap SAP-samples/sap-devs-cli https://github.com/SAP-samples/sap-devs-cli
brew install SAP-samples/sap-devs-cli/sap-devs
brew upgrade sap-devs   # to upgrade
```

Manifests are auto-generated by GoReleaser on each tagged release. Scoop manifest at `bucket/sap-devs.json`, Homebrew cask at `Casks/sap-devs.rb`.
```

- [ ] **Step 3: Mark TODO.md item as done**

Replace the "Package manager publishing" section (lines 15-31) with:

```markdown
### Package manager publishing - DONE ✔️

Implemented via GoReleaser native `scoops:` and `homebrew_casks:` sections. Manifests committed to same repo on release (`bucket/sap-devs.json`, `Casks/sap-devs.rb`). Signing workflow updates Scoop hash post-signing.
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md docs/developer/developer-guide.md TODO.md
git commit -m "docs: add package manager installation instructions"
```

---

### Task 5: Final Verification

- [ ] **Step 1: Validate all YAML files**

```bash
yq '.' .goreleaser.yml > /dev/null && echo "goreleaser OK"
yq '.' .github/workflows/sign-windows.yml > /dev/null && echo "sign-windows OK"
```

Expected: Both print OK.

- [ ] **Step 2: Dry-run GoReleaser to check config validity**

```bash
go run github.com/goreleaser/goreleaser/v2@latest check
```

Expected: No errors. Warnings about missing environment variables are expected (token not set locally).

- [ ] **Step 3: Verify build still compiles**

```bash
go build ./...
go vet ./...
```

Expected: Clean output, no errors.
