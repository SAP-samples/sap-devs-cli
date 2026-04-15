# Dynamic Content Injection Design

**Date:** 2026-04-15  
**Status:** Approved  
**Scope:** Extend the sync + inject pipeline to support inline fetch markers in `context.md` files, a terminal UI progress layer, a pre-inject freshness check, and an agent instruction pattern for tiered context delivery.

---

## Background

The current `context.md` files (e.g., the CAP pack) contain static, hand-authored content. This creates three failure modes for AI agents consuming the injected context:

- **Stale knowledge** — AI training data is behind the current CAP release cycle (monthly releases)
- **Missing tribal knowledge** — SAP-specific gotchas and annotation patterns not well-represented in training data
- **Recency gap** — release notes, breaking changes, and new APIs are entirely absent

The solution is a two-layer content model: a static curated base authored by humans, and dynamic sections that are fetched at sync time from authoritative URLs and expanded inline. A tiered agent instruction pattern further reduces context bloat by teaching agents to call the CLI for on-demand depth.

---

## Decisions

| Question | Decision |
| --- | --- |
| When does fetch happen? | Sync time — expanded content cached; inject renders from cache |
| Marker syntax | HTML comments (`<!-- sync:fetch ... -->`) — invisible in rendered Markdown |
| Fetch failure behaviour | Warn + keep previous cached expansion; never block inject |
| Terminal UI library | Bubbletea (charmbracelet) — long-term foundation for interactive commands |
| Pre-inject prompt | Check TTL staleness; prompt y/n; skippable with `--no-sync` / `--sync` |
| Agent instructions | Prose section in context.md (informal); MCP-structured variant deferred to backlog |
| Expanded file location | `context.expanded.md` alongside `context.md` in cache (official layer only — see Section 3) |
| State tracking | `sync-state.json` migrated to versioned `SyncState` struct with `Categories`, `Packs`, and `Markers` fields; marker keys are `<pack-id>::<index>` |
| Layers scanned for markers | Official layer only in this version; company/user/project layers deferred |
| max_lines vs max_tokens | `max_lines` takes precedence if both supplied; warning logged |
| Marker in code block | Markers inside fenced code blocks (` ``` `) are not expanded — parser skips markers inside open fences |
| Malformed markers | Missing `url` → sync warning + marker left unexpanded; unrecognised attributes → ignored |
| `--dry-run` + `--sync` | `--sync` triggers a real sync; `--dry-run` suppresses only the file writes of inject itself |
| First-run / never-synced | Inject prompts to sync; if user declines, falls back to `context.md` (markers rendered as comments, not fetched content) |
| "has markers" detection | Sync writes `has_markers: true` per pack entry in `sync-state.json`; inject reads this flag, avoids re-scanning `context.md` |
| Locale + expanded precedence | Locale variant wins over expansion: `context.<lang>.md` → `context.expanded.md` → `context.md`; no `context.expanded.<lang>.md` variant is generated |
| Inline sync from inject | A shared `runSync()` helper extracted from `cmd/sync.go RunE`; both `sync` command and inject's `--sync` / y-prompt path call it |
| Non-TTY inject behaviour | If stdin is not a TTY and content is stale, inject auto-proceeds with cached content (same as `--no-sync`) and prints a warning to stderr |
| `--sync` / `--no-sync` mutual exclusivity | Mutually exclusive; if both are passed inject exits with an error |

---

## Section 1: Marker Syntax

Pack authors embed fetch markers directly inside `context.md` as HTML comments. The comment is invisible in rendered Markdown and unambiguous to the parser.

```markdown
### Recent CAP Releases

<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" max_lines="60" label="CAP Release Notes (feb26)" -->

### Key Tools
- `@sap/cds-dk` — CAP development kit
```

At sync time the marker is replaced in `context.expanded.md`. The source `context.md` in the repo retains the marker intact.

### Marker Attributes

| Attribute | Required | Purpose |
| --- | --- | --- |
| `url` | yes | URL to fetch |
| `max_lines` | no | Truncate fetched content to N lines (takes precedence over `max_tokens` if both supplied) |
| `max_tokens` | no | Alternative budget in tokens |
| `label` | no | Human-readable name shown in sync progress output |
| `ttl_hours` | no | Override pack-level TTL for this marker |

Multiple markers per file are supported — each fetched independently and injected at its own position.

### Parser Rules

- Markers are single-line only: all attributes must fit on one line
- Markers inside fenced code blocks (between ` ``` ` delimiters) are **not** expanded — the parser tracks open/close fences and skips any marker found inside one
- Unrecognised attributes are silently ignored, allowing forward compatibility
- If both `max_lines` and `max_tokens` are supplied, `max_lines` wins and a warning is logged

