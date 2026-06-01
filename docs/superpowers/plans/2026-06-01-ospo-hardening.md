# OSPO Hardening Updates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move OSPO Hardening Controls 5 (branch protection) from "Action Required" to "Pass" by removing admin bypass on `main`, and refactor the news-sync workflow to a PR-based flow so it keeps working under the new restrictions.

**Architecture:** Three coordinated changes: (1) ruleset JSON swap (admin-bypass → narrow github-actions[bot] PR-merge bypass), (2) `news-sync.yml` refactor (direct push → PR + auto-merge), (3) CLAUDE.md update + admin demotion draft document. Spec at [docs/superpowers/specs/2026-06-01-ospo-hardening-design.md](../specs/2026-06-01-ospo-hardening-design.md).

**Tech Stack:** GitHub rulesets JSON, GitHub Actions YAML, `gh` CLI, bash. No application code touched, no Go changes.

---

## File Structure

| File | Action | Responsibility |
|---|---|---|
| `.github/rulesets/main-protection.json` | Modify | Source-of-truth ruleset; `bypass_actors` swap |
| `.github/workflows/news-sync.yml` | Modify | Schedule + fetch + open-PR-with-auto-merge (replaces direct push) |
| `docs/superpowers/specs/2026-06-01-ospo-hardening-design.md` | Already exists | Spec, no further edits |
| `CLAUDE.md` | Modify | Update "OSPO compliance" paragraph in the Release section to reflect new posture |
| `docs/ospo-admin-demotion-draft.md` | Create | The 16-account demotion draft from Change 3 of the spec, for the project owner to act on |

The ruleset and workflow files are tightly coupled — neither works correctly without the other — so they're staged in deployment order (workflow first, ruleset second) per the spec's "Deployment ordering" section.

**Verification note:** The plan does not include unit tests because nothing in the change is application code. The spec defines an empirical testing plan; that's reproduced as Task 8 and must be run after merge + ruleset import. Pre-merge verification is limited to syntactic validation: `jq` for the ruleset JSON (Task 1) and `yq` + `shellcheck` for the workflow YAML and embedded shell (Tasks 2–4). GitHub's REST API exposes no dry-run/check sub-resource on rulesets, so semantic validation of `actor_id` / `actor_type` / `bypass_mode` happens server-side at import time (Task 8); a 422 there fails harmlessly without changing the active ruleset.

---

## Task 1: Update ruleset JSON

**Files:**
- Modify: `.github/rulesets/main-protection.json:34-36` (`bypass_actors` array)

- [ ] **Step 1: Inspect current ruleset state**

Run: `cat .github/rulesets/main-protection.json`

Expected: file ends with the existing `bypass_actors` array containing `{"actor_id": 5, "actor_type": "RepositoryRole", "bypass_mode": "always"}`.

- [ ] **Step 2: Apply the bypass-actor swap**

Replace the contents of `.github/rulesets/main-protection.json` with:

```json
{
  "name": "main-protection",
  "target": "branch",
  "enforcement": "active",
  "conditions": {
    "ref_name": {
      "include": ["refs/heads/main"],
      "exclude": []
    }
  },
  "rules": [
    {"type": "deletion"},
    {"type": "non_fast_forward"},
    {
      "type": "pull_request",
      "parameters": {
        "required_approving_review_count": 1,
        "dismiss_stale_reviews_on_push": false,
        "require_code_owner_review": false,
        "require_last_push_approval": false,
        "required_review_thread_resolution": false
      }
    },
    {
      "type": "required_status_checks",
      "parameters": {
        "strict_required_status_checks_policy": false,
        "required_status_checks": [
          {"context": "test"}
        ]
      }
    }
  ],
  "bypass_actors": [
    {"actor_id": 15368, "actor_type": "Integration", "bypass_mode": "pull_request"}
  ]
}
```

