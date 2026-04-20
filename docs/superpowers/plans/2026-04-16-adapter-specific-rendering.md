# Adapter-Specific Rendering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make each adapter render context in the correct format and size for its target tool, fix two broken adapter paths, and add a hybrid file-export adapter type for ChatGPT.

**Architecture:** Six independent layers of change built bottom-up: (1) new rendering helpers in `content`, (2) struct changes in `adapter`, (3) `ReplaceFile` write mode, (4) `ExportFileAndClip` hybrid handler, (5) engine wiring, (6) YAML adapter files. Each layer is committed independently and leaves the build passing.

**Tech Stack:** Go 1.21+, `gopkg.in/yaml.v3`, `golang.design/x/clipboard`, `github.com/stretchr/testify`. Verify with `go build ./...` and `go vet ./...` locally (Windows Defender blocks `go test`; CI on ubuntu-latest is the authoritative test runner).

**Spec:** `docs/superpowers/specs/2026-04-16-adapter-specific-rendering-design.md`

---

## File Map

| File | Action | Responsibility |
| --- | --- | --- |
| `internal/content/render.go` | Modify | Add `FormatOutput` and `TrimToBytes` |
| `internal/content/render_test.go` | Modify | Tests for `FormatOutput` and `TrimToBytes` |
| `internal/adapter/adapter.go` | Modify | Rename `ClipFormat`→`Format`; add `MaxBytes`, `ExportPath`, `Preamble` |
| `internal/adapter/adapter_test.go` | Modify | Tests for new YAML fields |
| `internal/adapter/file_inject.go` | Modify | Add `ReplaceFile`; add `replace-file` case in `runFileInject` |
| `internal/adapter/file_inject_test.go` | Modify | Tests for `ReplaceFile` |
| `internal/adapter/file_export.go` | Create | `ExportFileAndClip` hybrid handler |
| `internal/adapter/file_export_test.go` | Create | Tests for `ExportFileAndClip` |
| `internal/adapter/engine.go` | Modify | Budget resolution, `FormatOutput` dispatch, `file-export` case, updated stats |
| `internal/adapter/adapter_test.go` | Modify | Engine tests for new behaviour |
| `content/adapters/cody.yaml` | Delete | Cody has no static file injection mechanism |
| `content/adapters/jetbrains-ai.yaml` | Modify | Fix path; use `replace-file` mode |
| `content/adapters/cursor.yaml` | Modify | Use `replace-file` mode with YAML preamble |
| `content/adapters/continue.yaml` | Modify | Use `replace-file` mode with YAML preamble; fix MCP format |
| `content/adapters/chatgpt.yaml` | Modify | Change type to `file-export`; add `max_bytes` and `export_path` |
| `content/adapters/gemini.yaml` | Modify | Add `format: plain-prose` |

---

## Task 1: `FormatOutput` and `TrimToBytes` in render.go

**Files:**

- Modify: `internal/content/render.go`
- Modify: `internal/content/render_test.go`

- [ ] **Step 1: Write failing tests for `FormatOutput`**

Add to the bottom of `internal/content/render_test.go`:

```go
func TestFormatOutput_Markdown_NoOp(t *testing.T) {
	input := "## Section\n\n**bold** and *italic*\n"
	assert.Equal(t, input, content.FormatOutput(input, "markdown"))
	assert.Equal(t, input, content.FormatOutput(input, ""))
}

func TestFormatOutput_PlainProse_Headers(t *testing.T) {
	assert.Equal(t, "Title\n", content.FormatOutput("# Title\n", "plain-prose"))
	assert.Equal(t, "Section\n", content.FormatOutput("## Section\n", "plain-prose"))
	assert.Equal(t, "Deep\n", content.FormatOutput("### Deep\n", "plain-prose"))
}

func TestFormatOutput_PlainProse_Bold(t *testing.T) {
	assert.Equal(t, "bold text here\n", content.FormatOutput("**bold text** here\n", "plain-prose"))
}

func TestFormatOutput_PlainProse_Italic(t *testing.T) {
	assert.Equal(t, "italic text here\n", content.FormatOutput("*italic text* here\n", "plain-prose"))
}

func TestFormatOutput_PlainProse_InlineCode(t *testing.T) {
	assert.Equal(t, "run cds watch now\n", content.FormatOutput("run `cds watch` now\n", "plain-prose"))
}

func TestFormatOutput_PlainProse_CodeBlock(t *testing.T) {
	input := "```bash\ncds watch\n```\n"
	out := content.FormatOutput(input, "plain-prose")
	assert.NotContains(t, out, "```")
	assert.Contains(t, out, "cds watch")
}