### Failure Behaviour

If a fetch fails (network error, timeout, non-2xx response):

- The marker position retains its original comment text unchanged
- The previously cached expansion (if any) is preserved in `context.expanded.md`
- `sync-state.json` records `"ok": false` for the marker
- Sync logs a warning but exits 0
- The pre-inject check treats `ok: false` entries as stale and re-prompts on next inject

If a marker is malformed (missing required `url` attribute):

- Sync logs a warning identifying the pack and line number
- The marker is left unexpanded
- Sync exits 0

### First Test Case

The CAP pack (`content/packs/cap/context.md`) will include the first live marker pointing to the February 2026 CAP release notes:

```markdown
<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" max_lines="80" label="CAP Release Notes (feb26)" -->
```

This serves as the integration test for the full marker pipeline.

---

## Section 2: Sync Engine

Sync gains a second phase after the existing zip-download phase.

### Flow

```text
sap-devs sync
 ├── Phase 1 (existing): download + extract pack zips → cache
 │    └── fmt output as before (no Bubbletea)
 └── Phase 2 (new): marker expansion  ← Bubbletea inline progress
      ├── scan official-layer context.md files for <!-- sync:fetch --> markers
      ├── record has_markers flag per pack in sync-state.json
      ├── collect all markers
      ├── fetch URLs in parallel (10s timeout per URL, max 4 concurrent)
      ├── truncate to max_lines / max_tokens
      ├── substitute marker positions with fetched content
      └── write result to context.expanded.md in the pack cache directory
```

Phase 1 and Phase 2 run sequentially. Phase 1 completes and prints its output via `fmt` before Phase 2 starts the Bubbletea program. This prevents mixing raw fmt output with Bubbletea's inline renderer.

### Bubbletea Integration

Bubbletea runs in **inline mode** (no `WithAltScreen`) so it renders below Phase 1's output rather than taking over the terminal. The `tea.Program` is started inside the sync `RunE` function, runs to completion, then returns. No goroutine coordination with Cobra is required.

Bubbletea is chosen over lighter libraries (pterm, progressbar) because it provides the foundation for future interactive commands: the `init` wizard, `mcp list`, and interactive `profile set` flows all benefit from the same reactive model.

### Progress Display

```text
  Syncing official content...               ✓
  Expanding dynamic markers
    cap  › CAP Release Notes (feb26)        fetching...
    cap  › CAP Release Notes (feb26)        ✓  (47 lines)
    btp  › BTP Service Catalog              ✗  fetch failed, using cached
```

### sync-state.json Migration

The current `sync-state.json` is a flat `map[string]time.Time`. This design requires a nested structure. The migration strategy:

1. Introduce a new `SyncState` struct (see below) with `Version`, `Categories`, `Packs`, and `Markers` fields.
2. `loadState` attempts to unmarshal into `SyncState`; if it fails, it resets to `SyncState{Version: 1}` and logs a one-time notice: "sync state reset after format upgrade". Both the old flat-format file (`"category": "timestamp"` shape) and a genuinely corrupt file produce a JSON unmarshal error into `SyncState` — the two cases are indistinguishable and both result in a reset, which is the correct behaviour.
3. `saveState` always writes the new struct format.
4. The reset is non-destructive: the only consequence is that all packs re-sync on the next `sap-devs sync` invocation.

New Go types:

