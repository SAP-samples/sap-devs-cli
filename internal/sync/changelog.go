package sync

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ChangelogEntry is a single human-curated change note from a pack.
type ChangelogEntry struct {
	Pack string `json:"pack"`
	Text string `json:"text"`
}

type changelogFile struct {
	SyncedAt time.Time        `json:"synced_at"`
	Entries  []ChangelogEntry `json:"entries"`
}

type changelogMeta struct {
	ID        string   `yaml:"id"`
	Changelog []string `yaml:"changelog"`
}

const changelogFilename = "sync-changelog.json"

// WriteChangelog writes changelog entries to sync-changelog.json in cacheDir.
// Returns nil without writing if entries is empty.
func WriteChangelog(cacheDir string, syncedAt time.Time, entries []ChangelogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	cf := changelogFile{SyncedAt: syncedAt, Entries: entries}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, changelogFilename), data, 0600)
}

// ReadChangelog reads sync-changelog.json from cacheDir.
// Returns nil entries and zero time if the file is missing.
func ReadChangelog(cacheDir string) ([]ChangelogEntry, time.Time, error) {
	data, err := os.ReadFile(filepath.Join(cacheDir, changelogFilename))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, err
	}
	var cf changelogFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, time.Time{}, err
	}
	return cf.Entries, cf.SyncedAt, nil
}

// ConsumeChangelog deletes sync-changelog.json from cacheDir.
// No-op if the file does not exist.
func ConsumeChangelog(cacheDir string) error {
	err := os.Remove(filepath.Join(cacheDir, changelogFilename))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// CollectChangelog scans pack.yaml files across one or more pack directories
// and extracts changelog entries. Directories that don't exist are silently skipped.
func CollectChangelog(packsDirs []string) ([]ChangelogEntry, error) {
	var all []ChangelogEntry
	for _, dir := range packsDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			packYAML := filepath.Join(dir, entry.Name(), "pack.yaml")
			data, err := os.ReadFile(packYAML)
			if err != nil {
				continue
			}
			var meta changelogMeta
			if err := yaml.Unmarshal(data, &meta); err != nil {
				continue
			}
			for _, text := range meta.Changelog {
				if text != "" {
					all = append(all, ChangelogEntry{Pack: meta.ID, Text: text})
				}
			}
		}
	}
	return all, nil
}