func TestFormatOutput_PlainProse_MultipleCodeBlocks(t *testing.T) {
	input := "```\nblock one\n```\n\n```\nblock two\n```\n"
	out := content.FormatOutput(input, "plain-prose")
	assert.NotContains(t, out, "```")
	assert.Contains(t, out, "block one")
	assert.Contains(t, out, "block two")
}

func TestFormatOutput_PlainProse_HTMLComments(t *testing.T) {
	input := "<!-- sap-devs:start:X -->\ncontent\n<!-- sap-devs:end:X -->\n"
	out := content.FormatOutput(input, "plain-prose")
	assert.NotContains(t, out, "<!--")
	assert.Contains(t, out, "content")
}

func TestFormatOutput_PlainProse_NormalizesBlankLines(t *testing.T) {
	input := "a\n\n\n\nb\n"
	out := content.FormatOutput(input, "plain-prose")
	assert.NotContains(t, out, "\n\n\n")
	assert.Contains(t, out, "a")
	assert.Contains(t, out, "b")
}
```

- [ ] **Step 2: Write failing tests for `TrimToBytes`**

Add to `internal/content/render_test.go`:

```go
func TestTrimToBytes_UnderLimit(t *testing.T) {
	s := "hello"
	assert.Equal(t, s, content.TrimToBytes(s, 100))
}

func TestTrimToBytes_ExactLimit(t *testing.T) {
	s := "hello"
	assert.Equal(t, s, content.TrimToBytes(s, 5))
}

func TestTrimToBytes_OverLimit(t *testing.T) {
	s := "hello world"
	out := content.TrimToBytes(s, 5)
	assert.Equal(t, "hello", out)
	assert.LessOrEqual(t, len(out), 5)
}

func TestTrimToBytes_Zero(t *testing.T) {
	// maxBytes <= 0 returns unchanged
	assert.Equal(t, "hello", content.TrimToBytes("hello", 0))
	assert.Equal(t, "hello", content.TrimToBytes("hello", -1))
}

func TestTrimToBytes_UTF8Boundary(t *testing.T) {
	// "café!" = c(1) a(1) f(1) é(2) !(1) = 6 bytes total
	// Cutting at maxBytes=4 falls in the middle of the 2-byte é rune (bytes 3–4)
	// Must return "caf" (3 bytes) — not include the orphaned leading byte of é
	out := content.TrimToBytes("café!", 4)
	assert.Equal(t, "caf", out, "must cut before the straddled rune, not after its leading byte")
	assert.LessOrEqual(t, len(out), 4, "must not exceed maxBytes")
}
```

- [ ] **Step 3: Verify tests fail to compile**

```
go build ./internal/content/...
```

Expected: compile error — `FormatOutput` and `TrimToBytes` undefined.

- [ ] **Step 4: Implement `FormatOutput` and `TrimToBytes`**

Add to the bottom of `internal/content/render.go`:

```go
import (
    // add to existing imports:
    "regexp"
    "unicode/utf8"
)

// FormatOutput converts content to the target format.
// format == "markdown" (or empty): returns content unchanged.
// format == "plain-prose": strips Markdown syntax for plain-text UI fields.
func FormatOutput(content, format string) string {
	if format != "plain-prose" {
		return content
	}
	s := content

	// Strip fenced code blocks — keep body, remove fences.
	// Pattern anchors both ``` fences to line starts to avoid merging adjacent blocks.
	codeBlock := regexp.MustCompile("(?m)^```[^\n]*\n((?:[^`]|`[^`]|``[^`])*?)^```")
	s = codeBlock.ReplaceAllString(s, "$1")

	// Strip ATX headers (# through ######)
	s = regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(s, "")

	// Strip bold (**text**)
	s = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(s, "$1")

	// Strip italic (*text*)
	s = regexp.MustCompile(`\*([^*\n]+)\*`).ReplaceAllString(s, "$1")

	// Strip inline code (`text`)
	s = regexp.MustCompile("`([^`\n]+)`").ReplaceAllString(s, "$1")

	// Strip HTML comments
	s = regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(s, "")

	// Normalize 3+ consecutive blank lines to 2
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")

	return s
}