The only substantive diff vs the previous version is the single `bypass_actors` entry: `RepositoryRole 5 / always` → `Integration 15368 / pull_request`. All other rules are unchanged.

- [ ] **Step 3: Validate JSON syntax**

```bash
cat .github/rulesets/main-protection.json | jq . > /dev/null && echo "JSON OK"
```

Expected: `JSON OK`. Exit 0.

Note: there is no public GitHub REST endpoint for *dry-run* validation of a ruleset payload (the `/repos/{owner}/{repo}/rulesets/{id}` endpoints are GET / PUT / DELETE only — no `/check` sub-resource). Semantic validation of fields like `actor_id`, `actor_type`, `bypass_mode` happens server-side at import time (Task 8, Step 2). A typo there returns a 422 with field-level errors and the import fails harmlessly without changing the active ruleset.

- [ ] **Step 4: Commit**

```bash
git add .github/rulesets/main-protection.json
git commit -m "chore(ospo): swap admin bypass for narrow github-actions[bot] bypass

Removes RepositoryRole 5 (Admin) bypass with mode 'always' and replaces
it with Integration 15368 (github-actions[bot]) bypass scoped to
'pull_request' merges only.

Effect: humans (including admins) can no longer push directly to main
or merge PRs that fail the rules. The bot can self-merge its own PRs
(e.g. news-sync) without an approver, but the required 'test' status
check still gates the merge.

Addresses OSPO Hardening Control 5.
Spec: docs/superpowers/specs/2026-06-01-ospo-hardening-design.md"
```

---

## Task 2: Add `pull-requests: write` permission and concurrency to news-sync workflow

**Files:**
- Modify: `.github/workflows/news-sync.yml:8-9` (top-level `permissions:` block)
- Modify: `.github/workflows/news-sync.yml` (add new top-level `concurrency:` block)

This task only adds the new top-level YAML keys. The actual step replacement happens in Task 3 to keep diffs reviewable.

- [ ] **Step 1: Read the current workflow**

Run: `cat .github/workflows/news-sync.yml`

Expected: file shows `permissions: contents: write` and no `concurrency:` block.

- [ ] **Step 2: Update the `permissions:` block**

In `.github/workflows/news-sync.yml`, replace:

```yaml
permissions:
  contents: write
```

with:

```yaml
permissions:
  contents: write
  pull-requests: write
```

- [ ] **Step 3: Add a top-level `concurrency:` block**

Insert immediately after the `permissions:` block (before `jobs:`):

```yaml
concurrency:
  group: news-sync
  cancel-in-progress: false
```

`cancel-in-progress: false` ensures a manual `workflow_dispatch` run started while the cron run is in flight will *queue* rather than abort it — preventing a force-push race on `bot/news-sync-update`.

- [ ] **Step 4: Validate YAML syntax**

Run: `yq '.' .github/workflows/news-sync.yml > /dev/null && echo OK`

Expected: `OK`. Exit code 0.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/news-sync.yml
git commit -m "chore(news-sync): add pull-requests:write permission and concurrency group

Prepares news-sync for PR-based flow (Task 3).

- pull-requests: write    needed for 'gh pr create' and 'gh pr merge'
- concurrency: news-sync  serializes overlapping cron + manual runs to
                          prevent force-push races on bot/news-sync-update"
