package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const cacheFile = "update_check.json"

type checkRecord struct {
	LastCheck string `json:"last_check"`
}

// ShouldCheck returns true if enough time has passed since the last update check.
// Returns true if the cache file is missing or unreadable (fail-open).
func ShouldCheck(cacheDir string, ttl time.Duration) bool {
	path := filepath.Join(cacheDir, cacheFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return true // missing or unreadable → check
	}
	var rec checkRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return true // corrupt → check
	}
	last, err := time.Parse(time.RFC3339, rec.LastCheck)
	if err != nil {
		return true // unparseable → check
	}
	return time.Since(last) >= ttl
}

// RecordCheck writes the current time to the cache file.
// Only called after a successful response from CheckLatest (not on network errors).
func RecordCheck(cacheDir string) error {
	path := filepath.Join(cacheDir, cacheFile)
	rec := checkRecord{LastCheck: time.Now().Format(time.RFC3339)}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
