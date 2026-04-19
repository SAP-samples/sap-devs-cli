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

No changes to the `Pack` struct — changelog entries don't flow through the content pipeline. They're extracted at sync time and written to the cache file.

### `DynamicContext` additions

Add to `internal/content/dynamic.go`:

```go
type ChangelogEntry struct {
    Pack string
    Text string
}

// New fields on DynamicContext:
WhatsNew     []ChangelogEntry
WhatsNewDate *time.Time
```

## Sync-side: Writing the Changelog

### Trigger point

After archive fetch succeeds in `cmd/sync.go` (Phase 1), alongside `engine.MarkAllSynced()`.

### Logic

1. After `FetchArchive()` extracts the ZIP, iterate each `content/packs/<id>/pack.yaml` in the official cache directory.
2. Parse the `changelog` field from each `pack.yaml` using the existing `packMeta` struct.
3. Collect all non-empty entries into `[]ChangelogEntry{Pack, Text}`.
4. If any entries exist, write `sync-changelog.json` to `{CacheDir}/`.
5. If no entries exist, do nothing (don't create an empty file).

### Company repo

If a company repo is also synced, its pack changelog entries are appended to the same file. The `synced_at` timestamp is set once at the start of the sync run.

### New code

File: `internal/sync/changelog.go`

Functions:
- `WriteChangelog(cacheDir string, entries []ChangelogEntry) error` — marshals and writes the JSON file
- `ReadChangelog(cacheDir string) ([]ChangelogEntry, time.Time, error)` — reads the file; returns nil entries and zero time if file missing
- `ConsumeChangelog(cacheDir string) error` — deletes the file
- `CollectChangelog(packsDir string) ([]ChangelogEntry, error)` — scans pack.yaml files and extracts changelog entries

## Inject-side: Consuming and Rendering

### Trigger point

In `cmd/inject.go`, after loading packs and before creating the adapter engine.

### Flow

1. `inject` calls `sync.ReadChangelog(cacheDir)` — returns entries + timestamp.
2. Entries are set on `DynamicContext.WhatsNew` and `WhatsNewDate`.
3. `RenderContext()` renders the `## What's New` block.
4. After `eng.Run()` completes successfully, `inject` calls `sync.ConsumeChangelog(cacheDir)` to delete the file.
5. If inject fails partway, the file survives and the block appears on the next inject attempt.

### Rendering placement

In `RenderContext()` (internal/content/render.go), after the profile line and before the scratch notes block:

```markdown
## What's New (since last sync, 2026-04-17)
- CAP 9.8: native SQLite support via `cds.requires.db.driver: node` (Node 22.5+)
- CAP 9.8: new `cds repl --ql` query mode for interactive CQL
- ABAP: new Tier-1 API released for business partner validation
```

Pack names are not shown in the bullets — entries are self-describing. If multiple packs have entries, they appear in pack-weight order (higher-weight packs first, matching the order packs are loaded).

### Edge cases

| Scenario | Behavior |
|----------|----------|
| No changelog file | `ReadChangelog` returns nil, no block rendered |
| Sync without inject | File accumulates until next inject |
| Multiple injects without sync | First inject consumes; subsequent see no file |
| `--dry-run` | Read and render, but skip `ConsumeChangelog` |
| Empty changelog entries in pack.yaml | Filtered out, not written to file |

## Schema Update

Add to `content/schemas/pack.schema.json`:

```json
"changelog": {
  "type": "array",
  "items": { "type": "string" },
  "description": "Human-curated change notes shown once after sync in the injected What's New block"
}
```

## Testing

### Unit tests: `internal/sync/changelog_test.go`

- `WriteChangelog` + `ReadChangelog` roundtrip
- `ReadChangelog` on missing file returns nil entries, zero time, no error
- `ConsumeChangelog` deletes the file
- `ConsumeChangelog` on missing file is a no-op (no error)
- `WriteChangelog` with empty entries does not create a file
- `CollectChangelog` reads changelog from pack.yaml files

### Render tests: `internal/content/render_test.go`

- When `DynamicContext.WhatsNew` is populated, output contains `## What's New` with correct date and bullets
- When `WhatsNew` is empty/nil, the section is absent

### Local verification

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run
```

Per project conventions, `go test` is validated in CI (ubuntu-latest). Local verification uses `go build ./...` and `go vet ./...`.

## Files Changed

| File | Change |
|------|--------|
| `internal/content/pack.go` | Add `Changelog []string` to `packMeta` |
| `internal/content/dynamic.go` | Add `ChangelogEntry` type, `WhatsNew` and `WhatsNewDate` fields to `DynamicContext` |
| `internal/content/render.go` | Render `## What's New` block after profile line |
| `internal/content/render_test.go` | Add test case for What's New rendering |
| `internal/sync/changelog.go` | New file: `WriteChangelog`, `ReadChangelog`, `ConsumeChangelog`, `CollectChangelog` |
| `internal/sync/changelog_test.go` | New file: unit tests for changelog functions |
| `cmd/sync.go` | After archive fetch, call `CollectChangelog` + `WriteChangelog` |
| `cmd/inject.go` | Read changelog, populate DynamicContext, consume after successful inject |
| `content/schemas/pack.schema.json` | Add `changelog` array field |
| `content/packs/cap/pack.yaml` | Seed example changelog entries |
| `CLAUDE.md` | Document the What's New injection lifecycle |
| `TODO.md` | Mark item as done |
