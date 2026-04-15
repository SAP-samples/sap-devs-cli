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

// writeCategoryTimestamps writes a SyncState with the given category timestamps.
// Use this instead of writing the old flat map[string]time.Time format.
func writeCategoryTimestamps(t *testing.T, dir string, cats map[string]time.Time) {
	t.Helper()
	type syncStateShape struct {
		Version    int                  `json:"version"`
		Categories map[string]time.Time `json:"categories"`
	}
	data, _ := json.Marshal(syncStateShape{Version: 1, Categories: cats})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sync-state.json"), data, 0600))
}

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
	writeCategoryTimestamps(t, dir, ts)

	eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
	assert.True(t, eng.IsStale("tips"))
}

func TestEngine_IsStale_HonoursPerCategoryTTL(t *testing.T) {
	dir := t.TempDir()
	// resources was synced 2 days ago
	ts := map[string]time.Time{"resources": time.Now().Add(-48 * time.Hour)}
	writeCategoryTimestamps(t, dir, ts)

	// 168h TTL for resources — 2 days is not stale
	eng := sapSync.NewEngine(dir, 24*time.Hour, map[string]time.Duration{"resources": 168 * time.Hour})
	assert.False(t, eng.IsStale("resources"))
	// But tips with default 24h TTL and no sync record is stale
	assert.True(t, eng.IsStale("tips"))
}

func TestEngine_SetAndGetPackHasMarkers(t *testing.T) {
	dir := t.TempDir()
	eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
	require.NoError(t, eng.SetPackHasMarkers("cap", true))
	packs := eng.PacksBlock()
	require.NotNil(t, packs)
	assert.True(t, packs["cap"].HasMarkers)
}

func TestEngine_RecordAndGetMarkerState(t *testing.T) {
	dir := t.TempDir()
	eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
	ms := sapSync.MarkerState{
		URL:         "https://example.com",
		LastFetched: time.Now(),
		TTLHours:    168,
		OK:          true,
	}
	require.NoError(t, eng.RecordMarkerState("cap", 0, ms))
	got, ok := eng.GetMarkerState("cap", 0)
	require.True(t, ok)
	assert.Equal(t, ms.URL, got.URL)
	assert.True(t, got.OK)
}

func TestEngine_GetMarkerState_NotFound(t *testing.T) {
	dir := t.TempDir()
	eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
	_, ok := eng.GetMarkerState("cap", 0)
	assert.False(t, ok)
}

func TestLoadSyncState_NewFormatRoundTrips(t *testing.T) {
	dir := t.TempDir()
	eng := sapSync.NewEngine(dir, 24*time.Hour, nil)
	require.NoError(t, eng.SetPackHasMarkers("cap", true))
	ms := sapSync.MarkerState{URL: "https://x.com", TTLHours: 168, OK: true, LastFetched: time.Now()}
	require.NoError(t, eng.RecordMarkerState("cap", 0, ms))
	// Reload from disk and verify
	got, ok := eng.GetMarkerState("cap", 0)
	require.True(t, ok)
	assert.Equal(t, ms.URL, got.URL)
	packs := eng.PacksBlock()
	assert.True(t, packs["cap"].HasMarkers)
}
