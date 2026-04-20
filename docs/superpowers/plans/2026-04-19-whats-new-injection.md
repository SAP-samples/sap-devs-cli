# What's New Injection Block — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After `sync` pulls new pack content, `inject` prepends a one-shot `## What's New` block to the AI context so agents learn what changed.

**Architecture:** Curated `changelog` entries in `pack.yaml` are collected by `sync` into a one-shot `sync-changelog.json` cache file. `inject` reads the file, renders it as a `## What's New` section at the top of the injected markdown, and deletes the file after successful injection.

**Tech Stack:** Go, YAML (`gopkg.in/yaml.v3`), JSON (`encoding/json`), testify

**Spec:** `docs/superpowers/specs/2026-04-19-whats-new-injection-design.md`

---

### Task 1: Schema + packMeta — allow `changelog` in pack.yaml

**Files:**
- Modify: `content/schemas/pack.schema.json`
- Modify: `internal/content/pack.go`

The schema must land before any pack.yaml uses the field (`additionalProperties: false` rejects unknown keys).

- [ ] **Step 1: Add `changelog` to JSON schema**

In `content/schemas/pack.schema.json`, add inside the `"properties"` object (after the `"versions"` field):

```json
    "changelog": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Human-curated change notes shown once after sync in the injected What's New block"
    }
```

- [ ] **Step 2: Add `Changelog` to `packMeta` struct**

In `internal/content/pack.go`, add to the `packMeta` struct (after the `Versions` field, before the closing `}`):

```go
	Changelog        []string                  `yaml:"changelog,omitempty"`
```

This field is parsed but not copied to `Pack`. It exists so a future strict YAML decoder won't reject the key.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: success, no errors

- [ ] **Step 4: Commit**

```bash
git add content/schemas/pack.schema.json internal/content/pack.go
git commit -m "feat: add changelog field to pack.yaml schema and packMeta"
```

---

### Task 2: Sync-side changelog collection and file writing

**Files:**
- Create: `internal/sync/changelog.go`
- Create: `internal/sync/changelog_test.go`

- [ ] **Step 1: Write failing tests for changelog functions**

Create `internal/sync/changelog_test.go`:

```go
package sync_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sapSync "github.com/SAP-samples/sap-devs-cli/internal/sync"
)

func TestWriteReadChangelog_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	entries := []sapSync.ChangelogEntry{
		{Pack: "cap", Text: "CAP 9.8: native SQLite support"},
		{Pack: "abap", Text: "New Tier-1 API for business partner"},
	}
	syncedAt := time.Date(2026, 4, 17, 15, 4, 5, 0, time.UTC)

	require.NoError(t, sapSync.WriteChangelog(dir, syncedAt, entries))

	gotEntries, gotTime, err := sapSync.ReadChangelog(dir)
	require.NoError(t, err)
	assert.Equal(t, entries, gotEntries)
	assert.True(t, syncedAt.Equal(gotTime))
}

func TestReadChangelog_MissingFile(t *testing.T) {
	dir := t.TempDir()
	entries, ts, err := sapSync.ReadChangelog(dir)
	assert.NoError(t, err)
	assert.Nil(t, entries)
	assert.True(t, ts.IsZero())
}

func TestConsumeChangelog_DeletesFile(t *testing.T) {
	dir := t.TempDir()
	entries := []sapSync.ChangelogEntry{{Pack: "cap", Text: "test"}}
	require.NoError(t, sapSync.WriteChangelog(dir, time.Now(), entries))

	require.NoError(t, sapSync.ConsumeChangelog(dir))

	_, err := os.Stat(filepath.Join(dir, "sync-changelog.json"))
	assert.True(t, os.IsNotExist(err))
}

func TestConsumeChangelog_MissingFile_NoOp(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, sapSync.ConsumeChangelog(dir))
}

func TestWriteChangelog_EmptyEntries_NoFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, sapSync.WriteChangelog(dir, time.Now(), nil))

	_, err := os.Stat(filepath.Join(dir, "sync-changelog.json"))
	assert.True(t, os.IsNotExist(err))
}

func TestCollectChangelog_ReadsFromMultipleDirs(t *testing.T) {
	// Create two pack directories with changelog entries
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	capDir := filepath.Join(dir1, "cap")
	require.NoError(t, os.MkdirAll(capDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(capDir, "pack.yaml"), []byte(`id: cap
