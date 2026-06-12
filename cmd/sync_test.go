package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveStaleExpansion_DeletesFile(t *testing.T) {
	packDir := t.TempDir()
	expandedPath := filepath.Join(packDir, "context.expanded.md")
	require.NoError(t, os.WriteFile(expandedPath, []byte("stale"), 0644))

	require.NoError(t, removeStaleExpansion(packDir))

	_, err := os.Stat(expandedPath)
	assert.True(t, os.IsNotExist(err), "context.expanded.md should be gone, got err=%v", err)
}

func TestRemoveStaleExpansion_NoFileIsNotAnError(t *testing.T) {
	packDir := t.TempDir()
	// No context.expanded.md exists.
	require.NoError(t, removeStaleExpansion(packDir))
}

func TestRemoveStaleExpansion_LeavesContextMdAlone(t *testing.T) {
	packDir := t.TempDir()
	contextPath := filepath.Join(packDir, "context.md")
	expandedPath := filepath.Join(packDir, "context.expanded.md")
	require.NoError(t, os.WriteFile(contextPath, []byte("source"), 0644))
	require.NoError(t, os.WriteFile(expandedPath, []byte("stale"), 0644))

	require.NoError(t, removeStaleExpansion(packDir))

	data, err := os.ReadFile(contextPath)
	require.NoError(t, err)
	assert.Equal(t, "source", string(data))
}
