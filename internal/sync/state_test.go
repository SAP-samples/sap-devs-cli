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

func TestLoadSyncState_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	state := sapSync.LoadSyncStateForTest(dir)
	assert.Equal(t, 1, state.Version)
	assert.NotNil(t, state.Categories)
	assert.NotNil(t, state.Packs)
	assert.NotNil(t, state.Markers)
}

func TestLoadSyncState_OldFlatFormat_Resets(t *testing.T) {
	dir := t.TempDir()
	// Write old flat map[string]time.Time format
	old := map[string]time.Time{"tips": time.Now().Add(-1 * time.Hour)}
	data, _ := json.Marshal(old)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sync-state.json"), data, 0600))

	state := sapSync.LoadSyncStateForTest(dir)
	// Old format cannot unmarshal into SyncState → reset
	assert.Equal(t, 1, state.Version)
	assert.Empty(t, state.Categories)
}
