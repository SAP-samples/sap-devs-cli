# Scratch/Session Context Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users append ephemeral working notes that get injected into AI tools as `## Current Context`, so agents know what the developer is currently working on.

**Architecture:** New `internal/scratch` package handles YAML I/O for `.sap-devs/scratch.yaml`. New `cmd/context.go` exposes add/list/clear subcommands. `RenderContext` in `render.go` emits a `## Current Context` section when `DynamicContext.ScratchNotes` is populated. Scratch loading happens in `cmd/inject.go` only when `--project` scope is active.

**Tech Stack:** Go, cobra, gopkg.in/yaml.v3, testify

**Spec:** `docs/superpowers/specs/2026-04-19-scratch-session-context-design.md`

---

### Task 1: Create `internal/scratch` package with tests

**Files:**
- Create: `internal/scratch/scratch.go`
- Create: `internal/scratch/scratch_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/scratch/scratch_test.go`:

```go
package scratch_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/scratch"
)

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	notes, err := scratch.Load(dir)
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestLoad_ExistingNotes(t *testing.T) {
	dir := t.TempDir()
	sapDir := filepath.Join(dir, ".sap-devs")
	require.NoError(t, os.MkdirAll(sapDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sapDir, "scratch.yaml"),
		[]byte("notes:\n  - \"note one\"\n  - \"note two\"\n"), 0o644))

	notes, err := scratch.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"note one", "note two"}, notes)
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	sapDir := filepath.Join(dir, ".sap-devs")
	require.NoError(t, os.MkdirAll(sapDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sapDir, "scratch.yaml"), []byte(""), 0o644))

	notes, err := scratch.Load(dir)
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestAdd_CreatesDirectoryAndFile(t *testing.T) {
	dir := t.TempDir()
	err := scratch.Add(dir, "first note")
	require.NoError(t, err)

	notes, err := scratch.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"first note"}, notes)
}

func TestAdd_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, scratch.Add(dir, "note one"))
	require.NoError(t, scratch.Add(dir, "note two"))

	notes, err := scratch.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"note one", "note two"}, notes)
}

func TestAdd_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, scratch.Add(dir, "  trimmed note  "))

	notes, err := scratch.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"trimmed note"}, notes)
}

func TestAdd_RejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	assert.Error(t, scratch.Add(dir, ""))
	assert.Error(t, scratch.Add(dir, "   "))
}

func TestClear_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, scratch.Add(dir, "note"))
	require.NoError(t, scratch.Clear(dir))

	notes, err := scratch.Load(dir)
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestClear_NoErrorWhenMissing(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, scratch.Clear(dir))
}

func TestHasNotes_TrueWhenPresent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, scratch.Add(dir, "note"))
	assert.True(t, scratch.HasNotes(dir))
}

func TestHasNotes_FalseWhenMissing(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, scratch.HasNotes(dir))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scratch/...`
Expected: FAIL — package does not exist yet

- [ ] **Step 3: Write minimal implementation**

Create `internal/scratch/scratch.go`:

```go
package scratch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const scratchDir = ".sap-devs"
const scratchFile = "scratch.yaml"

type fileData struct {
	Notes []string `yaml:"notes"`
}

func scratchPath(dir string) string {
	return filepath.Join(dir, scratchDir, scratchFile)
}

func Load(dir string) ([]string, error) {
	data, err := os.ReadFile(scratchPath(dir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var f fileData
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", scratchFile, err)
	}
	return f.Notes, nil
}

func Add(dir, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return fmt.Errorf("note cannot be empty")
	}
	notes, err := Load(dir)
	if err != nil {
		return err
	}
	notes = append(notes, note)
	return write(dir, notes)
}

func Clear(dir string) error {
	err := os.Remove(scratchPath(dir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func HasNotes(dir string) bool {
	notes, err := Load(dir)
	return err == nil && len(notes) > 0
}

func write(dir string, notes []string) error {
	dirPath := filepath.Join(dir, scratchDir)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return err
	}
	f := fileData{Notes: notes}
	data, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(scratchPath(dir), data, 0o644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scratch/...`
