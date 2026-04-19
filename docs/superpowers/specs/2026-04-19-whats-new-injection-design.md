# What's New Injection Block — Design Spec

**Date:** 2026-04-19
**Status:** Draft
**Feature:** After `sync` pulls new content, `inject` prepends a one-shot `## What's New` block so AI agents learn about changes without the user having to tell them.

## Problem

A developer runs `sap-devs sync` then `sap-devs inject`. The AI agent's context window is refreshed, but the agent has no signal that anything changed. If the developer doesn't mention "CAP 9.8 is out," the agent continues reasoning from its training data or the previous context snapshot.

## Approach

Curated changelog entries in `pack.yaml`, relayed through a one-shot cache file, consumed and deleted by inject.

1. Content authors add a `changelog` list to `pack.yaml` with human-written entries.
2. `sync` reads these entries and writes `sync-changelog.json` to the cache directory.
3. `inject` reads the file, renders a `## What's New` block at the top of the injected content, and deletes the file after successful injection.

## Data Model

### `pack.yaml` changelog field

```yaml
id: cap
name: SAP Cloud Application Programming Model
# ...existing fields...
changelog:
  - "CAP 9.8: native SQLite support via `cds.requires.db.driver: node` (Node 22.5+)"
  - "CAP 9.8: new `cds repl --ql` query mode for interactive CQL"
```

Each entry is a plain string — one human-readable bullet. The field is optional; packs without it have no entries.

### `sync-changelog.json` (cache file)

Location: `{CacheDir}/sync-changelog.json`

```json
{
  "synced_at": "2026-04-17T15:04:05Z",
  "entries": [
    {"pack": "cap", "text": "CAP 9.8: native SQLite support via ..."},
    {"pack": "abap", "text": "New Tier-1 API for business partner validation"}
  ]
}
```

Written by `sync`, read and deleted by `inject`.

### `packMeta` struct change

Add to `internal/content/pack.go`:

```go
Changelog []string `yaml:"changelog,omitempty"`
```

This field is parsed by `packMeta` but not copied to the `Pack` struct and not used by `LoadPack()`. It exists solely so that a future strict YAML decoder would not reject the `changelog` key as unknown. The `sync` package reads `pack.yaml` via its own local `changelogMeta` struct — it does not go through `LoadPack()`.

### Types in `sync` package

`ChangelogEntry` is defined in `internal/sync/changelog.go` to avoid an import cycle (`sync` cannot import `content`):

```go
// in internal/sync/changelog.go
type ChangelogEntry struct {
    Pack string `json:"pack"`
    Text string `json:"text"`
}
```

`CollectChangelog` uses a local minimal struct to parse `pack.yaml`:

```go
type changelogMeta struct {
    ID        string   `yaml:"id"`
    Changelog []string `yaml:"changelog"`
}
```

### `DynamicContext` additions

Add to `internal/content/dynamic.go`:

```go
type WhatsNewEntry struct {
    Pack string
    Text string
}

// New fields on DynamicContext:
WhatsNew     []WhatsNewEntry
WhatsNewDate *time.Time
```

`cmd/inject.go` translates `[]sync.ChangelogEntry` → `[]content.WhatsNewEntry` at the call site, following the same pattern used for `project.Check()` → `content.ProjectFinding` translation (the `findings` loop after the `GatherDynamic` call in `cmd/inject.go`).

## Sync-side: Writing the Changelog

### Trigger point

After archive fetch succeeds in `cmd/sync.go` (Phase 1), alongside `engine.MarkAllSynced()`. Specifically, changelog collection is gated on `archiveNeedsSync` — it only runs when the archive categories are synced. Non-archive categories (`tutorials`, `events`, `youtube`, `discovery`, `learning`) do not trigger changelog collection, since pack.yaml files are only present in the archive.

### Logic