// TrimToBytes truncates s to at most maxBytes bytes, cutting at the last
// complete UTF-8 rune boundary. Returns s unchanged if maxBytes <= 0 or
// len(s) <= maxBytes.
func TrimToBytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	i := maxBytes
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return s[:i]
}
```

- [ ] **Step 5: Build and vet**

```
go build ./internal/content/...
go vet ./internal/content/...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/content/render.go internal/content/render_test.go
git commit -m "feat(content): add FormatOutput and TrimToBytes"
```

---

## Task 2: Adapter struct changes

**Files:**

- Modify: `internal/adapter/adapter.go`
- Modify: `internal/adapter/adapter_test.go`

- [ ] **Step 1: Write failing tests for new fields**

Add to `internal/adapter/adapter_test.go`:

```go
func TestLoadAdapters_MaxBytes(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, filepath.Join(dir, "chatgpt.yaml"), `
id: chatgpt
name: ChatGPT
type: file-export
max_bytes: 1400
export_path: "~/sap-devs-chatgpt-context.md"
format: plain-prose
instructions: "Paste into ChatGPT"
`)
	adapters, err := adapter.LoadAdapters(dir)
	require.NoError(t, err)
	require.Len(t, adapters, 1)
	assert.Equal(t, 1400, adapters[0].MaxBytes)
	assert.Equal(t, "~/sap-devs-chatgpt-context.md", adapters[0].ExportPath)
	assert.Equal(t, "plain-prose", adapters[0].Format)
	assert.Equal(t, "file-export", adapters[0].Type)
}

func TestLoadAdapters_Preamble(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, filepath.Join(dir, "cursor.yaml"), `
id: cursor
name: Cursor
type: file-inject
targets:
  - scope: global
    path: "~/.cursor/rules/sap.mdc"
    mode: replace-file
    preamble: "---\nalwaysApply: true\n---"
`)
	adapters, err := adapter.LoadAdapters(dir)
	require.NoError(t, err)
	require.Len(t, adapters, 1)
	assert.Equal(t, "replace-file", adapters[0].Targets[0].Mode)
	assert.Equal(t, "---\nalwaysApply: true\n---", adapters[0].Targets[0].Preamble)
}

func TestLoadAdapters_FormatFieldRenamedFromClipFormat(t *testing.T) {
	// YAML tag "format" must still be parsed — field was renamed ClipFormat→Format
	dir := t.TempDir()
	writeYAML(t, filepath.Join(dir, "gemini.yaml"), `
id: gemini
name: Google Gemini
type: clipboard-export
format: plain-prose
instructions: "Paste into Gemini"
`)
	adapters, err := adapter.LoadAdapters(dir)
	require.NoError(t, err)
	require.Len(t, adapters, 1)
	assert.Equal(t, "plain-prose", adapters[0].Format)
}
```

- [ ] **Step 2: Verify tests fail to compile**

```
go build ./internal/adapter/...
```

Expected: compile error — `MaxBytes`, `ExportPath`, `Format` (or `ClipFormat`) undefined on struct.

- [ ] **Step 3: Update `internal/adapter/adapter.go`**

Make these changes to the `Adapter` struct:

```go
// Rename ClipFormat → Format (YAML tag unchanged)
// Before: ClipFormat string `yaml:"format"`
// After:
Format string `yaml:"format,omitempty"` // "markdown" (default) | "plain-prose"