name: CAP
description: test
tags: [test]
changelog:
  - "CAP 9.8: native SQLite"
  - "CAP 9.8: cds repl --ql"
`), 0644))

	abapDir := filepath.Join(dir2, "abap")
	require.NoError(t, os.MkdirAll(abapDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(abapDir, "pack.yaml"), []byte(`id: abap
name: ABAP
description: test
tags: [test]
changelog:
  - "New Tier-1 API"
`), 0644))

	entries, err := sapSync.CollectChangelog([]string{dir1, dir2})
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, "cap", entries[0].Pack)
	assert.Equal(t, "CAP 9.8: native SQLite", entries[0].Text)
	assert.Equal(t, "cap", entries[1].Pack)
	assert.Equal(t, "CAP 9.8: cds repl --ql", entries[1].Text)
	assert.Equal(t, "abap", entries[2].Pack)
	assert.Equal(t, "New Tier-1 API", entries[2].Text)
}

func TestCollectChangelog_SkipsPacksWithoutChangelog(t *testing.T) {
	dir := t.TempDir()
	capDir := filepath.Join(dir, "cap")
	require.NoError(t, os.MkdirAll(capDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(capDir, "pack.yaml"), []byte(`id: cap
name: CAP
description: test
tags: [test]
`), 0644))

	entries, err := sapSync.CollectChangelog([]string{dir})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCollectChangelog_SkipsMissingDirs(t *testing.T) {
	entries, err := sapSync.CollectChangelog([]string{"/nonexistent/path"})
	require.NoError(t, err)
	assert.Empty(t, entries)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/sync/...`
Expected: compilation error — `sapSync.ChangelogEntry`, `sapSync.WriteChangelog`, etc. undefined

- [ ] **Step 3: Implement changelog.go**

Create `internal/sync/changelog.go`:

```go
package sync

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ChangelogEntry is a single human-curated change note from a pack.
type ChangelogEntry struct {
	Pack string `json:"pack"`
	Text string `json:"text"`
}

type changelogFile struct {
	SyncedAt time.Time        `json:"synced_at"`
	Entries  []ChangelogEntry `json:"entries"`
}

type changelogMeta struct {
	ID        string   `yaml:"id"`
	Changelog []string `yaml:"changelog"`
}

const changelogFilename = "sync-changelog.json"

// WriteChangelog writes changelog entries to sync-changelog.json in cacheDir.
// Returns nil without writing if entries is empty.
func WriteChangelog(cacheDir string, syncedAt time.Time, entries []ChangelogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	cf := changelogFile{SyncedAt: syncedAt, Entries: entries}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, changelogFilename), data, 0600)
}

// ReadChangelog reads sync-changelog.json from cacheDir.
// Returns nil entries and zero time if the file is missing.
func ReadChangelog(cacheDir string) ([]ChangelogEntry, time.Time, error) {
	data, err := os.ReadFile(filepath.Join(cacheDir, changelogFilename))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, err
	}
	var cf changelogFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, time.Time{}, err
	}
	return cf.Entries, cf.SyncedAt, nil
}

// ConsumeChangelog deletes sync-changelog.json from cacheDir.
// No-op if the file does not exist.
func ConsumeChangelog(cacheDir string) error {
	err := os.Remove(filepath.Join(cacheDir, changelogFilename))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// CollectChangelog scans pack.yaml files across one or more pack directories
// and extracts changelog entries. Directories that don't exist are silently skipped.
func CollectChangelog(packsDirs []string) ([]ChangelogEntry, error) {
	var all []ChangelogEntry
	for _, dir := range packsDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // skip missing directories
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			packYAML := filepath.Join(dir, entry.Name(), "pack.yaml")
			data, err := os.ReadFile(packYAML)
			if err != nil {
				continue
			}
			var meta changelogMeta
			if err := yaml.Unmarshal(data, &meta); err != nil {
				continue
			}
			for _, text := range meta.Changelog {
				if text != "" {
					all = append(all, ChangelogEntry{Pack: meta.ID, Text: text})
				}
			}
		}
	}
	return all, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./internal/sync/...` and `go vet ./internal/sync/...`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add internal/sync/changelog.go internal/sync/changelog_test.go
