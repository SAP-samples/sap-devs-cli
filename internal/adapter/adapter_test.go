package adapter_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestLoadAdapters(t *testing.T) {
	dir := t.TempDir()

	writeYAML(t, filepath.Join(dir, "claude-code.yaml"), `
id: claude-code
name: Claude Code
type: file-inject
targets:
  - scope: global
    path: "~/.claude/CLAUDE.md"
    mode: replace-section
    section: "SAP Developer Context"
detect:
  - path: "~/.claude"
`)

	adapters, err := adapter.LoadAdapters(dir)
	require.NoError(t, err)
	require.Len(t, adapters, 1)
	assert.Equal(t, "claude-code", adapters[0].ID)
	assert.Equal(t, "file-inject", adapters[0].Type)
	require.Len(t, adapters[0].Targets, 1)
	assert.Equal(t, "global", adapters[0].Targets[0].Scope)
	assert.Equal(t, "~/.claude/CLAUDE.md", adapters[0].Targets[0].Path)
	assert.Equal(t, "replace-section", adapters[0].Targets[0].Mode)
	assert.Equal(t, "SAP Developer Context", adapters[0].Targets[0].Section)
}

func TestLoadAdapters_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	adapters, err := adapter.LoadAdapters(dir)
	require.NoError(t, err)
	assert.Empty(t, adapters)
}

func TestLoadAdapters_NonexistentDir(t *testing.T) {
	adapters, err := adapter.LoadAdapters("/no/such/dir")
	require.NoError(t, err)
	assert.Empty(t, adapters)
}

func TestLoadAdapters_SkipsNoID(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, filepath.Join(dir, "incomplete.yaml"), "name: Incomplete\ntype: file-inject\n")
	adapters, err := adapter.LoadAdapters(dir)
	require.NoError(t, err)
	assert.Empty(t, adapters)
}

func TestLoadAdapters_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, filepath.Join(dir, "bad.yaml"), "id: [broken yaml")
	_, err := adapter.LoadAdapters(dir)
	require.Error(t, err)
}

func writeYAML(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func TestEngine_FileInject_DryRun(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")

	adapters := []adapter.Adapter{
		{
			ID:   "test-tool",
			Type: "file-inject",
			Targets: []adapter.Target{
				{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
			},
		},
	}

	engine := adapter.NewEngine(adapters, nil, nil, adapter.Options{
		Scope:  "global",
		DryRun: true,
	})
	require.NoError(t, engine.Run())
	_, err := os.Stat(targetFile)
	assert.True(t, os.IsNotExist(err), "dry-run should not create file")
}

func TestEngine_SkipsWrongScope(t *testing.T) {
	dir := t.TempDir()
	projectFile := filepath.Join(dir, "proj.md")

	adapters := []adapter.Adapter{
		{
			ID:   "test-tool",
			Type: "file-inject",
			Targets: []adapter.Target{
				{Scope: "project", Path: projectFile, Mode: "replace-section", Section: "SAP Dev"},
			},
		},
	}

	// Running with global scope — project target should be skipped
	engine := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	require.NoError(t, engine.Run())
	_, err := os.Stat(projectFile)
	assert.True(t, os.IsNotExist(err), "global scope should skip project targets")
}

func TestEngine_FilterByTool(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.md")
	fileB := filepath.Join(dir, "b.md")

	adapters := []adapter.Adapter{
		{
			ID:   "tool-a",
			Type: "file-inject",
			Targets: []adapter.Target{{Scope: "global", Path: fileA, Mode: "replace-section", Section: "S"}},
		},
		{
			ID:   "tool-b",
			Type: "file-inject",
			Targets: []adapter.Target{{Scope: "global", Path: fileB, Mode: "replace-section", Section: "S"}},
		},
	}

	engine := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global", ToolFilter: "tool-a"})
	require.NoError(t, engine.Run())

	_, errA := os.Stat(fileA)
	_, errB := os.Stat(fileB)
	assert.NoError(t, errA, "tool-a target should be written")
	assert.True(t, os.IsNotExist(errB), "tool-b target should be skipped")
}

func TestEngine_ClipboardSkippedForProject(t *testing.T) {
	adapters := []adapter.Adapter{
		{
			ID:           "chatgpt",
			Type:         "clipboard-export",
			Instructions: "Paste into ChatGPT.",
		},
	}

	// clipboard-export should be skipped entirely for project scope
	engine := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "project"})
	// Should run without error (skipped, not attempted)
	require.NoError(t, engine.Run())
}

