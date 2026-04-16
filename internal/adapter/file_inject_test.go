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