```

---

## Task 3: Replace direct push with PR-based flow in news-sync

**Files:**
- Modify: `.github/workflows/news-sync.yml:32-39` (the existing "Commit if changed" step)

- [ ] **Step 1: Identify the step to replace**

Run: `grep -n "Commit if changed" .github/workflows/news-sync.yml`

Expected: matches the step with `run: |` block doing `git commit && git push`.

- [ ] **Step 2: Replace the "Commit if changed" step**

Replace the entire step (currently lines ~32–39) with:

```yaml
      - name: Open or update PR if changed
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          if git diff --quiet content/packs/base/news-episodes.json 2>/dev/null; then
            echo "No changes."
            exit 0
          fi
          BRANCH="bot/news-sync-update"
          git checkout -B "$BRANCH"
          git add content/packs/base/news-episodes.json
          git commit -m "chore: update news episode index"
          git push -f origin "$BRANCH"
          EXISTING=$(gh pr list --head "$BRANCH" --json number -q '.[0].number // empty' 2>/dev/null || echo "")
          if [ -z "$EXISTING" ]; then
            gh pr create \
              --title "chore: update news episode index" \
              --body  "Automated update of \`content/packs/base/news-episodes.json\`." \
              --base  main --head "$BRANCH"
          else
            echo "PR #$EXISTING already open; new commit pushed to $BRANCH"
          fi
          gh pr merge --auto --squash "$BRANCH" || \
            echo "::warning::auto-merge not armed; check 'Allow auto-merge' is enabled in repo Settings"
```

Key design points (cross-reference the spec's "Change 2" section):

- `gh pr merge` accepts a branch name directly — no fragile output-parsing of `gh pr create`.
- Calling `gh pr merge --auto` repeatedly is idempotent; if auto-merge is already armed, gh prints a notice and exits 0.
- The required `test` status check fires via `ci.yml`'s `push` trigger on `bot/news-sync-update`, satisfying the rule by check-run name.

- [ ] **Step 3: Validate the full YAML**

Run: `yq '.jobs."sync-news".steps' .github/workflows/news-sync.yml`

Expected: structured output listing all steps including the new `Open or update PR if changed` step. No parse errors.

- [ ] **Step 4: Lint with shellcheck (the embedded shell script)**

Extract the run block into a temp file and shellcheck it:

```bash
yq '.jobs."sync-news".steps[] | select(.name == "Open or update PR if changed") | .run' \
  .github/workflows/news-sync.yml > /tmp/news-sync-step.sh
shellcheck -s bash /tmp/news-sync-step.sh
```

Expected: no errors. (Warnings about `${BRANCH}` vs `$BRANCH` in shellcheck SC2086 are acceptable — the variable is not user-controlled.)

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/news-sync.yml
git commit -m "feat(news-sync): open auto-merging PR instead of pushing to main

The previous flow ran 'git commit && git push' directly to main. After
the OSPO hardening ruleset takes effect (sister commit), direct pushes
to main are blocked even for the bot. This step instead:

  1. Pushes to a stable bot branch (bot/news-sync-update, force-push)
  2. Opens a PR via 'gh pr create' if one isn't already open
  3. Arms 'gh pr merge --auto --squash' which fires when the test check
     passes

The bot's narrow ruleset bypass (Integration 15368 / pull_request mode)
allows the auto-merge to proceed without a human approver.

Spec: docs/superpowers/specs/2026-06-01-ospo-hardening-design.md"
```

---

## Task 4: Static validation of the workflow refactor

