package tutorials

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func tutorialsDir(cacheDir string) string {
	return filepath.Join(cacheDir, "tutorials")
}

// SaveIndex writes the tutorial index to the cache.
func SaveIndex(cacheDir string, index []TutorialMeta) error {
	dir := tutorialsDir(cacheDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "index.json"), data, 0644)
}

// LoadIndex reads the tutorial index from the cache and deduplicates by slug.
func LoadIndex(cacheDir string) ([]TutorialMeta, error) {
	data, err := os.ReadFile(filepath.Join(tutorialsDir(cacheDir), "index.json"))
	if err != nil {
		return nil, err
	}
	var index []TutorialMeta
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	return deduplicateBySlug(index), nil
}

func deduplicateBySlug(index []TutorialMeta) []TutorialMeta {
	seen := make(map[string]bool, len(index))
	out := make([]TutorialMeta, 0, len(index))
	for _, m := range index {
		if seen[m.Slug] {
			continue
		}
		seen[m.Slug] = true
		out = append(out, m)
	}
	return out
}

// IndexCacheAge returns the age of the index cache file, or a negative duration if missing.
func IndexCacheAge(cacheDir string) time.Duration {
	info, err := os.Stat(filepath.Join(tutorialsDir(cacheDir), "index.json"))
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime())
}

// SaveContent writes a parsed tutorial to the content cache.
func SaveContent(cacheDir string, tut *Tutorial) error {
	dir := filepath.Join(tutorialsDir(cacheDir), "content")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(tut)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, tut.Slug+".json"), data, 0644)
}

// LoadContent reads a parsed tutorial from the content cache.
func LoadContent(cacheDir, slug string) (*Tutorial, error) {
	data, err := os.ReadFile(filepath.Join(tutorialsDir(cacheDir), "content", slug+".json"))
	if err != nil {
		return nil, err
	}
	var tut Tutorial
	return &tut, json.Unmarshal(data, &tut)
}

// SaveRepoInfo writes cached repo metadata.
func SaveRepoInfo(cacheDir string, repos []RepoInfo) error {
	dir := tutorialsDir(cacheDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(repos)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "repos.json"), data, 0644)
}

// LoadRepoInfo reads cached repo metadata.
func LoadRepoInfo(cacheDir string) ([]RepoInfo, error) {
	data, err := os.ReadFile(filepath.Join(tutorialsDir(cacheDir), "repos.json"))
	if err != nil {
		return nil, err
	}
	var repos []RepoInfo
	return repos, json.Unmarshal(data, &repos)
}
