package sync

import (
	"time"
)

// Engine tracks sync timestamps and marker state.
type Engine struct {
	stateDir   string
	ttls       map[string]time.Duration
	defaultTTL time.Duration
}

// NewEngine creates an Engine that stores state in stateDir.
func NewEngine(stateDir string, defaultTTL time.Duration, ttls map[string]time.Duration) *Engine {
	return &Engine{stateDir: stateDir, defaultTTL: defaultTTL, ttls: ttls}
}

// IsStale reports whether the given category needs a refresh.
func (e *Engine) IsStale(category string) bool {
	ttl := e.defaultTTL
	if t, ok := e.ttls[category]; ok && t > 0 {
		ttl = t
	}
	state := loadSyncState(e.stateDir)
	last, ok := state.Categories[category]
	if !ok {
		return true
	}
	return time.Since(last) > ttl
}

// MarkSynced records the current time as the last sync time for category.
func (e *Engine) MarkSynced(category string) error {
	state := loadSyncState(e.stateDir)
	state.Categories[category] = time.Now()
	return saveSyncState(e.stateDir, state)
}

// MarkAllSynced records the current time for all given categories in a single write.
func (e *Engine) MarkAllSynced(categories []string) error {
	state := loadSyncState(e.stateDir)
	now := time.Now()
	for _, cat := range categories {
		state.Categories[cat] = now
	}
	return saveSyncState(e.stateDir, state)
}

// SetPackHasMarkers records whether a pack contains sync:fetch markers.
func (e *Engine) SetPackHasMarkers(packID string, hasMarkers bool) error {
	state := loadSyncState(e.stateDir)
	state.Packs[packID] = PackState{HasMarkers: hasMarkers}
	return saveSyncState(e.stateDir, state)
}

// RecordMarkerState persists the result of a marker fetch.
func (e *Engine) RecordMarkerState(packID string, index int, ms MarkerState) error {
	state := loadSyncState(e.stateDir)
	state.Markers[markerKey(packID, index)] = ms
	return saveSyncState(e.stateDir, state)
}

// GetMarkerState retrieves the last fetch result for a marker.
func (e *Engine) GetMarkerState(packID string, index int) (MarkerState, bool) {
	state := loadSyncState(e.stateDir)
	ms, ok := state.Markers[markerKey(packID, index)]
	return ms, ok
}

// PacksBlock returns the full packs map, or nil if no packs have been recorded yet.
func (e *Engine) PacksBlock() map[string]PackState {
	p := loadSyncState(e.stateDir).Packs
	if len(p) == 0 {
		return nil
	}
	return p
}

// MostRecentSync returns a pointer to the most recent non-zero category sync time
// recorded in stateDir, or nil if no syncs have been recorded.
func MostRecentSync(stateDir string) *time.Time {
	state := loadSyncState(stateDir)
	var most time.Time
	for _, ts := range state.Categories {
		if ts.After(most) {
			most = ts
		}
	}
	if most.IsZero() {
		return nil
	}
	return &most
}