Expected: PASS (all 10 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/scratch/scratch.go internal/scratch/scratch_test.go
git commit -m "feat: add internal/scratch package for ephemeral project notes"
```

---

### Task 2: Add i18n keys for context commands

**Files:**
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json`

- [ ] **Step 1: Add English keys**

Add before the closing `}` in `internal/i18n/catalogs/en.json`:

```json
  "context.short": "Manage ephemeral project context notes",
  "context.long": "Add, list, or clear scratch notes that are injected into AI tools\nwhen running 'sap-devs inject --project'. Notes are per-project,\nephemeral, and stored in .sap-devs/scratch.yaml.",
  "context.add.short": "Add a scratch note to project context",
  "context.add.done": "Added note to project context.",
  "context.add.empty": "Note cannot be empty.",
  "context.list.short": "List current scratch notes",
  "context.list.empty": "No scratch notes set. Use \"sap-devs context add\" to add one.",
  "context.list.header": "Current project context:",
  "context.clear.short": "Clear all scratch notes",
  "context.clear.done": "Cleared all scratch notes.",
  "context.clear.empty": "No scratch notes to clear."
```

- [ ] **Step 2: Add German keys**

Add before the closing `}` in `internal/i18n/catalogs/de.json`:

```json
  "context.short": "Ephemere Projektkontext-Notizen verwalten",
  "context.long": "Notizen hinzufügen, auflisten oder löschen, die beim Ausführen von\n'sap-devs inject --project' in KI-Tools eingefügt werden. Notizen sind\nprojektbezogen, kurzlebig und in .sap-devs/scratch.yaml gespeichert.",
  "context.add.short": "Notiz zum Projektkontext hinzufügen",
  "context.add.done": "Notiz zum Projektkontext hinzugefügt.",
  "context.add.empty": "Notiz darf nicht leer sein.",
  "context.list.short": "Aktuelle Notizen anzeigen",
  "context.list.empty": "Keine Notizen gesetzt. Verwende \"sap-devs context add\" zum Hinzufügen.",
  "context.list.header": "Aktueller Projektkontext:",
  "context.clear.short": "Alle Notizen löschen",
  "context.clear.done": "Alle Notizen gelöscht.",
  "context.clear.empty": "Keine Notizen zum Löschen vorhanden."
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Clean build (i18n keys are embedded at compile time)

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json
git commit -m "feat: add i18n keys for context command"
```

---

### Task 3: Create `cmd/context.go` command

**Files:**
- Create: `cmd/context.go`

- [ ] **Step 1: Create the cobra command file**

Create `cmd/context.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/scratch"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: i18n.T("en", "context.short"),
	Long:  i18n.T("en", "context.long"),
	RunE:  runContextList,
}

var contextAddCmd = &cobra.Command{
	Use:   "add <note>",
	Short: i18n.T("en", "context.add.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		lang := i18n.ActiveLang
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := scratch.Add(cwd, args[0]); err != nil {
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("%s", i18n.T(lang, "context.add.empty"))
			}
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "context.add.done"))
		return nil
	},
}

var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("en", "context.list.short"),
	RunE:  runContextList,
}

func runContextList(cmd *cobra.Command, args []string) error {
	lang := i18n.ActiveLang
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	notes, err := scratch.Load(cwd)
	if err != nil {
		return err
	}
	if len(notes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "context.list.empty"))
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "context.list.header"))
	for _, note := range notes {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", note)
	}
	return nil
}

var contextClearCmd = &cobra.Command{
	Use:   "clear",
	Short: i18n.T("en", "context.clear.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		lang := i18n.ActiveLang
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if !scratch.HasNotes(cwd) {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "context.clear.empty"))
			return nil
		}
		if err := scratch.Clear(cwd); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "context.clear.done"))
		return nil
	},
}

func init() {
	contextCmd.AddCommand(contextAddCmd)
	contextCmd.AddCommand(contextListCmd)
	contextCmd.AddCommand(contextClearCmd)
	rootCmd.AddCommand(contextCmd)
}
```

- [ ] **Step 2: Verify build compiles**

Run: `go build ./...`
Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add cmd/context.go
git commit -m "feat: add context add/list/clear commands for scratch notes"
```

---

### Task 4: Extend DynamicContext and render scratch notes

**Files:**
- Modify: `internal/content/dynamic.go`
- Modify: `internal/content/render.go`
- Modify: `internal/content/render_test.go`

- [ ] **Step 1: Write failing render test**

Add to end of `internal/content/render_test.go`:

```go
func TestRenderContext_ScratchNotes_RenderedAsCurrentContext(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "## CAP context."},
	}
	dyn := &content.DynamicContext{
		ScratchNotes: []string{"implementing draft for Books", "HANA only in dev space"},
	}
	out := content.RenderContext(packs, nil, dyn)
	assert.Contains(t, out, "## Current Context")
	assert.Contains(t, out, "- implementing draft for Books")
	assert.Contains(t, out, "- HANA only in dev space")
}

