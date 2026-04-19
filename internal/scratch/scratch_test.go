package scratch_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/scratch"
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
