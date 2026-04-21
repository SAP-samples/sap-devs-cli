package news

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const cacheFile = "news-cache.json"

func cachePath(cacheDir string) string {
	return filepath.Join(cacheDir, "news", cacheFile)
}

// SaveCache writes correlated news items to disk.
func SaveCache(cacheDir string, items []NewsItem) error {
	dir := filepath.Join(cacheDir, "news")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	b, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(cacheDir), b, 0644)
}

// LoadCache reads cached news items if the cache exists and is fresher than ttl.
func LoadCache(cacheDir string, ttl time.Duration) ([]NewsItem, bool) {
	p := cachePath(cacheDir)
	info, err := os.Stat(p)
	if err != nil {
		return nil, false
	}
	if ttl > 0 && time.Since(info.ModTime()) > ttl {
		return nil, false
	}
	return readCache(p)
}

// LoadCacheStale reads cached news items ignoring TTL.
// Used as fallback when live fetches fail.
func LoadCacheStale(cacheDir string) ([]NewsItem, bool) {
	return readCache(cachePath(cacheDir))
}

// CacheAge returns the age of the news cache, or -1 if it doesn't exist.
func CacheAge(cacheDir string) time.Duration {
	info, err := os.Stat(cachePath(cacheDir))
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime())
}

// LoadBaseline reads a pre-fetched news-episodes.json from the content pack.
// This file is committed by a GitHub Action and pulled during sync.
func LoadBaseline(officialCacheDir string) ([]NewsItem, bool) {
	p := filepath.Join(officialCacheDir, "content", "packs", "base", "news-episodes.json")
	return readCache(p)
}

func readCache(path string) ([]NewsItem, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var items []NewsItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, false
	}
	return items, true
}