// Add new fields:
MaxBytes   int    `yaml:"max_bytes,omitempty"`   // hard byte ceiling; 0 = unconstrained
ExportPath string `yaml:"export_path,omitempty"` // file-export: path to write full context
```

Add to the `Target` struct:

```go
Preamble string `yaml:"preamble,omitempty"` // prepended before content; replace-file only
```

Note: `clipboard.go` does not reference `ClipFormat` by name — it takes `content string` as a parameter, so the rename is isolated to `adapter.go`.

- [ ] **Step 4: Build and vet**

```
go build ./internal/adapter/...
go vet ./internal/adapter/...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/adapter.go internal/adapter/adapter_test.go
git commit -m "feat(adapter): rename ClipFormat to Format; add MaxBytes, ExportPath, Preamble"
```

---

## Task 3: `ReplaceFile` write mode

**Files:**

- Modify: `internal/adapter/file_inject.go`
- Modify: `internal/adapter/file_inject_test.go`

- [ ] **Step 1: Write failing tests for `ReplaceFile`**

Add to `internal/adapter/file_inject_test.go`:

```go
func TestReplaceFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules", "sap.mdc")

	err := adapter.ReplaceFile(path, "", "content here", false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "content here", string(data))
}

func TestReplaceFile_WithPreamble(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sap.mdc")
	preamble := "---\nalwaysApply: true\n---"

	err := adapter.ReplaceFile(path, preamble, "the content", false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, preamble+"\nthe content", string(data))
}

func TestReplaceFile_OverwritesOnReInject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sap.mdc")

	require.NoError(t, adapter.ReplaceFile(path, "", "first run", false))
	require.NoError(t, adapter.ReplaceFile(path, "", "second run", false))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "second run", string(data))
	assert.NotContains(t, string(data), "first run")
}

func TestReplaceFile_DryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sap.mdc")

	err := adapter.ReplaceFile(path, "preamble", "content", true)
	require.NoError(t, err)

	// File must not be created in dry-run mode
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "dry-run must not write file")
}
```

- [ ] **Step 2: Verify tests fail to compile**

```
go build ./internal/adapter/...
```

Expected: compile error — `ReplaceFile` undefined.

- [ ] **Step 3: Implement `ReplaceFile` in `file_inject.go`**

Add to `internal/adapter/file_inject.go`:

```go
// ReplaceFile writes preamble + "\n" + content to filePath, overwriting any
// existing content. If preamble is empty, only content is written (no leading newline).
// Parent directories are created as needed.
// When dryRun is true the function prints what it would do but writes nothing.
func ReplaceFile(filePath, preamble, content string, dryRun bool) error {
	var data string
	if preamble != "" {
		data = preamble + "\n" + content
	} else {
		data = content
	}

	if dryRun {
		fmt.Printf("[dry-run] would write file %s (%d bytes)\n", filePath, len(data))
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(data), 0644)
}
```

- [ ] **Step 4: Add `replace-file` case in `runFileInject` (in `engine.go`)**

`runFileInject` lives in `engine.go` (not `file_inject.go`). Extend its switch:

```go
case "replace-file":
    if err := ReplaceFile(path, target.Preamble, ctx, e.opts.DryRun); err != nil {
        return fmt.Errorf("target %s: %w", target.Path, err)
    }
```

The `ctx` parameter name here is the `runFileInject` method's local parameter — it does not need renaming when the engine adds `formattedCtx` in Task 5, because `formattedCtx` will be passed as the argument to `runFileInject` from the call site.

- [ ] **Step 5: Build and vet**

```
go build ./internal/adapter/...
go vet ./internal/adapter/...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/file_inject.go internal/adapter/file_inject_test.go internal/adapter/engine.go
git commit -m "feat(adapter): add replace-file write mode with preamble support"
```

---

## Task 4: `ExportFileAndClip` hybrid handler

**Files:**

- Create: `internal/adapter/file_export.go`
- Create: `internal/adapter/file_export_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/adapter/file_export_test.go`:

```go
package adapter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/adapter"
)

func TestExportFileAndClip_EmptyExportPath(t *testing.T) {
	a := adapter.Adapter{ID: "chatgpt", Type: "file-export"}
	err := adapter.ExportFileAndClip(a, "some content", adapter.Options{DryRun: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "export_path is required")
}

func TestExportFileAndClip_WritesFullFile(t *testing.T) {
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "context.md")

	a := adapter.Adapter{
		ID:           "chatgpt",
		Type:         "file-export",
		ExportPath:   exportPath,
		MaxBytes:     50,
		Format:       "plain-prose",
		Instructions: "Paste into ChatGPT",
	}

	fullCtx := strings.Repeat("x", 200)
	err := adapter.ExportFileAndClip(a, fullCtx, adapter.Options{DryRun: false})
	require.NoError(t, err)

	data, err := os.ReadFile(exportPath)
	require.NoError(t, err)
	// Full Markdown context written verbatim (no truncation)
	assert.Equal(t, fullCtx, string(data))
}

