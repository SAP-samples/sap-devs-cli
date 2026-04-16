# Adapter-Specific Rendering Design

**Date:** 2026-04-16
**Status:** Approved for implementation

## Problem Statement

The `inject` pipeline currently renders a single Markdown string and writes it identically to every adapter target. This ignores three real differences between targets:

1. **Hard size constraints** — ChatGPT custom instructions have a 1,500-character hard limit per field; the current renderer has no way to enforce this.
2. **Format expectations** — Some targets (ChatGPT, Gemini) paste into plain-text UI fields where Markdown syntax appears literally; they benefit from a plain-prose pass. File-inject targets (Claude Code, Cursor, Copilot) all understand Markdown natively.
3. **Broken or wrong paths** — The Cody adapter writes to a path the tool never reads (Cody has no static file context mechanism). The JetBrains AI adapter writes to `.idea/ai-context.md` instead of the correct `.aiassistant/rules/` directory.

Additionally, Cursor and Continue.dev support YAML frontmatter in their rules files (`alwaysApply: true`, `name`) that causes context to be reliably applied in every session. Without it, Cursor uses "agent-decided" mode and may silently skip the SAP context.

---

## Scope

- Rename `ClipFormat` → `Format` on `Adapter`; add `MaxBytes`, `ExportPath` fields
- New `plain-prose` formatter in `internal/content/render.go`
- New `replace-file` target mode + `preamble` field on `Target`
- New `file-export` adapter type for the ChatGPT hybrid (writes local file + clips short summary)
- New `TrimToBytes` helper in `internal/content/render.go`
- Fix JetBrains AI path; delete Cody adapter; fix Continue.dev MCP config format
- Updated YAML for all 10 remaining adapters with correct formats and modes
- Tests for all new code paths

---

## Data Model Changes

### `Adapter` struct changes

The existing `ClipFormat string \`yaml:"format"\`` field is **renamed** to `Format` so it covers all adapter types uniformly. The YAML tag `format` is unchanged — no YAML migration needed.

```go
// Before (rename this field):
ClipFormat string `yaml:"format"`

// After:
Format string `yaml:"format,omitempty"` // "markdown" (default) | "plain-prose"
```

New fields added alongside the rename:

```go
MaxBytes   int    `yaml:"max_bytes,omitempty"`   // hard byte ceiling; 0 = unconstrained
ExportPath string `yaml:"export_path,omitempty"` // file-export only: path to write full context
```

**Budget resolution in the engine** — replace the existing `maxBytes := a.MaxTokens * 4` line with:

```go
maxBytes := a.MaxBytes
if maxBytes == 0 && a.MaxTokens > 0 {
    maxBytes = a.MaxTokens * 4
}
```

