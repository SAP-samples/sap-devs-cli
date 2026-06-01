# OSPO Hardening Updates — Design

**Date:** 2026-06-01
**Status:** Draft (pending spec review)
**Triggered by:** SAP OSPO Hardening Controls report, Controls 5 + 7

## Background

OSPO's Hardening Controls dashboard flagged two findings on `SAP-samples/sap-devs-cli`:

| ID | Control | Status | Description |
|----|---------|--------|-------------|
| 5 | Restrict main / release branches with Branch Restrictions | ❌ Action Required | Main/release branches should be protected with branch protection rules or rulesets. |
| 7 | Access Restrictions | ⚠️ Warning | Repository access should follow least privilege with limited admin count. |

The repo *has* a ruleset on `main` ([.github/rulesets/main-protection.json](../../.github/rulesets/main-protection.json)), but it grants `bypass_mode: "always"` to the Admin role. OSPO's checker treats "admins always bypass" as effectively unprotected — anyone with admin can push directly to `main`, skipping the PR + status-check gates documented in [CLAUDE.md](../../CLAUDE.md) under "OSPO compliance."

A separate audit shows the repo carries **25 admin collaborators**, well above OSPO's least-privilege threshold of 3–5. The two findings are coupled: while admins can bypass the ruleset, the size of the admin set determines how many people can effectively rewrite `main`.

## Goals

- **G1.** Move Control 5 from "Action Required" to "Pass" by removing admin bypass on the `main` ruleset.
- **G2.** Keep the existing automation (release pipeline, scheduled news-sync) working without granting blanket bypass.
- **G3.** Provide the project owner a per-account demotion plan for Control 7 — without executing demotions in this change.
- **G4.** Document the rollback path so any unintended breakage is one revert away.

## Non-goals

- Demoting any admin in this PR. The drafted list is for the project owner to act on (via OSPO ticket or org-admin tooling).
- Tightening any rule beyond what Control 5 demands (no `required_signatures`, no `required_linear_history`, no review-thread-resolution requirement). Each adds friction without addressing the flagged finding.
- Touching the release pipeline. It runs as `workflow_dispatch` and pushes a *tag* (not to `main`), which is unaffected by branch rulesets.

## Design

### Change 1 — Ruleset bypass scope

Edit [.github/rulesets/main-protection.json](../../.github/rulesets/main-protection.json):

```diff
   "bypass_actors": [
-    {"actor_id": 5, "actor_type": "RepositoryRole", "bypass_mode": "always"}
+    {"actor_id": 15368, "actor_type": "Integration", "bypass_mode": "pull_request"}
   ]
```

| Field | Value | Meaning |
|-------|-------|---------|
| `actor_id: 15368` | GitHub Actions integration ID | The well-known, stable ID for the `github-actions[bot]` |
| `actor_type: "Integration"` | GitHub App / integration class | Distinct from `RepositoryRole` (humans by role) |
| `bypass_mode: "pull_request"` | Narrow bypass | Bot can merge a PR that doesn't satisfy rules; **cannot** push directly to `main` |

After this change:

- No human, including admins, can `git push origin main`. Every change goes through a PR with ≥1 approver and a passing `test` status check.
- `github-actions[bot]` PRs can self-merge without an approver, but the `test` status check still has to pass (it is a `required_status_checks` rule, not a bypass-eligible one).
- Direct push to `main` from any actor is impossible — the integration's bypass is scoped to PR-merge, not push.

The ruleset JSON in this repo is a **declarative source of truth**, not auto-applied. The owner (or an org admin) imports it via the GitHub UI ("Import a ruleset") or the API. Updating the file is the engineering deliverable; applying it is an operational step.

### Change 2 — news-sync workflow refactor

[.github/workflows/news-sync.yml](../../.github/workflows/news-sync.yml) currently does `git commit && git push` directly to `main` after fetching the news episode index. With Change 1 applied, this fails: the bot's bypass is `"pull_request"` only, not `"always"`.

Replace the final commit-and-push block with a PR-based flow that uses only the built-in `gh` CLI (no third-party action introduced):

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
    EXISTING=$(gh pr list --head "$BRANCH" --json number -q '.[0].number' || echo "")
    if [ -z "$EXISTING" ]; then
      PR_NUM=$(gh pr create \
        --title "chore: update news episode index" \
        --body  "Automated update of \`content/packs/base/news-episodes.json\`." \
        --base  main --head "$BRANCH" \
        --label "automation,news-sync" | tail -1 | grep -oE '[0-9]+$')
      gh pr merge --auto --squash "$PR_NUM"
    else
      echo "PR #$EXISTING already open; new commit pushed to $BRANCH"
    fi