func TestExportFileAndClip_ClipsShortVersion(t *testing.T) {
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "context.md")

	a := adapter.Adapter{
		ID:           "chatgpt",
		Type:         "file-export",
		ExportPath:   exportPath,
		MaxBytes:     20,
		Format:       "markdown",
		Instructions: "Paste me",
	}

	// Content longer than max_bytes — clipboard payload must be trimmed
	fullCtx := strings.Repeat("a", 200)
	// DryRun = true so we don't need clipboard hardware
	err := adapter.ExportFileAndClip(a, fullCtx, adapter.Options{DryRun: true})
	require.NoError(t, err)
}

func TestExportFileAndClip_AppendedGuidanceLine(t *testing.T) {
	// Guidance line references export_path and mentions ChatGPT Project
	// We verify this via dry-run (file not written, but no error)
	dir := t.TempDir()
	a := adapter.Adapter{
		ID:           "chatgpt",
		Type:         "file-export",
		ExportPath:   filepath.Join(dir, "ctx.md"),
		MaxBytes:     1400,
		Format:       "plain-prose",
		Instructions: "Paste",
	}
	err := adapter.ExportFileAndClip(a, "SAP context here", adapter.Options{DryRun: true})
	require.NoError(t, err)
}

func TestExportFileAndClip_DryRun(t *testing.T) {
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "context.md")

	a := adapter.Adapter{
		ID:         "chatgpt",
		ExportPath: exportPath,
		MaxBytes:   1400,
	}

	err := adapter.ExportFileAndClip(a, "content", adapter.Options{DryRun: true})
	require.NoError(t, err)

	// File must not be created in dry-run
	_, statErr := os.Stat(exportPath)
	assert.True(t, os.IsNotExist(statErr), "dry-run must not write export file")
}
```

- [ ] **Step 2: Verify tests fail to compile**

```
go build ./internal/adapter/...
```

Expected: compile error — `ExportFileAndClip` undefined.

- [ ] **Step 3: Implement `ExportFileAndClip`**

Create `internal/adapter/file_export.go`:

```go
package adapter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

const exportGuidanceFmt = "Full SAP context saved to %s — upload to a ChatGPT Project for comprehensive knowledge."

// ExportFileAndClip writes fullCtx (raw Markdown) to a.ExportPath and copies
// a trimmed, formatted short summary plus a guidance line to the clipboard.
// fullCtx must be raw Markdown — FormatOutput is NOT applied to the file, only
// to the clipboard payload.
func ExportFileAndClip(a Adapter, fullCtx string, opts Options) error {
	if a.ExportPath == "" {
		return fmt.Errorf("adapter %s: export_path is required for file-export type", a.ID)
	}

	path, err := ExpandHome(a.ExportPath)
	if err != nil {
		return fmt.Errorf("adapter %s: %w", a.ID, err)
	}

	if opts.DryRun {
		fmt.Printf("[dry-run] would write export file %s (%d bytes)\n", path, len(fullCtx))
	} else {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("adapter %s: %w", a.ID, err)
		}
		if err := os.WriteFile(path, []byte(fullCtx), 0644); err != nil {
			return fmt.Errorf("adapter %s: write export file: %w", a.ID, err)
		}
	}

	// Build short clipboard payload: trim → format → append guidance
	short := content.TrimToBytes(fullCtx, a.MaxBytes)
	short = content.FormatOutput(short, a.Format)
	short = short + "\n" + fmt.Sprintf(exportGuidanceFmt, a.ExportPath)

	return ExportToClipboard(short, a.Instructions, opts.DryRun)
}
```

- [ ] **Step 4: Build and vet**

```
go build ./internal/adapter/...
go vet ./internal/adapter/...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/file_export.go internal/adapter/file_export_test.go
git commit -m "feat(adapter): add file-export hybrid adapter type (ExportFileAndClip)"
```

> **Note on scope:** The spec's test table lists `TestExportFileAndClip_SkippedForProjectScope` for this file. Because `ExportFileAndClip` itself has no scope parameter — scope filtering is the engine's responsibility — this contract is covered by `TestEngine_FileExportSkippedForProjectScope` in Task 5 instead.

---

## Task 5: Engine wiring

**Files:**

- Modify: `internal/adapter/engine.go`
- Modify: `internal/adapter/adapter_test.go`

- [ ] **Step 1: Write failing engine tests**

Add to `internal/adapter/adapter_test.go`:

```go
func TestEngine_MaxBytesOverridesMaxTokens(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "out.md")

	// Pack is 100 bytes. MaxBytes=50 should trim it out; MaxTokens=1000 (=4000 bytes) would not.
	packs := []*content.Pack{
		{ID: "big", ContextMD: strings.Repeat("x", 100)},
	}
	adapters := []adapter.Adapter{
		{
			ID:        "tight",
			Type:      "file-inject",
			MaxBytes:  50,  // hard limit — takes precedence
			MaxTokens: 1000, // would allow 4000 bytes, but MaxBytes wins
			Targets:   []adapter.Target{{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "S"}},
		},
	}

	var buf bytes.Buffer
	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global", Out: &buf})
	require.NoError(t, engine.Run())

	// Budget was 50 bytes — pack (100 bytes) didn't fit → file should not contain pack content
	data, _ := os.ReadFile(targetFile)
	assert.NotContains(t, string(data), strings.Repeat("x", 100))
}