git commit -m "feat: add sync changelog collection and file lifecycle functions"
```

---

### Task 3: DynamicContext — add WhatsNew fields

**Files:**
- Modify: `internal/content/dynamic.go`

- [ ] **Step 1: Add WhatsNewEntry type and fields to DynamicContext**

In `internal/content/dynamic.go`, add after the `CommandInfo` struct (at the end of the file):

```go
// WhatsNewEntry is a single changelog item rendered in the ## What's New block.
type WhatsNewEntry struct {
	Pack string
	Text string
}
```

Then add two fields to the `DynamicContext` struct, after `ScratchNotes`:

```go
	WhatsNew     []WhatsNewEntry
	WhatsNewDate *time.Time
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/content/dynamic.go
git commit -m "feat: add WhatsNewEntry type and fields to DynamicContext"
```

---

### Task 4: Render the ## What's New block

**Files:**
- Modify: `internal/content/render.go`
- Modify: `internal/content/render_test.go`

- [ ] **Step 1: Write failing tests for What's New rendering**

Append to `internal/content/render_test.go`:

```go
func TestRenderContext_WhatsNew_RenderedWhenPresent(t *testing.T) {
	syncDate := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	dyn := &content.DynamicContext{
		WhatsNew: []content.WhatsNewEntry{
			{Pack: "cap", Text: "CAP 9.8: native SQLite support"},
			{Pack: "abap", Text: "New Tier-1 API"},
		},
		WhatsNewDate: &syncDate,
	}
	out := content.RenderContext(nil, nil, dyn)
	assert.Contains(t, out, "## What's New (since last sync, 2026-04-17)")
	assert.Contains(t, out, "- CAP 9.8: native SQLite support")
	assert.Contains(t, out, "- New Tier-1 API")
}

func TestRenderContext_WhatsNew_OmittedWhenEmpty(t *testing.T) {
	dyn := &content.DynamicContext{WhatsNew: nil}
	out := content.RenderContext(nil, nil, dyn)
	assert.NotContains(t, out, "What's New")
}

func TestRenderContext_WhatsNew_BeforeScratchNotes(t *testing.T) {
	syncDate := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	dyn := &content.DynamicContext{
		WhatsNew:     []content.WhatsNewEntry{{Pack: "cap", Text: "test change"}},
		WhatsNewDate: &syncDate,
		ScratchNotes: []string{"working on auth"},
	}
	out := content.RenderContext(nil, nil, dyn)
	whatsNewIdx := strings.Index(out, "## What's New")
	scratchIdx := strings.Index(out, "## Current Context")
	require.NotEqual(t, -1, whatsNewIdx, "What's New must be present")
	require.NotEqual(t, -1, scratchIdx, "scratch notes must be present")
	assert.Less(t, whatsNewIdx, scratchIdx, "What's New must appear before scratch notes")
}

