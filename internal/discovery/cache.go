package discovery

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LoadCache reads a cached JSON file and unmarshals it into T.
// Returns the zero value of T and false if the cache is missing or older than ttl.
func LoadCache[T any](cacheDir, name string, ttl time.Duration) (T, bool) {
	var zero T
	p := cachePath(cacheDir, name)
	info, err := os.Stat(p)
	if err != nil {
		return zero, false
	}
	if time.Since(info.ModTime()) > ttl {
		return zero, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return zero, false
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return zero, false
	}
	return v, true
}

// LoadCacheStale reads a cached JSON file ignoring TTL.
// Used as fallback when the network is unavailable.
func LoadCacheStale[T any](cacheDir, name string) (T, bool) {
	var zero T
	p := cachePath(cacheDir, name)
	data, err := os.ReadFile(p)
	if err != nil {
		return zero, false
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return zero, false
	}
	return v, true
}

// SaveCache marshals data to JSON and writes it to the cache directory.
func SaveCache[T any](cacheDir, name string, data T) error {
	p := cachePath(cacheDir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}

// CacheAge returns the age of a cache file, or -1 if it doesn't exist.
func CacheAge(cacheDir, name string) time.Duration {
	info, err := os.Stat(cachePath(cacheDir, name))
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime())
}

// SearchCacheKey returns a deterministic cache name for a search query + filters.
func SearchCacheKey(query string, filters SearchFilters) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%d",
		query, filters.Category, filters.Product, filters.LoB,
		filters.Industry, filters.FocusTags, filters.Partners, filters.Top)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("search-%x", h[:8])
}

func cachePath(cacheDir, name string) string {
	return filepath.Join(cacheDir, "discovery", name+".json")
}
