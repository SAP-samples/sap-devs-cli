package tutorials_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
)

func TestIndexCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	index := []tutorials.TutorialMeta{
		{Slug: "cap-getting-started", Title: "Getting Started with CAP", Time: 30, Level: "beginner", Repo: "Tutorials"},
		{Slug: "abap-rap-create", Title: "Create a RAP BO", Time: 20, Level: "intermediate", Repo: "abap-core-development"},
	}

	require.NoError(t, tutorials.SaveIndex(dir, index))
	loaded, err := tutorials.LoadIndex(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, "cap-getting-started", loaded[0].Slug)
	assert.Equal(t, 30, loaded[0].Time)
}

func TestIndexCache_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := tutorials.LoadIndex(dir)
	assert.Error(t, err)
}

func TestContentCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	tut := &tutorials.Tutorial{
		TutorialMeta: tutorials.TutorialMeta{Slug: "test-tutorial", Title: "Test"},
		Steps: []tutorials.TutorialStep{
			{Number: 1, Title: "Step One", Content: "Do this."},
		},
	}

	require.NoError(t, tutorials.SaveContent(dir, tut))
	loaded, err := tutorials.LoadContent(dir, "test-tutorial")
	require.NoError(t, err)
	require.Len(t, loaded.Steps, 1)
	assert.Equal(t, "Step One", loaded.Steps[0].Title)
}

func TestContentCache_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := tutorials.LoadContent(dir, "nonexistent")
	assert.Error(t, err)
}

func TestRepoInfoCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	repos := []tutorials.RepoInfo{
		{Name: "Tutorials", DefaultBranch: "master", TreeSHA: "abc123"},
		{Name: "abap-core-development", DefaultBranch: "main", TreeSHA: "def456"},
	}

	require.NoError(t, tutorials.SaveRepoInfo(dir, repos))
	loaded, err := tutorials.LoadRepoInfo(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, "master", loaded[0].DefaultBranch)
}

func TestCacheAge_Exists(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "tutorials"), 0755)
	os.WriteFile(filepath.Join(dir, "tutorials", "index.json"), []byte("[]"), 0644)
	age := tutorials.IndexCacheAge(dir)
	assert.True(t, age >= 0)
}

func TestCacheAge_Missing(t *testing.T) {
	dir := t.TempDir()
	age := tutorials.IndexCacheAge(dir)
	assert.True(t, age < 0)
}