func TestRenderContext_ScratchNotes_BeforeRuntimeContext(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "## CAP context."},
	}
	now := time.Now()
	dyn := &content.DynamicContext{
		CLIVersion:   "1.0.0",
		LastSynced:   &now,
		ScratchNotes: []string{"working on auth"},
	}
	out := content.RenderContext(packs, nil, dyn)
	scratchIdx := strings.Index(out, "## Current Context")
	runtimeIdx := strings.Index(out, "## sap-devs Runtime Context")
	require.NotEqual(t, -1, scratchIdx, "scratch section must be present")
	require.NotEqual(t, -1, runtimeIdx, "runtime section must be present")
	assert.Less(t, scratchIdx, runtimeIdx, "scratch notes must precede runtime context")
}

func TestRenderContext_ScratchNotes_OmittedWhenEmpty(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "## CAP context."},
	}
	dyn := &content.DynamicContext{ScratchNotes: nil}
	out := content.RenderContext(packs, nil, dyn)
	assert.NotContains(t, out, "## Current Context")
}

func TestRenderContext_ScratchNotes_SanitizesNewlines(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "## CAP context."},
	}
	dyn := &content.DynamicContext{
		ScratchNotes: []string{"line one\nline two", "cr\ronly", "win\r\nstyle"},
	}
	out := content.RenderContext(packs, nil, dyn)
	assert.Contains(t, out, "- line one line two")
	assert.Contains(t, out, "- cr only")
	assert.Contains(t, out, "- win style")
}

func TestRenderContext_ScratchNotes_TruncatesLongNotes(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", ContextMD: "## CAP context."},
	}
	longNote := strings.Repeat("a", 600)
	dyn := &content.DynamicContext{
		ScratchNotes: []string{longNote},
	}
	out := content.RenderContext(packs, nil, dyn)
	assert.Contains(t, out, "...")
	assert.NotContains(t, out, strings.Repeat("a", 501))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/content/... -run "ScratchNotes"`
Expected: FAIL — `ScratchNotes` field does not exist

- [ ] **Step 3: Add `ScratchNotes` field to DynamicContext**

In `internal/content/dynamic.go`, add to the `DynamicContext` struct:

```go
ScratchNotes    []string
```

Add it after the `Commands` field (last field before the closing brace).

- [ ] **Step 4: Add scratch notes rendering to `RenderContext`**

In `internal/content/render.go`, add scratch notes rendering **before** the `if dynamic != nil {` block (around line 38). Insert between the profile line write and the `renderDynamic` call:

```go
	// Scratch notes — rendered before runtime context so they are the first thing agents read.
	if dynamic != nil && len(dynamic.ScratchNotes) > 0 {
		b.WriteString("## Current Context\n\n")
		for _, note := range dynamic.ScratchNotes {
			sanitized := strings.ReplaceAll(note, "\r\n", " ")
			sanitized = strings.ReplaceAll(sanitized, "\r", " ")
			sanitized = strings.ReplaceAll(sanitized, "\n", " ")
			if len(sanitized) > 500 {
				sanitized = TrimToBytes(sanitized, 500) + "..."
			}
			b.WriteString("- " + sanitized + "\n")
		}
		b.WriteString("\n")
	}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/content/... -run "ScratchNotes"`
Expected: PASS (all 5 new tests)

- [ ] **Step 6: Run full test suite**

Run: `go build ./... && go vet ./...`
Expected: Clean build and vet

- [ ] **Step 7: Commit**

```bash
git add internal/content/dynamic.go internal/content/render.go internal/content/render_test.go
git commit -m "feat: render scratch notes as Current Context section in injected output"
```

---

### Task 5: Wire scratch notes into inject flow

**Files:**
- Modify: `cmd/inject.go`

- [ ] **Step 1: Add scratch import**

Add to the import block in `cmd/inject.go`:

```go
"github.com/SAP-samples/sap-devs-cli/internal/scratch"
```

- [ ] **Step 2: Load scratch notes when project scope**

In `cmd/inject.go`, after the `dynCtx` is built and project health checks are attached (after the `}` closing the `if pc != nil && pc.Type != ""` block, around line 266), add:

```go
		// Load scratch notes for project-scope injection
		if injectProject {
			notes, _ := scratch.Load(cwd)
			dynCtx.ScratchNotes = notes
		}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add cmd/inject.go