`MaxBytes` is used directly when set (e.g. ChatGPT's hard 1,400-byte cap). Otherwise `MaxTokens * 4` is the approximation. Zero means unconstrained. The existing `MaxTokens` field remains for backwards compatibility.

### `Target` struct addition

```go
Preamble string `yaml:"preamble,omitempty"` // prepended verbatim before content; replace-file mode only
```

`preamble` is **silently ignored** for `replace-section` targets — it is only consumed by `runFileInject` when `mode == "replace-file"`. A non-empty `preamble` on a `replace-section` target does not cause an error.

### New `Target.Mode` value: `replace-file`

Owns the entire file. Writes `preamble + "\n" + content` and overwrites on every inject. No HTML comment markers. Used for adapters that exclusively own a dedicated file (Cursor `.mdc`, Continue rules file, JetBrains rules file).

Contrast with `replace-section` (existing): writes between `<!-- sap-devs:start:NAME -->` / `<!-- sap-devs:end:NAME -->` markers inside a file the user also owns (Claude Code `CLAUDE.md`, Copilot `copilot-instructions.md`).

### New `Adapter.Type` value: `file-export`

A hybrid adapter that does two things in a single `inject` run:

1. Writes the full rendered context (no budget cap) to a local file at `export_path`.
2. Copies a short summary (≤ `max_bytes`, `format`-processed) plus a guidance line to the system clipboard.

```yaml
# chatgpt.yaml shape
type: file-export
export_path: "~/sap-devs-chatgpt-context.md"
max_bytes: 1400
format: plain-prose
instructions: "Paste into ChatGPT → Settings → Custom Instructions → ..."
```

The short clipboard payload appends: `"Full SAP context saved to ~/sap-devs-chatgpt-context.md — upload to a ChatGPT Project for comprehensive knowledge."`

`file-export` is **skipped for project scope** (same guard as `clipboard-export`) since the export file and clipboard are global resources.

If `export_path` is empty or missing, `ExportFileAndClip` returns an error: `"adapter %s: export_path is required for file-export type"`.

---

## Rendering Pipeline

```text
TrimPacks(packs, effectiveBudget)
    → RenderContext(trimmed, profile, dynamic)      // always produces Markdown (unchanged)
    → FormatOutput(ctx, adapter.Format)             // skipped for file-export (see below)
    → dispatch: file-inject | clipboard-export | file-export | mcp-wire
```

`RenderContext` is **unchanged** — it always produces Markdown. All existing tests pass without modification.

**`file-export` exception:** The engine-level `FormatOutput` call is **skipped** for `file-export` adapters. `ExportFileAndClip` receives raw Markdown as `fullCtx`, writes it verbatim to disk (preserving Markdown for the ChatGPT Project upload), and applies `TrimToBytes` + `FormatOutput` only on the clipboard payload.

### `FormatOutput(content string, format string) string`

Lives in `internal/content/render.go`. Exported. For `format == "markdown"` (or empty string): returns input unchanged. For `format == "plain-prose"`, applies these transforms in order:

| Input pattern | Output |
| --- | --- |
| `## Section Title` (ATX headers, any level) | `Section Title` |
| `**bold text**` | `bold text` |
| `*italic text*` | `italic text` |
| `` `inline code` `` | `inline code` |
| Fenced code block (see below) | body kept, fences stripped |
| `<!-- comment -->` | removed |
| 3+ consecutive blank lines | 2 blank lines |

**Fenced code block regex** — use a multiline, non-greedy pattern anchored to line starts to avoid merging adjacent blocks:

```go
regexp.MustCompile("(?m)^```[^\n]*\n((?:[^`]|`[^`]|``[^`])*?)^```")
```

Replacement: `$1` (body only). This pattern anchors both fences at the start of a line, preventing greedy matches across multiple consecutive code blocks. Add `TestFormatOutput_PlainProse_MultipleCodeBlocks` to verify two adjacent blocks are each stripped independently.

### `TrimToBytes(s string, maxBytes int) string`

Lives in `internal/content/render.go`. Exported. Truncates `s` to at most `maxBytes` bytes, cutting at the last complete UTF-8 rune boundary. If `maxBytes <= 0` or `len(s) <= maxBytes`, returns `s` unchanged. Used by `ExportFileAndClip` to produce the short clipboard payload before appending the guidance line.

---

## File Write Modes

### `replace-section` (existing, unchanged)

Writes between HTML comment markers in a potentially shared file. Used for:

- Claude Code: `~/.claude/CLAUDE.md`, `./CLAUDE.md`
- GitHub Copilot: `.github/copilot-instructions.md`

### `replace-file` (new)

Writes `preamble + "\n" + content` to the entire file, overwriting on every inject. Parent directories created as needed.

```go
func ReplaceFile(filePath, preamble, content string, dryRun bool) error
```

**Dry-run output:**

```text
[dry-run] would write file <filePath> (<N> bytes)
```

Where `N` is `len(preamble) + 1 + len(content)` (the `"\n"` separator is counted). No file content is printed to avoid flooding the terminal with large context output.

Used for adapters that own a dedicated file:

- Cursor: `.cursor/rules/sap-developer-context.mdc`
- Continue.dev: `.continue/rules/sap-developer-context.md`
- JetBrains AI Assistant: `.aiassistant/rules/sap-developer-context.md`

---

## Stats Reporting

`adapterStats` gains a `BudgetBytes int` field (replacing the misleading `BudgetTokens` for `max_bytes` adapters) and a `Format string` field. `printStats` displays effective budget in bytes when `BudgetBytes > 0`, otherwise in tokens.

```go
type adapterStats struct {
    AdapterID    string
    PackIDs      []string
    ApproxTokens int
    BudgetBytes  int    // effective budget in bytes; 0 = unconstrained
    Format       string // "markdown" | "plain-prose" | ""
    Trimmed      bool
}
```

The stats column header changes from `Budget` to `Budget (bytes)` when any adapter uses `max_bytes`; otherwise stays as `Budget (tokens)`. Simplest approach: always display bytes (multiply token budget by 4 for display).

---

## Adapter YAML Updates

### `content/adapters/cody.yaml` — DELETED

Cody has no static file context injection mechanism. It uses retrieval-based context exclusively. The current adapter writes to `.cody/context.md`, which Cody never reads. Correct future integration path: MCP (tracked separately).

### `content/adapters/jetbrains-ai.yaml` — path fix + replace-file

```yaml
id: jetbrains-ai
name: JetBrains AI Assistant
type: file-inject
targets:
  - scope: project
    path: ".aiassistant/rules/sap-developer-context.md"
    mode: replace-file
    # no preamble — rule type (always/conditional) is configured in IDE settings, not frontmatter
detect:
  - path: "~/.config/JetBrains"
```

### `content/adapters/cursor.yaml` — replace-file + frontmatter

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

### `content/adapters/continue.yaml` — replace-file + frontmatter + MCP format fix

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

Two changes from the previous version: rules path (`~/.continue/rules/` instead of `~/.continue/sap-context.md`) and `mcp_config.format: yaml` (the config file is YAML, not JSON — the previous value was a pre-existing bug).

### `content/adapters/chatgpt.yaml` — file-export hybrid

```yaml
id: chatgpt
name: ChatGPT
type: file-export
export_path: "~/sap-devs-chatgpt-context.md"
max_bytes: 1400
format: plain-prose
instructions: "Paste into ChatGPT → Settings → Custom Instructions → 'What would you like ChatGPT to know about you?'"
```

### `content/adapters/gemini.yaml` — plain-prose format

```yaml
id: gemini
name: Google Gemini
type: clipboard-export
format: plain-prose
instructions: "Paste into Gemini → Settings → Custom Instructions or into your Gemini for Google Workspace prompt."
```

### Adapters with no changes

| Adapter | Reason |
| --- | --- |
| `claude-code.yaml` | Plain Markdown; no hard limit; `replace-section` correct for shared CLAUDE.md |
| `copilot.yaml` | Plain Markdown; no documented limit; `replace-section` correct for shared file |
| `claude-ai.yaml` | Markdown renders in claude.ai; context-window-bound (no hard cap) |
| `sap-joule.yaml` | System prompt target; Markdown fine |
| `sap-ai-core.yaml` | System prompt target; Markdown fine |

---

## Go Code Changes

### `internal/adapter/adapter.go`

- Rename `ClipFormat` → `Format` (YAML tag `format` unchanged)
- Add `MaxBytes int` with tag `yaml:"max_bytes,omitempty"`
- Add `ExportPath string` with tag `yaml:"export_path,omitempty"`
- Add `Preamble string` to `Target` with tag `yaml:"preamble,omitempty"`

### `internal/adapter/engine.go`

- Replace `maxBytes := a.MaxTokens * 4` with the two-step resolution shown above
- After `RenderContext`, apply `content.FormatOutput(ctx, a.Format)`
- Add `case "file-export"` to the dispatch switch; skip if `opts.Scope == "project"`
- For `file-export`, skip the engine-level `FormatOutput` call; pass raw `ctx` as `fullCtx` to `ExportFileAndClip`
- Call `ExportFileAndClip(a, fullCtx, e.opts)` for `file-export`
- Update `adapterStats` to include `BudgetBytes int` and `Format string`; update `printStats`
- In the stats-append block, set `BudgetBytes: maxBytes` (the resolved value from the two-step budget calculation, not the raw `a.MaxBytes` field) and `Format: a.Format`

### `internal/adapter/file_inject.go`

- Add `ReplaceFile(filePath, preamble, content string, dryRun bool) error`
- Add `case "replace-file"` in `runFileInject` calling `ReplaceFile`

### `internal/adapter/file_export.go` (new file)

`ExportFileAndClip(a Adapter, fullCtx string, opts Options) error`:

1. If `a.ExportPath == ""` → return `fmt.Errorf("adapter %s: export_path is required for file-export type", a.ID)`
2. Expand and write `fullCtx` to `ExpandHome(a.ExportPath)` (create parent dirs; 0644)
3. Short payload: `content.TrimToBytes(fullCtx, a.MaxBytes)` → `content.FormatOutput(trimmed, a.Format)` → append guidance line
4. Call `ExportToClipboard(short, a.Instructions, opts.DryRun)`

### `internal/content/render.go`

- Add `FormatOutput(content, format string) string` — exported
- Add `TrimToBytes(s string, maxBytes int) string` — exported

---

## Tests

| File | New tests |
| --- | --- |
| `internal/content/render_test.go` | `TestFormatOutput_Markdown_NoOp`, `TestFormatOutput_PlainProse_Headers`, `TestFormatOutput_PlainProse_Bold`, `TestFormatOutput_PlainProse_InlineCode`, `TestFormatOutput_PlainProse_CodeBlock`, `TestFormatOutput_PlainProse_MultipleCodeBlocks`, `TestFormatOutput_PlainProse_HTMLComments`, `TestFormatOutput_PlainProse_NormalizesBlankLines`, `TestTrimToBytes_UnderLimit`, `TestTrimToBytes_ExactLimit`, `TestTrimToBytes_OverLimit`, `TestTrimToBytes_UTF8Boundary`, `TestTrimToBytes_Zero` |
| `internal/adapter/file_inject_test.go` | `TestReplaceFile_CreatesFile`, `TestReplaceFile_OverwritesOnReInject`, `TestReplaceFile_WithPreamble`, `TestReplaceFile_DryRun` |
| `internal/adapter/file_export_test.go` | `TestExportFileAndClip_WritesFullFile`, `TestExportFileAndClip_ClipsShortVersion`, `TestExportFileAndClip_AppendedGuidanceLine`, `TestExportFileAndClip_EmptyExportPath`, `TestExportFileAndClip_DryRun`, `TestExportFileAndClip_SkippedForProjectScope` |
| `internal/adapter/engine_test.go` | `TestEngine_MaxBytesOverridesMaxTokens`, `TestEngine_FormatApplied`, `TestEngine_FileExportType`, `TestEngine_FileExportSkippedForProjectScope` |

---

## Migration Notes

- **Cody users**: The `.cody/context.md` file written by previous versions is harmless but unused. Users can delete it manually. No automated cleanup.
- **JetBrains users**: Old `.idea/ai-context.md` files remain on disk but are no longer updated. Users can delete them manually. The new `.aiassistant/rules/sap-developer-context.md` is created on next inject.
- **Continue users**: Old `.continue/sap-context.md` files remain on disk. New files go to `.continue/rules/sap-developer-context.md`. Continue.dev picks up the new path automatically on next session.
- **Cursor users**: Old `.cursor/rules/sap-developer-context.mdc` is overwritten in-place (same path, new write mode). The first inject after this change replaces the HTML-marker-wrapped content with frontmatter + clean content. No manual action needed.
