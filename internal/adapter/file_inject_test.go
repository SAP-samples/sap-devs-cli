package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceSection_FirstInject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(path, []byte("# My Notes\n\nExisting content.\n"), 0644))

	err := ReplaceSection(path, "SAP Developer Context", "## SAP Tips\n\nUse CAP.\n", false)
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
	require.NoError(t, ReplaceSection(path, "SAP Developer Context", "v1 content", false))
	// Second inject with different content
	require.NoError(t, ReplaceSection(path, "SAP Developer Context", "v2 content", false))

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

	err := ReplaceSection(path, "SAP Developer Context", "content", false)
	require.NoError(t, err)

	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestReplaceSection_DryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")

	err := ReplaceSection(path, "SAP Developer Context", "injected", true)
	require.NoError(t, err)

	// File should not be created in dry-run
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestReplaceFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules", "sap.mdc")

	err := ReplaceFile(path, "", "content here", false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "content here", string(data))
}

func TestReplaceFile_WithPreamble(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sap.mdc")
	preamble := "---\nalwaysApply: true\n---"

	err := ReplaceFile(path, preamble, "the content", false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, preamble+"\nthe content", string(data))
}

func TestReplaceFile_OverwritesOnReInject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sap.mdc")

	require.NoError(t, ReplaceFile(path, "", "first run", false))
	require.NoError(t, ReplaceFile(path, "", "second run", false))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "second run", string(data))
	assert.NotContains(t, string(data), "first run")
}

func TestReplaceFile_DryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sap.mdc")

	err := ReplaceFile(path, "preamble", "content", true)
	require.NoError(t, err)

	// File must not be created in dry-run mode
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "dry-run must not write file")
}

func TestExpandHome_TildeSlash(t *testing.T) {
	result, err := ExpandHome("~/foo/bar")
	require.NoError(t, err)
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, "foo", "bar"), result)
}

func TestExpandHome_TildeOnly(t *testing.T) {
	result, err := ExpandHome("~")
	require.NoError(t, err)
	home, _ := os.UserHomeDir()
	assert.Equal(t, home, result)
}

func TestExpandHome_NoTilde(t *testing.T) {
	result, err := ExpandHome("/absolute/path")
	require.NoError(t, err)
	assert.Equal(t, "/absolute/path", result)
}

func TestExpandHome_Relative(t *testing.T) {
	result, err := ExpandHome("./relative/path")
	require.NoError(t, err)
	assert.Equal(t, "./relative/path", result)
}

func TestFindSection_Found(t *testing.T) {
	content := "before\n<!-- sap-devs:start:X -->\nbody\n<!-- sap-devs:end:X -->\nafter\n"
	start := "<!-- sap-devs:start:X -->"
	end := "<!-- sap-devs:end:X -->"
	startIdx, endIdx, status := findSection(content, start, end)
	assert.Equal(t, sectionFound, status)
	assert.Equal(t, strings.Index(content, start), startIdx)
	assert.Equal(t, strings.Index(content, end), endIdx)
}

func TestFindSection_NotFound(t *testing.T) {
	_, _, status := findSection("no markers here", "<!-- sap-devs:start:X -->", "<!-- sap-devs:end:X -->")
	assert.Equal(t, sectionNotFound, status)
}

func TestFindSection_OrphanedStart(t *testing.T) {
	content := "before\n<!-- sap-devs:start:X -->\nbody\n"
	_, _, status := findSection(content, "<!-- sap-devs:start:X -->", "<!-- sap-devs:end:X -->")
	assert.Equal(t, sectionOrphaned, status)
}

func TestFindSection_OrphanedEnd(t *testing.T) {
	content := "before\n<!-- sap-devs:end:X -->\nbody\n"
	_, _, status := findSection(content, "<!-- sap-devs:start:X -->", "<!-- sap-devs:end:X -->")
	assert.Equal(t, sectionOrphaned, status)
}

func TestRemoveSection_LiveSectionPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	input := "before\n\n<!-- sap-devs:start:X -->\nbody\n<!-- sap-devs:end:X -->\n\nafter\n"
	require.NoError(t, os.WriteFile(path, []byte(input), 0644))

	var w strings.Builder
	found, removed, err := removeSection(path, "X", false, &w)
	require.NoError(t, err)
	assert.True(t, found)
	assert.True(t, removed)
	assert.Empty(t, w.String(), "live mode writes nothing to w")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "before\n\nafter\n", string(data))
}

func TestRemoveSection_LiveSectionAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(path, []byte("no markers\n"), 0644))

	var w strings.Builder
	found, removed, err := removeSection(path, "X", false, &w)
	require.NoError(t, err)
	assert.False(t, found)
	assert.False(t, removed)
}

func TestRemoveSection_FileAbsent(t *testing.T) {
	var w strings.Builder
	found, removed, err := removeSection("/no/such/path.md", "X", false, &w)
	require.NoError(t, err)
	assert.False(t, found)
	assert.False(t, removed)
}

func TestRemoveSection_OrphanedStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(path, []byte("before\n<!-- sap-devs:start:X -->\nbody\n"), 0644))

	var w strings.Builder
	_, _, err := removeSection(path, "X", false, &w)
	require.Error(t, err)
}

func TestRemoveSection_OrphanedEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(path, []byte("before\n<!-- sap-devs:end:X -->\nbody\n"), 0644))

	var w strings.Builder
	_, _, err := removeSection(path, "X", false, &w)
	require.Error(t, err)
}

func TestRemoveSection_DryRunSectionPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	input := "before\n<!-- sap-devs:start:X -->\nbody\n<!-- sap-devs:end:X -->\nafter\n"
	require.NoError(t, os.WriteFile(path, []byte(input), 0644))

	var w strings.Builder
	found, removed, err := removeSection(path, "X", true, &w)
	require.NoError(t, err)
	assert.True(t, found)
	assert.False(t, removed)
	assert.Contains(t, w.String(), "[dry-run]")
	assert.Contains(t, w.String(), "X")

	// File must be unchanged
	data, _ := os.ReadFile(path)
	assert.Equal(t, input, string(data))
}

func TestRemoveSection_DryRunSectionAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(path, []byte("no markers\n"), 0644))

	var w strings.Builder
	found, removed, err := removeSection(path, "X", true, &w)
	require.NoError(t, err)
	assert.False(t, found)
	assert.False(t, removed)
	assert.Empty(t, w.String())
}
