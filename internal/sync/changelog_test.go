package sync_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
)

func TestWriteReadChangelog_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	entries := []sapSync.ChangelogEntry{
		{Pack: "cap", Text: "CAP 9.8: native SQLite support"},
		{Pack: "abap", Text: "New Tier-1 API for business partner"},
	}
	syncedAt := time.Date(2026, 4, 17, 15, 4, 5, 0, time.UTC)

	require.NoError(t, sapSync.WriteChangelog(dir, syncedAt, entries))

	gotEntries, gotTime, err := sapSync.ReadChangelog(dir)
	require.NoError(t, err)
	assert.Equal(t, entries, gotEntries)
	assert.True(t, syncedAt.Equal(gotTime))
}

func TestReadChangelog_MissingFile(t *testing.T) {
	dir := t.TempDir()
	entries, ts, err := sapSync.ReadChangelog(dir)
	assert.NoError(t, err)
	assert.Nil(t, entries)
	assert.True(t, ts.IsZero())
}

func TestConsumeChangelog_DeletesFile(t *testing.T) {
	dir := t.TempDir()
	entries := []sapSync.ChangelogEntry{{Pack: "cap", Text: "test"}}
	require.NoError(t, sapSync.WriteChangelog(dir, time.Now(), entries))

	require.NoError(t, sapSync.ConsumeChangelog(dir))

	_, err := os.Stat(filepath.Join(dir, "sync-changelog.json"))
	assert.True(t, os.IsNotExist(err))
}

func TestConsumeChangelog_MissingFile_NoOp(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, sapSync.ConsumeChangelog(dir))
}

func TestWriteChangelog_EmptyEntries_NoFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, sapSync.WriteChangelog(dir, time.Now(), nil))

	_, err := os.Stat(filepath.Join(dir, "sync-changelog.json"))
	assert.True(t, os.IsNotExist(err))
}

func TestCollectChangelog_ReadsFromMultipleDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	capDir := filepath.Join(dir1, "cap")
	require.NoError(t, os.MkdirAll(capDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(capDir, "pack.yaml"), []byte("id: cap\nname: CAP\ndescription: test\ntags: [test]\nchangelog:\n  - \"CAP 9.8: native SQLite\"\n  - \"CAP 9.8: cds repl --ql\"\n"), 0644))

	abapDir := filepath.Join(dir2, "abap")
	require.NoError(t, os.MkdirAll(abapDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(abapDir, "pack.yaml"), []byte("id: abap\nname: ABAP\ndescription: test\ntags: [test]\nchangelog:\n  - \"New Tier-1 API\"\n"), 0644))

	entries, err := sapSync.CollectChangelog([]string{dir1, dir2})
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, "cap", entries[0].Pack)
	assert.Equal(t, "CAP 9.8: native SQLite", entries[0].Text)
	assert.Equal(t, "cap", entries[1].Pack)
	assert.Equal(t, "CAP 9.8: cds repl --ql", entries[1].Text)
	assert.Equal(t, "abap", entries[2].Pack)
	assert.Equal(t, "New Tier-1 API", entries[2].Text)
}

func TestCollectChangelog_SkipsPacksWithoutChangelog(t *testing.T) {
	dir := t.TempDir()
	capDir := filepath.Join(dir, "cap")
	require.NoError(t, os.MkdirAll(capDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(capDir, "pack.yaml"), []byte("id: cap\nname: CAP\ndescription: test\ntags: [test]\n"), 0644))

	entries, err := sapSync.CollectChangelog([]string{dir})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCollectChangelog_SkipsMissingDirs(t *testing.T) {
	entries, err := sapSync.CollectChangelog([]string{filepath.Join(os.TempDir(), "nonexistent-sap-devs-test")})
	require.NoError(t, err)
	assert.Empty(t, entries)
}