1. After `FetchArchive()` extracts the ZIP, call `CollectChangelog(packsDirs)` with a list of pack directories to scan (official cache, and company cache if configured).
2. `CollectChangelog` accepts `[]string` (multiple pack directory paths). It iterates each directory, parses `pack.yaml` using a local `changelogMeta` struct, and collects entries.
3. Entries are collected in directory order (official first, then company), preserving the layer priority.
4. If any entries exist, call `WriteChangelog(cacheDir, time.Now(), entries)` to write `sync-changelog.json`.
5. If no entries exist across all layers, do nothing (don't create an empty file).

### `WriteChangelog` signature

```go
func WriteChangelog(cacheDir string, syncedAt time.Time, entries []ChangelogEntry) error
```

The `syncedAt` parameter is passed explicitly from the caller (set once at the start of the sync run via `time.Now()`), so the function is deterministic and testable.

### Company repo

If a company repo is configured and synced, its packs directory is appended to the `packsDirs` slice passed to `CollectChangelog`. Company entries appear after official entries in the resulting list. A single `WriteChangelog` call writes all entries — there is no merge or multi-write.

### New code

File: `internal/sync/changelog.go`

Functions:

- `WriteChangelog(cacheDir string, syncedAt time.Time, entries []ChangelogEntry) error` — marshals and writes the JSON file. Returns nil without writing if entries is empty.
- `ReadChangelog(cacheDir string) ([]ChangelogEntry, time.Time, error)` — reads the file; returns nil entries and zero time if file missing
- `ConsumeChangelog(cacheDir string) error` — deletes the file; no-op if file doesn't exist
- `CollectChangelog(packsDirs []string) ([]ChangelogEntry, error)` — scans pack.yaml files across one or more pack directories and extracts changelog entries using a local YAML struct

## Inject-side: Consuming and Rendering

### Trigger point

In `cmd/inject.go`, **after** the inline sync block (which handles `--sync` and staleness prompts) and after packs are (re)loaded. This ensures that if `inject --sync` triggers an inline sync that writes a new `sync-changelog.json`, the subsequent `ReadChangelog` picks up those entries.

Concretely: the `ReadChangelog` call is placed after the pack reload at approximately line 200, alongside the dynamic context gathering phase (lines 202+).

### Flow

1. `inject` calls `sync.ReadChangelog(cacheDir)` — returns entries + timestamp.
2. Entries are translated to `[]content.WhatsNewEntry` and set on `DynamicContext.WhatsNew` and `WhatsNewDate`.
3. `RenderContext()` renders the `## What's New` block.
4. After `eng.Run()` completes successfully **and `--dry-run` is false**, `inject` calls `sync.ConsumeChangelog(cacheDir)` to delete the file. This is called unconditionally (not gated on whether `ReadChangelog` returned entries), which is safe because `ConsumeChangelog` is a no-op when the file is absent.
5. If inject fails partway, the file survives and the block appears on the next inject attempt.

### `--dry-run` behavior

`ConsumeChangelog` is skipped whenever `injectDryRun == true`, regardless of `--tool` or `--scope` flags. This is intentional: dry-run is a preview that must not have side effects. The changelog file survives for the next real inject run.

### Rendering placement

In `RenderContext()` (`internal/content/render.go`), a new conditional block is inserted between the profile name/description line and the existing `## Current Context` scratch notes block. This requires adding a new `if` block — not modifying `renderDynamic()` — since What's New is a top-level ephemeral section, not runtime metadata:

```go
// After profile line, before scratch notes:
if dynamic != nil && len(dynamic.WhatsNew) > 0 {
    // render ## What's New block
}
```

Output:

```markdown
## What's New (since last sync, 2026-04-17)
- CAP 9.8: native SQLite support via `cds.requires.db.driver: node` (Node 22.5+)
- CAP 9.8: new `cds repl --ql` query mode for interactive CQL
- ABAP: new Tier-1 API released for business partner validation
```

Pack names are not shown in the bullets — entries are self-describing. If multiple packs have entries, they appear in the order collected (official packs in directory order, then company packs).

### Edge cases

| Scenario | Behavior |
|----------|----------|
| No changelog file | `ReadChangelog` returns nil, no block rendered |
| Sync without inject | File accumulates until next inject |
| Multiple injects without sync | First inject consumes; subsequent see no file |
| `--dry-run` (any combination of `--tool`/`--scope`) | Read and render, but always skip `ConsumeChangelog` |
| Empty changelog entries in pack.yaml | Filtered out by `CollectChangelog`, not written to file |
| `inject --sync` triggers inline sync | Inline sync writes changelog, subsequent read picks it up |
| `sync --category tutorials` | Archive not synced, no changelog collection |

## Schema Update

Add to `content/schemas/pack.schema.json`. **This change must land before or alongside the first `pack.yaml` that uses `changelog`**, since the schema has `additionalProperties: false` and VS Code validation will reject unknown fields.

```json
"changelog": {
  "type": "array",
  "items": { "type": "string" },
  "description": "Human-curated change notes shown once after sync in the injected What's New block"
}
```

## Testing

### Unit tests: `internal/sync/changelog_test.go`

- `WriteChangelog` + `ReadChangelog` roundtrip (entries and timestamp preserved)
- `ReadChangelog` on missing file returns nil entries, zero time, no error
- `ConsumeChangelog` deletes the file
- `ConsumeChangelog` on missing file is a no-op (no error)
- `WriteChangelog` with empty entries does not create a file
- `CollectChangelog` reads changelog from pack.yaml files across multiple directories
- `CollectChangelog` skips packs without changelog field
- `CollectChangelog` with a local `changelogMeta` struct (not importing `content`)

### Render tests: `internal/content/render_test.go`

- When `DynamicContext.WhatsNew` is populated, output contains `## What's New` with correct date and bullets
- When `WhatsNew` is empty/nil, the section is absent
- What's New block appears before scratch notes and before runtime context

### Local verification

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run
```

Per project conventions, `go test` is validated in CI (ubuntu-latest). Local verification uses `go build ./...` and `go vet ./...`.

## Files Changed

Order matters: schema must be updated before or alongside pack.yaml changes.

| File | Change |
|------|--------|
| `content/schemas/pack.schema.json` | Add `changelog` array field |
| `internal/content/pack.go` | Add `Changelog []string` to `packMeta` (for completeness) |
| `internal/content/dynamic.go` | Add `WhatsNewEntry` type, `WhatsNew` and `WhatsNewDate` fields to `DynamicContext` |
| `internal/content/render.go` | Render `## What's New` block between profile line and scratch notes |
| `internal/content/render_test.go` | Add test case for What's New rendering |
| `internal/sync/changelog.go` | New file: `ChangelogEntry`, `changelogMeta`, `WriteChangelog`, `ReadChangelog`, `ConsumeChangelog`, `CollectChangelog` |
| `internal/sync/changelog_test.go` | New file: unit tests for changelog functions |
| `cmd/sync.go` | After archive fetch, collect from official + company pack dirs, call `WriteChangelog` |
| `cmd/inject.go` | After inline sync + pack reload, read changelog, translate to `WhatsNewEntry`, populate DynamicContext, consume after successful non-dry-run inject |
| `content/packs/cap/pack.yaml` | Seed example changelog entries |
| `CLAUDE.md` | Document the What's New injection lifecycle |
| `TODO.md` | Mark item as done |
