package videos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

// LoadCache reads cached videos for a given pack and source.
func LoadCache(cacheDir, packID, sourceID string) ([]content.Video, error) {
	data, err := os.ReadFile(cachePath(cacheDir, packID, sourceID))
	if err != nil {
		return nil, err
	}
	var vids []content.Video
	if err := json.Unmarshal(data, &vids); err != nil {
		return nil, err
	}
	return vids, nil
}

// SaveCache writes videos to the cache file for a given pack and source.
func SaveCache(cacheDir, packID, sourceID string, vids []content.Video) error {
	dir := filepath.Join(cacheDir, "youtube", packID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(vids)
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(cacheDir, packID, sourceID), data, 0644)
}

// CacheAge returns the age of the cache file, or -1 if it doesn't exist.
func CacheAge(cacheDir, packID, sourceID string) time.Duration {
	info, err := os.Stat(cachePath(cacheDir, packID, sourceID))
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime())
}

func cachePath(cacheDir, packID, sourceID string) string {
	return filepath.Join(cacheDir, "youtube", packID, sourceID+".json")
}