```go
type SyncState struct {
    Version    int                        `json:"version"`
    Categories map[string]time.Time       `json:"categories"`
    Packs      map[string]PackState       `json:"packs"`
    Markers    map[string]MarkerState     `json:"markers"`
}

type PackState struct {
    HasMarkers bool `json:"has_markers"`
}

type MarkerState struct {
    URL         string    `json:"url"`
    LastFetched time.Time `json:"last_fetched"`
    TTLHours    int       `json:"ttl_hours"`
    OK          bool      `json:"ok"`
}
```

### Engine API Changes

The `Engine` struct's public method signatures (`IsStale`, `MarkSynced`, `MarkAllSynced`) are unchanged. The internal `loadState` and `saveState` helpers are updated to operate on `SyncState` instead of `map[string]time.Time`; the existing methods continue to work by reading from and writing to `state.Categories`. Three new methods are added to `Engine`:

- `RecordMarkerState(packID string, index int, ms MarkerState)` — called by Phase 2 after each fetch
- `GetMarkerState(packID string, index int) (MarkerState, bool)` — called by the pre-inject staleness check
- `SetPackHasMarkers(packID string, hasMarkers bool)` — called during the marker scan in Phase 2

New `sync-state.json` format:

```json
{
  "version": 1,
  "categories": {
    "official": "2026-04-15T09:00:00Z"
  },
  "packs": {
    "cap": { "has_markers": true },
    "btp": { "has_markers": false }
  },
  "markers": {
    "cap::0": {
      "url": "https://cap.cloud.sap/docs/releases/2026/feb26",
      "last_fetched": "2026-04-15T09:00:00Z",
      "ttl_hours": 168,
      "ok": true
    }
  }
}
```

**Key format:** `<pack-id>::<index>` where index is the zero-based position of the marker in the file. This avoids collisions when the same URL appears twice in a pack, and keeps the key stable across fetches.

---

## Section 3: Cache Structure

Expanded content sits alongside the source file in the official-layer pack cache directory.

```text
~/.cache/sap-devs/
  official/
    content/
      packs/
        cap/
          context.md            ← original, markers intact (from zip)
          context.expanded.md   ← generated by sync phase 2
          tips.md
          resources.yaml
          ...
  sync-state.json               ← versioned; gains packs + markers blocks
```

Note: the existing `FetchArchive` strips one top-level prefix from the zip (e.g. `sap-devs-cli-main/`) and writes to `~/.cache/sap-devs/official/`. The zip contains `content/packs/cap/context.md`, so the on-disk path is `~/.cache/sap-devs/official/content/packs/cap/context.md`. Phase 2 marker scanning and `context.expanded.md` writing use this same path.

**Scope of marker expansion:** Only the `official` layer is scanned and expanded in this version. Company, user, and project layers may contain markers but they are treated as literal text until a future extension adds multi-layer expansion support. This is a deliberate scope boundary — the complexity of per-layer expansion and TTL tracking for non-cached layers is deferred.

`context.expanded.md` is never committed to the repo and is not user-editable. The inject pipeline reads it when present; falls back to `context.md` when absent.

### LoadPack Changes

`LoadPack` in `internal/content/pack.go` is the single point where `context.md` is read into `pack.ContextMD`. It gains the expanded-file preference with the following precedence (highest to lowest):

1. `context.<lang>.md` — locale variant (existing behaviour, unchanged)
2. `context.expanded.md` — sync-expanded content (new)
3. `context.md` — static source (existing fallback)

No `context.expanded.<lang>.md` variant is generated — locale variants are not expanded in this version. If a locale variant exists it is used as-is, without dynamic expansion.

---

## Section 4: Pre-inject Sync Check

Before `inject` renders and writes context, it evaluates freshness of expanded content.

### "Has Markers" Detection

The inject command reads the `packs` block from `sync-state.json` to determine which active packs have markers. It does **not** re-scan `context.md` at inject time.

**"Active packs"** for the purposes of the staleness check means all packs returned by `LoadPacks()` for the current profile — the same set that inject would render. The staleness check runs after `LoadPacks()` returns, so the pack list is already available with no circular dependency.

If `sync-state.json` has no `packs` block (first run, or state reset), all loaded packs are treated as potentially having markers and the check falls through to the missing-file condition.

### Staleness Conditions (evaluated in order)

