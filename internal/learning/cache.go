package learning

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SaveIndex writes the learning journey index to cache.
func SaveIndex(cacheDir string, journeys []LearningJourney) error {
	p := indexPath(cacheDir)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	b, err := json.Marshal(journeys)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}

// LoadIndex reads the cached learning journey index.
// Returns nil, false if the cache is missing or older than ttl.
func LoadIndex(cacheDir string, ttl time.Duration) ([]LearningJourney, bool) {
	p := indexPath(cacheDir)
	info, err := os.Stat(p)
	if err != nil {
		return nil, false
	}
	if ttl > 0 && time.Since(info.ModTime()) > ttl {
		return nil, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	var v []LearningJourney
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, false
	}
	return v, true
}

// LoadIndexStale reads the cached index ignoring TTL (offline fallback).
func LoadIndexStale(cacheDir string) ([]LearningJourney, bool) {
	return LoadIndex(cacheDir, 0)
}

// IndexCacheAge returns the age of the index cache, or -1 if missing.
func IndexCacheAge(cacheDir string) time.Duration {
	info, err := os.Stat(indexPath(cacheDir))
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime())
}

// SaveSearchCache writes search results with a deterministic key.
func SaveSearchCache(cacheDir, key string, results []LearningJourney) error {
	p := filepath.Join(cacheDir, "learning", key+".json")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	b, err := json.Marshal(results)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}

// LoadSearchCache reads cached search results if fresh.
func LoadSearchCache(cacheDir, key string) ([]LearningJourney, bool) {
	p := filepath.Join(cacheDir, "learning", key+".json")
	info, err := os.Stat(p)
	if err != nil {
		return nil, false
	}
	if time.Since(info.ModTime()) > SearchCacheTTL {
		return nil, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	var v []LearningJourney
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, false
	}
	return v, true
}

// SearchCacheKey returns a deterministic cache name for a search query.
func SearchCacheKey(query string, level, role string) string {
	raw := fmt.Sprintf("%s|%s|%s", query, level, role)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("search-%x", h[:8])
}

func indexPath(cacheDir string) string {
	return filepath.Join(cacheDir, "learning", "index.json")
}
