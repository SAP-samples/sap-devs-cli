package adapter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
)

func TestReplaceSection_FirstInject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(path, []byte("# My Notes\n\nExisting content.\n"), 0644))

	err := adapter.ReplaceSection(path, "SAP Developer Context", "## SAP Tips\n\nUse CAP.\n", false)
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(got)

	assert.Contains(t, content, "# My Notes")
	assert.Contains(t, content, "Existing content.")
	assert.Contains(t, content, "<!-- sap-devs:start:SAP Developer Context -->")
	assert.Contains(t, content, "## SAP Tips")
	assert.Contains(t, content, "Use CAP.")
	assert.Contains(t, content, "<!-- sap-devs:end:SAP Developer Context -->")
}

func TestReplaceSection_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")

	// First inject
	require.NoError(t, adapter.ReplaceSection(path, "SAP Developer Context", "v1 content", false))
	// Second inject with different content
	require.NoError(t, adapter.ReplaceSection(path, "SAP Developer Context", "v2 content", false))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(got)

	// Only one section
	assert.Equal(t, 1, strings.Count(content, "<!-- sap-devs:start:SAP Developer Context -->"))
	assert.Contains(t, content, "v2 content")
	assert.NotContains(t, content, "v1 content")
}

func TestReplaceSection_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "CLAUDE.md")

	err := adapter.ReplaceSection(path, "SAP Developer Context", "content", false)
	require.NoError(t, err)

	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestReplaceSection_DryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")

	err := adapter.ReplaceSection(path, "SAP Developer Context", "injected", true)
	require.NoError(t, err)

	// File should not be created in dry-run
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestExpandHome_TildeSlash(t *testing.T) {
	result, err := adapter.ExpandHome("~/foo/bar")
	require.NoError(t, err)
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, "foo", "bar"), result)
}

func TestExpandHome_TildeOnly(t *testing.T) {
	result, err := adapter.ExpandHome("~")
	require.NoError(t, err)
	home, _ := os.UserHomeDir()
	assert.Equal(t, home, result)
}

func TestExpandHome_NoTilde(t *testing.T) {
	result, err := adapter.ExpandHome("/absolute/path")
	require.NoError(t, err)
	assert.Equal(t, "/absolute/path", result)
}

func TestExpandHome_Relative(t *testing.T) {
	result, err := adapter.ExpandHome("./relative/path")
	require.NoError(t, err)
	assert.Equal(t, "./relative/path", result)
}
