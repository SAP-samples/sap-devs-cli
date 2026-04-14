package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldCheck_MissingFile(t *testing.T) {
	dir := t.TempDir()
	if !ShouldCheck(dir, time.Hour) {
		t.Fatal("expected true for missing cache file")
	}
}

func TestShouldCheck_RecentCheck(t *testing.T) {
	dir := t.TempDir()
	writeCache(t, dir, time.Now().Add(-10*time.Minute))
	if ShouldCheck(dir, time.Hour) {
		t.Fatal("expected false: recent check within TTL")
	}
}

func TestShouldCheck_ExpiredCheck(t *testing.T) {
	dir := t.TempDir()
	writeCache(t, dir, time.Now().Add(-25*time.Hour))
	if !ShouldCheck(dir, 24*time.Hour) {
		t.Fatal("expected true: check expired beyond TTL")
	}
}

func TestShouldCheck_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update_check.json")
	os.WriteFile(path, []byte("not json"), 0o644)
	if !ShouldCheck(dir, time.Hour) {
		t.Fatal("expected true: fail-open on corrupt file")
	}
}

func TestRecordCheck_WritesTimestamp(t *testing.T) {
	dir := t.TempDir()
	if err := RecordCheck(dir); err != nil {
		t.Fatalf("RecordCheck failed: %v", err)
	}
	path := filepath.Join(dir, "update_check.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cache file not created: %v", err)
	}
	var rec struct {
		LastCheck string `json:"last_check"`
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("invalid JSON in cache file: %v", err)
	}
	ts, err := time.Parse(time.RFC3339, rec.LastCheck)
	if err != nil {
		t.Fatalf("last_check is not RFC3339: %v", err)
	}
	if time.Since(ts) > 5*time.Second {
		t.Fatalf("last_check timestamp is too old: %v", ts)
	}
}

// writeCache writes a cache file with the given timestamp.
func writeCache(t *testing.T, dir string, ts time.Time) {
	t.Helper()
	path := filepath.Join(dir, "update_check.json")
	data, _ := json.Marshal(map[string]string{"last_check": ts.Format(time.RFC3339)})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
