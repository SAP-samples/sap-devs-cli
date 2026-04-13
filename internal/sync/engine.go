package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Engine tracks per-category sync timestamps and determines staleness.
type Engine struct {
	stateDir   string
	ttls       map[string]time.Duration
	defaultTTL time.Duration
}

// NewEngine creates an Engine that stores state in stateDir.
// ttls maps category name → TTL; categories not in the map use defaultTTL.
func NewEngine(stateDir string, defaultTTL time.Duration, ttls map[string]time.Duration) *Engine {
	return &Engine{stateDir: stateDir, defaultTTL: defaultTTL, ttls: ttls}
}

// IsStale reports whether the given category needs a refresh.
func (e *Engine) IsStale(category string) bool {
	ttl := e.defaultTTL
	if t, ok := e.ttls[category]; ok && t > 0 {
		ttl = t
	}
	state := e.loadState()
	last, ok := state[category]
	if !ok {
		return true
	}
	return time.Since(last) > ttl
}

// MarkSynced records the current time as the last sync time for category.
func (e *Engine) MarkSynced(category string) error {
	state := e.loadState()
	state[category] = time.Now()
	return e.saveState(state)
}

// MarkAllSynced records the current time as the last sync time for all given categories in a single write.
func (e *Engine) MarkAllSynced(categories []string) error {
	state := e.loadState()
	now := time.Now()
	for _, cat := range categories {
		state[cat] = now
	}
	return e.saveState(state)
}

func (e *Engine) loadState() map[string]time.Time {
	state := make(map[string]time.Time)
	data, err := os.ReadFile(filepath.Join(e.stateDir, "sync-state.json"))
	if err != nil {
		return state
	}
	if err := json.Unmarshal(data, &state); err != nil {
		// Corrupted state file — remove it so it gets rebuilt cleanly
		_ = os.Remove(filepath.Join(e.stateDir, "sync-state.json"))
		return make(map[string]time.Time)
	}
	return state
}

func (e *Engine) saveState(state map[string]time.Time) error {
	if err := os.MkdirAll(e.stateDir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.stateDir, "sync-state.json"), data, 0600)
}