```

Behaviour:

- A stable bot branch (`bot/news-sync-update`) is force-pushed each run. One PR, refreshed in place, until merged or closed.
- `gh pr merge --auto --squash` queues the merge to fire once the `test` check passes. No human approval required (Change 1's narrow bypass).
- If a PR is already open against the same branch (e.g., test failure left it open), the next run just refreshes the branch and skips PR creation.
- The cron cadence stays at 2x/day. Net user-visible change: the news index updates land via reviewable PR commits instead of opaque direct pushes.

### Change 3 — Admin demotion draft (Control 7)

The current admin roster (25 accounts) classified for the project owner:

| Bucket | Logins | Count | Recommendation |
|--------|--------|-------|----------------|
| **Project leads — keep admin** | `qmacro`, `jung-thomas`, `ajmaradiaga` | 3 | Keep |
| **Org-mandated — cannot demote** | `SAP-OSPO-ADMIN`, `sap-ospo-bot`, `SebastianWolf-SAP`, `nicoschoenteich`, `christianneu`, `dellagustin-sap` | 6 | Verify with OSPO; out of repo-owner reach |
| **Inherited / unclear — propose demote to `maintain`** | `akula86`, `ihrigb`, `vipinvkmenon`, `Sygyzmundovych`, `KevinRiedelsheimer`, `rbrainey`, `thecodester`, `ajinkyapatil8190`, `Shegox`, `rich-heilman`, `noravth`, `sheenamk`, `PoojaGidaveer`, `btbernard`, `ajaysoreng`, `neelamegams` | 16 | Demote unless confirmed active maintainer |

If the bottom bucket is fully demoted, the admin count drops from 25 to ~9 (3 leads + 6 org-mandated). If the org-mandated set can also be pruned via OSPO, the count approaches the 3–5 threshold OSPO checks for.

**No demotions execute as part of this change.** The list is for the project owner to confirm per-account and act on via the GitHub UI or `gh api`.

## Data flow

```
┌─────────────────┐
│   Cron 2x/day   │
└────────┬────────┘
         ▼
┌─────────────────┐
│ news-sync       │
│ workflow        │
└────────┬────────┘
         │ git push -f origin bot/news-sync-update    (allowed — feature branch)
         │ gh pr create --base main
         ▼
┌─────────────────────────────────┐
│ PR (bot-authored, auto-merge)   │
│  ─ test status check runs       │
│  ─ no human approval needed     │
│    (bypass: Integration 15368)  │
└────────┬────────────────────────┘
         │ checks pass + gh pr merge --auto
         ▼
┌─────────────────┐
│   main updated  │
└─────────────────┘
```

Human-authored PRs follow the same path but without the bypass — they need 1 approving review and a passing `test` check.

## Error handling & rollback

| Scenario | Handling |
|---|---|
| Ruleset breaks an unforeseen workflow | `git revert` the ruleset commit and re-import via the GitHub UI / `gh api`. The old (admin-bypass) state is one revert away. |
| news-sync PR can't auto-merge (test fails on the bot's PR) | PR stays open; next cron run force-pushes to the same branch, refreshing the diff. A human can intervene. No data loss. |
| `bot/news-sync-update` branch grows stale during a long test outage | Force-push semantics keep the branch current; the PR auto-updates. |
| GitHub Actions Integration ID changes (~unprecedented) | Update `actor_id` in the ruleset. |
| Release pipeline regression | Unaffected — release pushes a tag, not to `main`. Tags are not gated by branch rulesets. |

## Testing plan

Verification is empirical (the ruleset only takes effect server-side after import):

1. **Direct push blocked** — apply the new ruleset, attempt `git push origin main` from a maintainer clone. Must fail with a ruleset violation.
2. **PR approval enforced** — open a PR with no approvers; the GitHub UI "Merge" button must be greyed out for humans.
3. **Bot auto-merge works** — `gh workflow run news-sync.yml` (or wait for the next cron). Verify: PR opens, `test` check runs, PR auto-merges, `main` reflects the update.
4. **Stale-PR behaviour** — manually fail the `test` check on a bot PR (e.g., temporarily push a broken commit to the same branch); confirm the PR stays open and the next run refreshes it.
5. **Release pipeline unchanged** — `gh workflow run release.yml -f version=<next>` on a test version. Tag pushes successfully; release artifacts upload.

## Open questions

None blocking. Items the project owner needs to follow up on after this change merges:

- File an OSPO ticket to prune the org-mandated admin bucket if least-privilege thresholds need to be met.
- Decide per-account on the 16 "Inherited / unclear" admins listed above.

## Files touched

- [.github/rulesets/main-protection.json](../../.github/rulesets/main-protection.json) — bypass actor swap
- [.github/workflows/news-sync.yml](../../.github/workflows/news-sync.yml) — direct push → PR with auto-merge
- [docs/superpowers/specs/2026-06-01-ospo-hardening-design.md](2026-06-01-ospo-hardening-design.md) — this spec (new file)
- [CLAUDE.md](../../CLAUDE.md) — update "OSPO compliance" paragraph to reflect the new posture (admins no longer bypass; bot has narrow PR-merge bypass)