1. `sync-state.json` has no `packs` block, or any active pack with `has_markers: true` has no `context.expanded.md` → **always prompt**. If the user declines (`n`), inject falls back to `context.md` in both sub-cases — the same fallback as first-run.
2. Any marker entry has `"ok": false` → **always prompt**
3. The oldest marker `last_fetched` timestamp exceeds its `ttl_hours` → **prompt**
4. All markers fresh → **proceed silently**

### Prompt

```text
  Dynamic content last synced 3 days ago (CAP Release Notes).
  Sync now for latest content? [Y/n]
```

Answering `Y` runs sync inline with the Bubbletea progress display, then continues with inject.  
Answering `n` proceeds with whatever is in cache (`context.expanded.md` if present, else `context.md`).

**Non-TTY behaviour:** When stdin is not a TTY (CI, scripts, piped input), the staleness prompt is skipped and inject proceeds with cached content — equivalent to `--no-sync`. A warning is printed to stderr: `sap-devs: dynamic content is stale; run "sap-devs sync" to refresh`.

### First-Run / Never-Synced Behaviour

On a fresh install where `sync` has never been run:

- `sync-state.json` does not exist → treated as "no packs block" → prompt fires
- If user answers `Y` → sync runs, expanded files are created, inject continues
- If user answers `n` → inject falls back to `context.md`; markers appear as HTML comments in the injected CLAUDE.md (harmless but unfetched). No error.

### Flags

| Flag | Default | Behaviour |
| --- | --- | --- |
| `--sync` | false | Force sync before inject without prompting; mutually exclusive with `--no-sync` |
| `--no-sync` | false | Skip staleness check entirely; use whatever is cached; mutually exclusive with `--sync` |
| `--dry-run` | false | Suppresses inject file writes only; a `--sync` triggered sync runs for real |

If both `--sync` and `--no-sync` are passed, inject exits immediately with: `error: --sync and --no-sync are mutually exclusive`.

### Inline Sync Path

Both the `sync` command and inject's inline sync (triggered by `--sync` or a `Y` prompt response) call a shared `runSync(ctx context.Context, force bool) error` helper extracted from `cmd/sync.go`. The Cobra `sync` command's `RunE` becomes a thin wrapper around this helper. The inject command calls the same helper, obtaining the context via `cmd.Context()` inside its own `RunE`.

---

## Section 5: Agent Instruction Pattern

Pack authors add an `### Agent Instructions` section to `context.md`. This teaches AI agents what CLI commands to invoke for on-demand depth, keeping the always-present context window lean.

### Example (CAP pack)

```markdown
### Agent Instructions

This CLI provides deeper context on demand — prefer these over web searches
for SAP-specific information:

- `sap-devs resources --pack cap` — curated CAP docs, samples, and tutorials
- `sap-devs tip --pack cap` — CAP best practice tips
- `sap-devs sync` — refresh with latest CAP release notes
```

### Tiered Context Model

| Tier | Delivery | When used |
| --- | --- | --- |
| Always-present | Static curated content + expanded markers in CLAUDE.md | Every agent turn |
| On-demand | CLI commands listed in Agent Instructions | Agent pulls when it needs depth |
| Future: structured | MCP tool registration for `sap-devs` subcommands | Deferred to backlog |

The MCP-structured variant (registering subcommands as formal MCP tools for broader agent support) is out of scope for this design and tracked in the project backlog (TODO.md).

---

## Section 6: Documentation Updates

All new features are documented as part of implementation — not as a follow-up:

- `README.md` — update `inject` and `sync` command references to describe marker syntax, pre-inject prompt, and new flags
- `content/packs/cap/context.md` — inline comments explaining marker syntax for pack authors
- `docs/content-authoring.md` (new file) — full guide covering: marker syntax, attributes, parser rules, failure behaviour, agent instructions pattern, and token budget guidance

---

## Out of Scope

- MCP-structured registration of `sap-devs` subcommands as agent tools (project backlog)
- Marker expansion for company, user, and project content layers (project backlog)
- Per-adapter `max_tokens` budgets and ranked section trimming (separate inject optimisation item in project backlog)
- Incremental inject / content hash tracking (project backlog)
- `--watch` mode for live reload during content development (project backlog)