The spec says "Verification is empirical (the ruleset only takes effect server-side after import)" — so the meaningful verification of the new news-sync flow happens **post-merge** (Task 8). Pre-merge, the most we can soundly do is static validation: parse the YAML, shellcheck the embedded script, and confirm the workflow structure matches expectations. Running `workflow_dispatch` against the feature branch is **not safe** — `Allow auto-merge` is not yet enabled (it's a post-merge operational step), so `gh pr merge --auto` would error out and either short-circuit on `git diff --quiet` (validating nothing) or open a real PR against `main` from a feature branch run.

- [ ] **Step 1: Verify the full workflow YAML parses**

```bash
yq '.' .github/workflows/news-sync.yml > /dev/null && echo "YAML OK"
```

Expected: `YAML OK`. Exit 0.

- [ ] **Step 2: Confirm the new step is wired correctly**

```bash
yq -r '.jobs."sync-news".steps[] | select(.name == "Open or update PR if changed") | .name' \
  .github/workflows/news-sync.yml
```

Expected: prints `Open or update PR if changed`. If it prints nothing, the step name in Task 3 was applied incorrectly.

- [ ] **Step 3: Confirm `permissions:` and `concurrency:` are wired**

```bash
yq -r '.permissions["pull-requests"], .concurrency.group' .github/workflows/news-sync.yml
```

Expected output (two lines):

```text
write
news-sync
```

- [ ] **Step 4: Shellcheck the embedded script**

```bash
yq -r '.jobs."sync-news".steps[] | select(.name == "Open or update PR if changed") | .run' \
  .github/workflows/news-sync.yml > /tmp/news-sync-step.sh
shellcheck -s bash /tmp/news-sync-step.sh
```

Expected: no errors. SC2086 / SC2155 warnings about variable expansion are acceptable (variables are not user-controlled).

- [ ] **Step 5: No commit needed** — Task 4 is verification only.

**Live verification of the new flow happens post-merge in Task 8.**

---

## Task 5: Update CLAUDE.md OSPO compliance paragraph

**Files:**
- Modify: `CLAUDE.md` (the "OSPO compliance" paragraph in the Release section — locate via `grep`, line numbers shift between commits)

- [ ] **Step 1: Locate the current paragraph**

Run: `grep -n "OSPO compliance" CLAUDE.md`

Expected: matches the line containing the text `**OSPO compliance:** \`main\` is protected by ruleset...admins can bypass...`. Note the resulting line number — Step 2 modifies that single line.

- [ ] **Step 2: Replace the paragraph**

Replace the line currently reading:

```markdown
**OSPO compliance:** `main` is protected by ruleset (`main-protection`: requires PR + CI `test` check, blocks force-push and deletion, admins can bypass). Workflows that touch secrets or publish artifacts run in named environments: `release`, `signing` (required reviewer = repo admin), `news-sync`. SignPath secrets and `YOUTUBE_API_KEY` should be scoped to their respective environments rather than the org/repo level. Ruleset spec: [.github/rulesets/main-protection.json](.github/rulesets/main-protection.json).
```

with:

```markdown
**OSPO compliance:** `main` is protected by ruleset (`main-protection`: requires PR + CI `test` check, blocks force-push and deletion). Admins **cannot** bypass — every change to `main` goes through a PR with one approving review and a passing `test` check. The `github-actions[bot]` integration has a narrow `pull_request`-mode bypass so automated PRs (notably `news-sync`) can self-merge once their `test` check passes; it cannot push directly to `main`. Workflows that touch secrets or publish artifacts run in named environments: `release`, `signing` (required reviewer = repo admin), `news-sync`. SignPath secrets and `YOUTUBE_API_KEY` should be scoped to their respective environments rather than the org/repo level. Ruleset spec: [.github/rulesets/main-protection.json](.github/rulesets/main-protection.json).
```

- [ ] **Step 3: Verify the change**

Run: `grep -A1 "OSPO compliance" CLAUDE.md | head -3`

Expected: output contains "Admins **cannot** bypass".

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: reflect new OSPO posture in CLAUDE.md

Admins no longer bypass the main ruleset; only github-actions[bot] has
a narrow PR-merge bypass for automation."
```

---

## Task 6: Create admin demotion draft document

**Files:**
- Create: `docs/ospo-admin-demotion-draft.md`

This is the operational handoff for Change 3 of the spec — a document the project owner can act on without re-deriving the classification.

- [ ] **Step 1: Create the file**

Write this content to `docs/ospo-admin-demotion-draft.md`:

```markdown
# OSPO Control 7 — Admin Demotion Draft

**Status:** Draft — pending project owner action. **Nothing is executed by the OSPO hardening PR.**

OSPO Hardening Control 7 ("Access Restrictions") flagged 25 admins on this repository, well above the least-privilege threshold of 3–5. This document classifies the roster and proposes per-account actions.

## Bucket 1 — Project leads (keep admin)

| Login | Rationale |
|---|---|
| `qmacro` | Project lead |
| `jung-thomas` | Project owner |
| `ajmaradiaga` | Active maintainer (Developer Advocacy) |

## Bucket 2 — Org-mandated (cannot demote at repo level)

These admins are inherited from SAP organization-level membership or are platform automation. Removing them from the repo collaborator list does not actually remove their admin powers; that requires an OSPO ticket or org-admin action.

- `SAP-OSPO-ADMIN`
- `sap-ospo-bot`
- `SebastianWolf-SAP`
- `nicoschoenteich`
- `christianneu`
- `dellagustin-sap`

**Action:** verify with OSPO. If the inherited grant is intentional, leave alone. If a subset can be pruned via OSPO ticket, file one.

## Bucket 3 — Inherited / unclear (propose demote to `maintain` or `write`)

These accounts have admin but no obvious active stake in this repo. Recommended action: demote each to `maintain` (still allows triaging issues, accepting PRs, managing labels) or `write` (just code contribution). Confirm per-account before demoting; demotion notifies the user.

| Login | Suggested role after demotion |
|---|---|
| `akula86` | maintain |
| `ihrigb` | maintain |
| `vipinvkmenon` | maintain |
| `Sygyzmundovych` | maintain |
| `KevinRiedelsheimer` | maintain |
| `rbrainey` | maintain |
| `thecodester` | maintain |
| `ajinkyapatil8190` | maintain |
| `Shegox` | maintain |
| `rich-heilman` | maintain |
| `noravth` | maintain |
| `sheenamk` | maintain |
| `PoojaGidaveer` | maintain |
| `btbernard` | maintain |
| `ajaysoreng` | maintain |
| `neelamegams` | maintain |

## Demotion command (per account)

```bash
gh api -X PUT repos/SAP-samples/sap-devs-cli/collaborators/<login> \
  -f permission=maintain
```

Expected response: `204 No Content`. The user receives an email notification.

## Expected end state

| Bucket | Count |
|---|---|
| Admin (project leads) | 3 |
| Admin (org-mandated, awaiting OSPO) | 6 |
| Demoted to `maintain` | 16 |
| **Net admin count** | **9** (or fewer once OSPO bucket is pruned) |

This brings the repo close to OSPO Control 7's least-privilege threshold (3–5) without disrupting active maintainers.
```

- [ ] **Step 2: Verify file**

Run: `wc -l docs/ospo-admin-demotion-draft.md`

Expected: ~70 lines, exit 0.

- [ ] **Step 3: Commit**

```bash
git add docs/ospo-admin-demotion-draft.md
git commit -m "docs: admin demotion draft for OSPO Control 7

Classifies all 25 current admins into project leads (3, keep), org-mandated
(6, cannot demote at repo level), and inherited/unclear (16, propose
demote to maintain).

No demotions are executed by this PR. The project owner reviews and
acts per-account."
```

---

## Task 7: Open PR and confirm pre-merge state

- [ ] **Step 1: Push the latest state**

```bash
git push origin ospo-hardening-2026-06
```

- [ ] **Step 2: Open the PR**

```bash
gh pr create \
  --title "chore(ospo): close Hardening Controls 5 + 7 findings" \
  --body "$(cat <<'EOF'
## Summary

Closes the OSPO Hardening Controls 5 ("Action Required") and partially addresses Control 7 ("Warning") findings on this repo.

## Changes

1. **Ruleset** — \`.github/rulesets/main-protection.json\`: swap admin (\`RepositoryRole 5 / always\`) bypass for narrow \`github-actions[bot]\` (\`Integration 15368 / pull_request\`) bypass. After this, admins cannot bypass; only the bot can self-merge its own PRs (still gated by the required \`test\` status check).
2. **Workflow** — \`.github/workflows/news-sync.yml\`: refactor direct \`git push origin main\` to a PR-based flow (\`gh pr create\` + \`gh pr merge --auto --squash\`). Adds \`pull-requests: write\` permission and a \`concurrency: news-sync\` block.
3. **Docs** — \`CLAUDE.md\`: update OSPO compliance paragraph. \`docs/ospo-admin-demotion-draft.md\`: new draft document classifying the 25-admin roster for the project owner to act on (no demotions executed in this PR).

## Spec

[docs/superpowers/specs/2026-06-01-ospo-hardening-design.md](docs/superpowers/specs/2026-06-01-ospo-hardening-design.md)

## Deployment ordering (post-merge)

1. **Merge this PR** under the current admin-bypass ruleset (this is the last admin-bypassable merge).
2. **Enable** Settings → General → Pull Requests → **Allow auto-merge**.
3. **Import** the new \`main-protection\` ruleset via Settings → Rulesets → Import (use the file in \`.github/rulesets/main-protection.json\`).
4. **Verify** by triggering \`news-sync.yml\` manually and confirming a PR opens, auto-merges, and \`main\` updates.

## Rollback

\`git revert\` this PR's merge commit and re-import the previous ruleset version. One \`gh api\` call away.

EOF
)" \
  --base main --head ospo-hardening-2026-06 \
  --label "ospo,security,documentation"
```

Expected: PR URL printed.

- [ ] **Step 3: Confirm CI passes**

```bash
gh pr checks --watch
```

Expected: `test` check passes (no application code changed; failure here would indicate an unrelated CI regression).

- [ ] **Step 4: No commit needed.** Hand off to human reviewer for approval.

---

## Task 8: Post-merge verification (operational, not pre-merge)

**This task runs after the PR is merged.** It is the spec's empirical testing plan, reproduced as actionable steps. The maintainer (or the user, post-merge) executes this.

- [ ] **Step 1: Enable Allow auto-merge**

Settings → General → Pull Requests → check **Allow auto-merge** → Save.

Verify with:

```bash
gh api repos/SAP-samples/sap-devs-cli -q .allow_auto_merge
```

Expected: `true`.

- [ ] **Step 2: Import the new ruleset**

```bash
RULESET_ID=$(gh api repos/SAP-samples/sap-devs-cli/rulesets \
  -q '.[] | select(.name=="main-protection") | .id')
if [ -z "$RULESET_ID" ]; then
  echo "main-protection ruleset not found on server."
  echo "Create it via Settings → Rulesets → New ruleset → Import a ruleset"
  echo "and select .github/rulesets/main-protection.json, then re-run this step."
  exit 1
fi
gh api -X PUT "repos/SAP-samples/sap-devs-cli/rulesets/$RULESET_ID" \
  --input .github/rulesets/main-protection.json
```

Or use the GitHub UI: Settings → Rulesets → main-protection → Edit → Import → select the file.

- [ ] **Step 3: Verify direct-push to main is blocked**

From a maintainer clone:

```bash
git checkout main && git pull
echo "test" >> /tmp/scratch
git commit --allow-empty -m "should be rejected"
git push origin main
```

Expected: rejection with a ruleset violation message. Reset with `git reset --hard origin/main`.

- [ ] **Step 4: Verify a real news-sync run produces a PR**

```bash
gh workflow run news-sync.yml
sleep 60
gh pr list --head bot/news-sync-update
```

Expected: a PR exists (if there were content changes since the last sync). Watch it auto-merge once `test` passes.

- [ ] **Step 5: Verify the release pipeline still works (optional, only if a release is due)**

```bash
gh workflow run release.yml -f version=X.Y.Z   # next semver
```

Expected: tag pushes successfully; release artifacts upload. Tags are not gated by branch rulesets, so this should be unaffected.

---

## Done criteria

- [ ] All 8 tasks complete
- [ ] PR merged to main
- [ ] Ruleset imported (Step 2 of Task 8)
- [ ] OSPO Hardening dashboard shows Control 5 = Pass on next refresh
- [ ] (Project owner follow-up) Admin demotion draft acted on per-account
