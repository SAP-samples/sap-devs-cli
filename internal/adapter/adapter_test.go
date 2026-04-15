package adapter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
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

	engine := adapter.NewEngine(adapters, "# SAP Context\nUse CAP.", adapter.Options{
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
	engine := adapter.NewEngine(adapters, "content", adapter.Options{Scope: "global"})
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

	engine := adapter.NewEngine(adapters, "content", adapter.Options{Scope: "global", ToolFilter: "tool-a"})
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
	engine := adapter.NewEngine(adapters, "content", adapter.Options{Scope: "project"})
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