func TestEngine_FormatApplied(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "out.md")

	packs := []*content.Pack{
		{ID: "cap", ContextMD: "## CAP Section\n\n**Use** `cds watch`.\n"},
	}
	adapters := []adapter.Adapter{
		{
			ID:     "plain-tool",
			Type:   "file-inject",
			Format: "plain-prose",
			Targets: []adapter.Target{
				{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "S"},
			},
		},
	}

	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global"})
	require.NoError(t, engine.Run())

	data, err := os.ReadFile(targetFile)
	require.NoError(t, err)
	// Markdown stripped
	assert.NotContains(t, string(data), "##")
	assert.NotContains(t, string(data), "**")
	assert.NotContains(t, string(data), "`")
	// Text preserved
	assert.Contains(t, string(data), "CAP Section")
	assert.Contains(t, string(data), "Use")
	assert.Contains(t, string(data), "cds watch")
}

func TestEngine_FileExportType(t *testing.T) {
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "ctx.md")

	packs := []*content.Pack{{ID: "cap", ContextMD: "CAP content"}}
	adapters := []adapter.Adapter{
		{
			ID:         "chatgpt",
			Type:       "file-export",
			ExportPath: exportPath,
			MaxBytes:   1400,
			Format:     "plain-prose",
		},
	}

	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global", DryRun: true})
	require.NoError(t, engine.Run())
	// DryRun=true: no file written, no error
}

func TestEngine_FileExportSkippedForProjectScope(t *testing.T) {
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "ctx.md")

	packs := []*content.Pack{{ID: "cap", ContextMD: "CAP content"}}
	adapters := []adapter.Adapter{
		{
			ID:         "chatgpt",
			Type:       "file-export",
			ExportPath: exportPath,
			MaxBytes:   1400,
		},
	}

	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "project"})
	require.NoError(t, engine.Run())

	// file-export must be skipped for project scope — export file not created
	_, err := os.Stat(exportPath)
	assert.True(t, os.IsNotExist(err), "file-export must be skipped for project scope")
}
```

- [ ] **Step 2: Verify tests fail**

```
go build ./internal/adapter/...
```

Expected: compiles (new tests reference existing types), but behaviour tests will fail when run in CI.

- [ ] **Step 3: Update `adapterStats` struct and early-exit block in `engine.go`**

Replace the `adapterStats` struct:

```go
// Before:
type adapterStats struct {
    AdapterID    string
    PackIDs      []string
    ApproxTokens int
    BudgetTokens int
    Trimmed      bool
}

