package sync_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
)

func TestEngine_IsStale_TrueWhenNeverSynced(t *testing.T) {
	dir := t.TempDir()
	eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
	assert.True(t, eng.IsStale("tips"))
}

func TestEngine_IsStale_FalseWhenRecentlySynced(t *testing.T) {
	dir := t.TempDir()
	eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
	require.NoError(t, eng.MarkSynced("tips"))
	assert.False(t, eng.IsStale("tips"))
}

func TestEngine_IsStale_TrueWhenExpired(t *testing.T) {
	dir := t.TempDir()
	// Write a timestamp 2 days ago
	ts := map[string]time.Time{"tips": time.Now().Add(-48 * time.Hour)}
	data, _ := json.Marshal(ts)
	os.WriteFile(filepath.Join(dir, "sync-state.json"), data, 0600)

	eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
	assert.True(t, eng.IsStale("tips"))
}

func TestEngine_IsStale_HonoursPerCategoryTTL(t *testing.T) {
	dir := t.TempDir()
	// resources was synced 2 days ago
	ts := map[string]time.Time{"resources": time.Now().Add(-48 * time.Hour)}
	data, _ := json.Marshal(ts)
	os.WriteFile(filepath.Join(dir, "sync-state.json"), data, 0600)

	// 168h TTL for resources — 2 days is not stale
	eng := sapSync.NewEngine(dir, 24*time.Hour, map[string]time.Duration{"resources": 168 * time.Hour})
	assert.False(t, eng.IsStale("resources"))
	// But tips with default 24h TTL and no sync record is stale
	assert.True(t, eng.IsStale("tips"))
}
