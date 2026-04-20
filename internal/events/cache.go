package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

// LoadCache reads cached events for a given event type.
func LoadCache(cacheDir, typeID string) ([]content.EventInstance, error) {
	path := cachePath(cacheDir, typeID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var events []content.EventInstance
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// SaveCache writes events to the cache file for a given event type.
func SaveCache(cacheDir, typeID string, evts []content.EventInstance) error {
	dir := filepath.Join(cacheDir, "events")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(evts)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(cacheDir, typeID), data, 0644)
}

// CacheAge returns the age of the cache file, or -1 if it doesn't exist.
func CacheAge(cacheDir, typeID string) time.Duration {
	info, err := os.Stat(cachePath(cacheDir, typeID))
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime())
}

func cachePath(cacheDir, typeID string) string {
	return filepath.Join(cacheDir, "events", typeID+".json")
}