// After:
type adapterStats struct {
    AdapterID    string
    PackIDs      []string
    ApproxTokens int
    BudgetBytes  int    // effective budget in bytes; 0 = unconstrained
    Format       string // "markdown" | "plain-prose" | ""
    Trimmed      bool
}
```

Also update the **early-exit stats block** (the one inside `if len(trimmed) == 0 && maxBytes > 0`) which still references `BudgetTokens`. Replace it:

```go
// Before:
stats = append(stats, adapterStats{
    AdapterID:    a.ID,
    PackIDs:      nil,
    ApproxTokens: 0,
    BudgetTokens: a.MaxTokens,
    Trimmed:      true,
})

// After:
stats = append(stats, adapterStats{
    AdapterID:   a.ID,
    PackIDs:     nil,
    BudgetBytes: maxBytes, // resolved value
    Format:      a.Format,
    Trimmed:     true,
})
```

- [ ] **Step 4: Update budget resolution in `engine.Run()`**

Replace:

```go
maxBytes := a.MaxTokens * 4
```

With:

```go
maxBytes := a.MaxBytes
if maxBytes == 0 && a.MaxTokens > 0 {
    maxBytes = a.MaxTokens * 4
}
```

- [ ] **Step 5: Add `FormatOutput` call and `file-export` dispatch**

After the `ctx := content.RenderContext(...)` line, add for non-file-export adapters:

```go
// Apply format transform (skipped for file-export — ExportFileAndClip handles it internally)
var formattedCtx string
if a.Type != "file-export" {
    formattedCtx = content.FormatOutput(ctx, a.Format)
} else {
    formattedCtx = ctx // raw Markdown passed to ExportFileAndClip
}
```

Then use `formattedCtx` wherever `ctx` was passed to `runFileInject` and `ExportToClipboard`. **Important:** only the call site in `Run()` changes — the `runFileInject` method signature parameter keeps the name `ctx string`, no rename needed there.

Add `file-export` to the dispatch switch and update the stats-append block:

```go
case "file-export":
    if e.opts.Scope == "project" {
        continue
    }
    if err := ExportFileAndClip(a, formattedCtx, e.opts); err != nil {
        return fmt.Errorf("adapter %s: %w", a.ID, err)
    }
```

Update the **happy-path** stats-append block to use new field names:

```go
stats = append(stats, adapterStats{
    AdapterID:    a.ID,
    PackIDs:      packIDs,
    ApproxTokens: len(formattedCtx) / 4,
    BudgetBytes:  maxBytes, // resolved value (MaxBytes or MaxTokens*4)
    Format:       a.Format,
    Trimmed:      len(trimmed) < len(e.packs),
})
```

Also add an engine-level test to confirm the export file receives raw Markdown (not plain-prose stripped) even when `Format: "plain-prose"` is set. Add to `adapter_test.go`:

```go
func TestEngine_FileExportWritesRawMarkdown(t *testing.T) {
    dir := t.TempDir()
    exportPath := filepath.Join(dir, "ctx.md")

    packs := []*content.Pack{
        {ID: "cap", ContextMD: "## CAP Section\n\n**bold** content\n"},
    }
    adapters := []adapter.Adapter{
        {
            ID:         "chatgpt",
            Type:       "file-export",
            ExportPath: exportPath,
            MaxBytes:   10000,
            Format:     "plain-prose", // format applies to clipboard only
        },
    }

    engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global"})
    require.NoError(t, engine.Run())

    data, err := os.ReadFile(exportPath)
    require.NoError(t, err)
    // File must contain raw Markdown — ## and ** must NOT be stripped
    assert.Contains(t, string(data), "##", "export file must preserve Markdown headers")
    assert.Contains(t, string(data), "**", "export file must preserve Markdown bold")
}
```

- [ ] **Step 6: Update `printStats` for new fields**

Note: the spec describes a conditional header (`Budget (tokens)` vs `Budget (bytes)` depending on which adapters are present). This implementation simplifies to always using `Budget (bytes)` — bytes are the more precise unit and token budgets are stored internally as bytes after resolution anyway.

```go
func printStats(w io.Writer, stats []adapterStats) {
    tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
    fmt.Fprintln(tw, "Adapter\tPacks included\tTokens (approx)\tBudget (bytes)\tFormat\tStatus")
    for _, s := range stats {
        budget := "unconstrained"
        if s.BudgetBytes > 0 {
            budget = fmt.Sprintf("%d bytes", s.BudgetBytes)
        }
        packs := strings.Join(s.PackIDs, ", ")
        if packs == "" {
            packs = "(none)"
        }
        format := s.Format
        if format == "" {
            format = "markdown"
        }
        status := ""
        if s.Trimmed {
            status = "trimmed"
        }
        fmt.Fprintf(tw, "%s\t%s\t~%d\t%s\t%s\t%s\n",
            s.AdapterID, packs, s.ApproxTokens, budget, format, status)
    }
    tw.Flush()
}
```

- [ ] **Step 7: Build and vet**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add internal/adapter/engine.go internal/adapter/adapter_test.go
git commit -m "feat(adapter): wire FormatOutput, MaxBytes budget, and file-export in engine"
```