git commit -m "feat: load scratch notes into inject when --project scope is active"
```

---

### Task 6: Update CLI manifest, docs, and TODO

**Files:**
- Modify: `content/packs/base/context.md`
- Modify: `TODO.md`
- Modify: `CLAUDE.md`
- Modify: `docs/developer-guide.md` (if it exists)

- [ ] **Step 1: Add context commands to CLI reference table**

In `content/packs/base/context.md`, add three rows to the CLI reference table before the closing `inject --status` row:

```markdown
| `sap-devs context add "note"` | Developer wants to tell the agent about current work | Appends note to project scratch; visible in next `inject --project` |
| `sap-devs context list` | Check what scratch notes are set for this project | Bullet list of current notes |
| `sap-devs context clear` | Done with current task, clear working notes | Removes all scratch notes |
```

- [ ] **Step 2: Mark TODO item as done**

In `TODO.md`, replace the "Scratch/session context" section heading and body with:

```markdown
### ~~Scratch/session context — `sap-devs context add`~~ ✅

Implemented: `context add/list/clear` commands with `.sap-devs/scratch.yaml` storage and `## Current Context` injection in project-scope output.
```

- [ ] **Step 3: Update CLAUDE.md and developer-guide.md**

Add `context` to the CLI Commands table in `CLAUDE.md`:

```markdown
| `context` | Manage ephemeral project context notes; `context add/list/clear` |
```

If `docs/developer-guide.md` exists and has a commands section, add the same entry there.

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: Clean build

- [ ] **Step 5: Commit**

```bash
git add content/packs/base/context.md TODO.md CLAUDE.md
git commit -m "docs: add context commands to CLI manifest, CLAUDE.md, and mark TODO as done"
```

---

### Task 7: Final verification

- [ ] **Step 1: Full build and vet**

Run: `go build ./... && go vet ./...`
Expected: Clean

- [ ] **Step 2: Verify context command works**

Run (with SAP_DEVS_DEV=1 for local content):
```bash
SAP_DEVS_DEV=1 go run . context add "testing scratch notes"
SAP_DEVS_DEV=1 go run . context list
SAP_DEVS_DEV=1 go run . context clear
SAP_DEVS_DEV=1 go run . context list
```

Expected:
1. "Added note to project context."
2. Shows the note in bullet list
3. "Cleared all scratch notes."
4. "No scratch notes set..."

- [ ] **Step 3: Verify inject integration**

Run:
```bash
SAP_DEVS_DEV=1 go run . context add "implementing draft enablement"
SAP_DEVS_DEV=1 go run . inject --project --dry-run
```

Expected: Dry-run output should mention `## Current Context` section with the note.

- [ ] **Step 4: Clean up scratch file**

Run:
```bash
SAP_DEVS_DEV=1 go run . context clear
```

- [ ] **Step 5: Final commit if any cleanup needed**

Only if changes were needed during verification.