func TestLoadAdapters_MaxTokens(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, filepath.Join(dir, "cursor.yaml"), `
id: cursor
name: Cursor
type: file-inject
max_tokens: 2000
targets:
  - scope: global
    path: "~/.cursor/rules/sap.mdc"
    mode: replace-section
    section: "SAP Developer Context"
`)
	adapters, err := adapter.LoadAdapters(dir)
	require.NoError(t, err)
	require.Len(t, adapters, 1)
	assert.Equal(t, 2000, adapters[0].MaxTokens)
}

func TestLoadAdapters_MaxTokensDefaultsToZero(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, filepath.Join(dir, "claude-code.yaml"), `
id: claude-code
name: Claude Code
type: file-inject
targets:
  - scope: global
    path: "~/.claude/CLAUDE.md"
    mode: replace-section
    section: "SAP Developer Context"
`)
	adapters, err := adapter.LoadAdapters(dir)
	require.NoError(t, err)
	require.Len(t, adapters, 1)
	assert.Equal(t, 0, adapters[0].MaxTokens)
}

func TestEngine_PerAdapterBudget(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.md")

	packs := []*content.Pack{
		{ID: "cap", ContextMD: strings.Repeat("x", 1000)},  // 1000 bytes ≈ 250 tokens
		{ID: "btp", ContextMD: strings.Repeat("y", 1000)},  // 1000 bytes ≈ 250 tokens
	}

	// budget of 500 tokens = 2000 bytes: both packs fit
	adapters := []adapter.Adapter{
		{
			ID:        "tool-a",
			Type:      "file-inject",
			MaxTokens: 500,
			Targets:   []adapter.Target{{Scope: "global", Path: fileA, Mode: "replace-section", Section: "S"}},
		},
	}

	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global"})
	require.NoError(t, engine.Run())

	data, err := os.ReadFile(fileA)
	require.NoError(t, err)
	assert.Contains(t, string(data), strings.Repeat("x", 1000))
	assert.Contains(t, string(data), strings.Repeat("y", 1000))
}

func TestEngine_BudgetTooSmall_EmitsWarning(t *testing.T) {
	var buf bytes.Buffer
	packs := []*content.Pack{
		{ID: "cap", ContextMD: strings.Repeat("x", 1000)},
	}
	adapters := []adapter.Adapter{
		{
			ID:        "tiny-tool",
			Type:      "file-inject",
			MaxTokens: 1, // 4 bytes — too small for any pack
			Targets:   []adapter.Target{{Scope: "global", Path: t.TempDir() + "/f.md", Mode: "replace-section", Section: "S"}},
		},
	}
	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global", Out: &buf})
	require.NoError(t, engine.Run())
	// Warning goes to stderr (os.Stderr), not to Out
	assert.Empty(t, buf.String(), "budget-too-small warning should not appear in Out")
}

func TestEngine_Stats(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "out.md")

	packs := []*content.Pack{
		{ID: "cap", ContextMD: "CAP content"},
		{ID: "btp", ContextMD: "BTP content"},
	}
	adapters := []adapter.Adapter{
		{
			ID:        "test-tool",
			Type:      "file-inject",
			MaxTokens: 0,
			Targets:   []adapter.Target{{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "S"}},
		},
	}

	var buf bytes.Buffer
	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{
		Scope:  "global",
		DryRun: true,
		Stats:  true,
		Out:    &buf,
	})
	require.NoError(t, engine.Run())

	out := buf.String()
	assert.Contains(t, out, "Adapter")
	assert.Contains(t, out, "Status")
	assert.Contains(t, out, "test-tool")
	assert.Contains(t, out, "cap")
}

func TestEngine_NilOutIsSafe(t *testing.T) {
	packs := []*content.Pack{{ID: "cap", ContextMD: "content"}}
	adapters := []adapter.Adapter{
		{
			ID:   "test",
			Type: "file-inject",
			Targets: []adapter.Target{
				{Scope: "global", Path: t.TempDir() + "/f.md", Mode: "replace-section", Section: "S"},
			},
		},
	}
	// Out is nil — NewEngine must normalise to io.Discard
	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global"})
	require.NoError(t, engine.Run())
}

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
			MaxBytes:  50,   // hard limit — takes precedence
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