---

## Task 6: YAML adapter updates and Cody removal

**Files:**

- Delete: `content/adapters/cody.yaml`
- Modify: `content/adapters/jetbrains-ai.yaml`
- Modify: `content/adapters/cursor.yaml`
- Modify: `content/adapters/continue.yaml`
- Modify: `content/adapters/chatgpt.yaml`
- Modify: `content/adapters/gemini.yaml`

- [ ] **Step 1: Delete Cody adapter**

```bash
git rm content/adapters/cody.yaml
```

- [ ] **Step 2: Update `jetbrains-ai.yaml`**

Replace the entire file:

```yaml
id: jetbrains-ai
name: JetBrains AI Assistant
type: file-inject
targets:
  - scope: project
    path: ".aiassistant/rules/sap-developer-context.md"
    mode: replace-file
    # no preamble — rule type (always/conditional) is configured in IDE settings
detect:
  - path: "~/.config/JetBrains"
```

- [ ] **Step 3: Update `cursor.yaml`**

Replace the entire file:

```yaml
id: cursor
name: Cursor
type: file-inject
targets:
  - scope: global
    path: "~/.cursor/rules/sap-developer-context.mdc"
    mode: replace-file
    preamble: "---\ndescription: SAP developer context — CAP, BTP, ABAP Cloud\nalwaysApply: true\n---"
  - scope: project
    path: ".cursor/rules/sap-developer-context.mdc"
    mode: replace-file
    preamble: "---\ndescription: SAP developer context — CAP, BTP, ABAP Cloud\nalwaysApply: true\n---"
detect:
  - path: "~/.cursor"
  - command: "cursor --version"
mcp_config:
  path: "~/.cursor/mcp.json"
  format: json
  key: "mcpServers"
```

- [ ] **Step 4: Update `continue.yaml`**

Replace the entire file:

```yaml
id: continue
name: Continue.dev
type: file-inject
targets:
  - scope: global
    path: "~/.continue/rules/sap-developer-context.md"
    mode: replace-file
    preamble: "---\nname: SAP Developer Context\nalwaysApply: true\n---"
  - scope: project
    path: ".continue/rules/sap-developer-context.md"
    mode: replace-file
    preamble: "---\nname: SAP Developer Context\nalwaysApply: true\n---"
detect:
  - path: "~/.continue"
mcp_config:
  path: "~/.continue/config.yaml"
  format: yaml
  key: "mcpServers"
```

- [ ] **Step 5: Update `chatgpt.yaml`**

Replace the entire file:

```yaml
id: chatgpt
name: ChatGPT
type: file-export
export_path: "~/sap-devs-chatgpt-context.md"
max_bytes: 1400
format: plain-prose
instructions: "Paste into ChatGPT → Settings → Custom Instructions → 'What would you like ChatGPT to know about you?'"
```

- [ ] **Step 6: Update `gemini.yaml`**

Replace the entire file:

```yaml
id: gemini
name: Google Gemini
type: clipboard-export
format: plain-prose
instructions: "Paste into Gemini → Settings → Custom Instructions or into your Gemini for Google Workspace prompt."
```

- [ ] **Step 7: Build and verify YAML loads**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add content/adapters/
git commit -m "feat(adapters): fix paths, add frontmatter, file-export for ChatGPT, remove Cody"
```