func TestRenderContext_WhatsNew_BeforeRuntimeContext(t *testing.T) {
	syncDate := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	dyn := &content.DynamicContext{
		CLIVersion:   "1.0.0",
		WhatsNew:     []content.WhatsNewEntry{{Pack: "cap", Text: "test change"}},
		WhatsNewDate: &syncDate,
	}
	out := content.RenderContext(nil, nil, dyn)
	whatsNewIdx := strings.Index(out, "## What's New")
	runtimeIdx := strings.Index(out, "## sap-devs Runtime Context")
	require.NotEqual(t, -1, whatsNewIdx, "What's New must be present")
	require.NotEqual(t, -1, runtimeIdx, "runtime section must be present")
	assert.Less(t, whatsNewIdx, runtimeIdx, "What's New must appear before runtime context")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/content/...`
Expected: compilation succeeds (types exist from Task 3), but the test assertions will fail because `RenderContext` doesn't render the block yet.

Run: `go vet ./internal/content/...`
Expected: success

- [ ] **Step 3: Add What's New rendering to RenderContext**

In `internal/content/render.go`, in the `RenderContext` function, insert a new block **between** the profile line block (ending at line 36 with `}`) and the scratch notes block (starting at line 38 with `if dynamic != nil && len(dynamic.ScratchNotes) > 0`):

```go
	if dynamic != nil && len(dynamic.WhatsNew) > 0 {
		if dynamic.WhatsNewDate != nil {
			b.WriteString(fmt.Sprintf("## What's New (since last sync, %s)\n\n",
				dynamic.WhatsNewDate.Format("2006-01-02")))
		} else {
			b.WriteString("## What's New\n\n")
		}
		for _, entry := range dynamic.WhatsNew {
			b.WriteString("- " + entry.Text + "\n")
		}
		b.WriteString("\n")
	}
```

The insertion point is after line 36 (`}` closing the profile block) and before line 38 (`if dynamic != nil && len(dynamic.ScratchNotes) > 0`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./internal/content/...` and `go vet ./internal/content/...`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add internal/content/render.go internal/content/render_test.go
git commit -m "feat: render What's New block in injected context"
```

---

### Task 5: Wire sync — collect changelog after archive fetch

**Files:**
- Modify: `cmd/sync.go`

- [ ] **Step 1: Add changelog collection after archive fetch**

In `cmd/sync.go`, inside the `if archiveNeedsSync {` block, after the marker expansion block (after line 122 `}`) and before the company repo block (line 124 `if cfg.CompanyRepo != ""`), restructure to collect changelog from both official and company repos:

Replace the section from line 118 (`// Phase 2: marker expansion`) through line 137 (end of company repo block `}`) with:

```go
		// Phase 2: marker expansion (Bubbletea progress)
		if err := runMarkerExpansion(officialCache, engine); err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: marker expansion warning: %v\n", err)
		}

		// Collect changelog entries from official packs
		changelogDirs := []string{filepath.Join(officialCache, "content", "packs")}

		// Sync company repo if configured
		if cfg.CompanyRepo != "" {
			if !strings.HasPrefix(cfg.CompanyRepo, "https://") {
				fmt.Fprintln(out, i18n.Tf(i18n.ActiveLang, "sync.warn_https", map[string]any{"URL": cfg.CompanyRepo}))
			} else {
				companyCache := filepath.Join(paths.CacheDir, "company")
				repoURL := strings.TrimRight(cfg.CompanyRepo, "/")
				companyArchive := repoURL + "/archive/refs/heads/main.zip"
				fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.syncing_company"))
				if err := sapSync.FetchArchive(companyArchive, companyCache, token); err != nil {
					fmt.Fprintln(out, i18n.Tf(i18n.ActiveLang, "sync.warn_company_failed", map[string]any{"Err": err}))
				} else {
					changelogDirs = append(changelogDirs, filepath.Join(companyCache, "content", "packs"))
				}
			}
		}

		// Write changelog file for inject to consume
		syncedAt := time.Now()
		clEntries, err := sapSync.CollectChangelog(changelogDirs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: changelog collection warning: %v\n", err)
		}
		if writeErr := sapSync.WriteChangelog(paths.CacheDir, syncedAt, clEntries); writeErr != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: changelog write warning: %v\n", writeErr)
		}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add cmd/sync.go
git commit -m "feat: collect pack changelog entries during sync"
```

