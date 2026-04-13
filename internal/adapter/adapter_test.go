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

func writeYAML(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}
