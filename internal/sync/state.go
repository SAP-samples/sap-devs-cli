package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// SyncState is the versioned on-disk format of sync-state.json.
type SyncState struct {
	Version    int                    `json:"version"`
	Categories map[string]time.Time   `json:"categories"`
	Packs      map[string]PackState   `json:"packs"`
	Markers    map[string]MarkerState `json:"markers"`
}

// PackState records whether a pack has dynamic fetch markers.
type PackState struct {
	HasMarkers bool `json:"has_markers"`
}

// MarkerState records the result of the last fetch for a single marker.
type MarkerState struct {
	URL         string    `json:"url"`
	LastFetched time.Time `json:"last_fetched"`
	TTLHours    int       `json:"ttl_hours"`
	OK          bool      `json:"ok"`
}

func newSyncState() SyncState {
	return SyncState{
		Version:    1,
		Categories: make(map[string]time.Time),
		Packs:      make(map[string]PackState),
		Markers:    make(map[string]MarkerState),
	}
}

func loadSyncState(stateDir string) SyncState {
	data, err := os.ReadFile(filepath.Join(stateDir, "sync-state.json"))
	if err != nil {
		return newSyncState()
	}
	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil || state.Version < 1 {
		// Old flat format (map[string]time.Time) or corrupt file — reset.
		fmt.Fprintf(os.Stderr, "sap-devs: sync state reset after format upgrade\n")
		if removeErr := os.Remove(filepath.Join(stateDir, "sync-state.json")); removeErr != nil && !os.IsNotExist(removeErr) {
			fmt.Fprintf(os.Stderr, "sap-devs: could not remove stale sync state: %v\n", removeErr)
		}
		return newSyncState()
	}
	if state.Categories == nil {
		state.Categories = make(map[string]time.Time)
	}
	if state.Packs == nil {
		state.Packs = make(map[string]PackState)
	}
	if state.Markers == nil {
		state.Markers = make(map[string]MarkerState)
	}
	return state
}

func saveSyncState(stateDir string, state SyncState) error {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, "sync-state.json"), data, 0600)
}

// markerKey returns the sync-state.json key for a marker identified by pack + position index.
func markerKey(packID string, index int) string {
	return packID + "::" + strconv.Itoa(index)
}

// LoadSyncStateForTest exposes the internal state loader for tests.
func LoadSyncStateForTest(dir string) SyncState { return loadSyncState(dir) }