---

### Task 6: Wire inject — read, populate, and consume changelog

**Files:**
- Modify: `cmd/inject.go`

- [ ] **Step 1: Read changelog after inline sync + pack reload, populate DynamicContext**

In `cmd/inject.go`, after the staleness check block (ending at line 200 with `}`) and before the learning journeys resolution (line 202 `// Resolve featured learning journeys`), add:

```go
		// Read changelog for What's New injection block
		clEntries, clTime, _ := sapSync.ReadChangelog(paths.CacheDir)
```

Then after the scratch notes block (after line 273 `}`) and before the adapter options (line 275 `opts := adapter.Options{`), add the translation from `sync.ChangelogEntry` to `content.WhatsNewEntry`:

```go
		// Translate sync changelog entries to content WhatsNewEntry for rendering
		if len(clEntries) > 0 {
			for _, e := range clEntries {
				dynCtx.WhatsNew = append(dynCtx.WhatsNew, content.WhatsNewEntry{
					Pack: e.Pack,
					Text: e.Text,
				})
			}
			dynCtx.WhatsNewDate = &clTime
		}
```

- [ ] **Step 2: Consume changelog after successful non-dry-run inject**

In `cmd/inject.go`, immediately after line 296 (`fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(...))`), still inside the `if !injectDryRun {` block (line 295) but before the `if injectTool == ""` check (line 297), add:

```go
			_ = sapSync.ConsumeChangelog(paths.CacheDir)
```

This must stay inside `if !injectDryRun` but outside the `if injectTool == ""` guard, so the changelog is consumed regardless of which tool was targeted.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add cmd/inject.go
git commit -m "feat: wire inject to read, render, and consume changelog"
```

---

### Task 7: Seed example changelog in cap pack

**Files:**
- Modify: `content/packs/cap/pack.yaml`

- [ ] **Step 1: Add changelog entries to cap pack**

Append to `content/packs/cap/pack.yaml` (after the `versions` block):

```yaml
changelog:
  - "CAP 9.8: native SQLite support via `cds.requires.db.driver: node` (Node 22.5+)"
  - "CAP 9.8: new `cds repl --ql` query mode for interactive CQL"
```

- [ ] **Step 2: Verify with dry-run**

Run: `SAP_DEVS_DEV=1 go run . sync --force` then `SAP_DEVS_DEV=1 go run . inject --dry-run`
Expected: output contains `## What's New (since last sync, 2026-04-19)` with the two CAP entries

- [ ] **Step 3: Commit**

```bash
git add content/packs/cap/pack.yaml
git commit -m "feat: seed changelog entries in CAP pack"
```

---

### Task 8: Build verification and documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `TODO.md`

- [ ] **Step 1: Full build check**

Run: `go build ./...` and `go vet ./...`
Expected: success with no warnings

- [ ] **Step 2: Update CLAUDE.md**

In `CLAUDE.md`, under the `### Content Layer System` section, after the paragraph about `LoadPacks()`, add a brief note about the changelog lifecycle:

```markdown
**What's New Injection:** Each pack may include a `changelog` list in `pack.yaml` with human-curated change notes. During `sync`, these entries are collected into `~/.cache/sap-devs/sync-changelog.json`. On the next `inject`, the entries are rendered as a `## What's New` block at the top of the injected context, then the file is deleted (one-shot). See `internal/sync/changelog.go` for the file lifecycle functions.
```

- [ ] **Step 3: Update TODO.md**

Mark the "What's changed since last sync" item as done in `TODO.md`.

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md TODO.md
git commit -m "docs: document What's New injection lifecycle"
```
