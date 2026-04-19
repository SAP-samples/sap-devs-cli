package scratch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const scratchDir = ".sap-devs"
const scratchFile = "scratch.yaml"

type fileData struct {
	Notes []string `yaml:"notes"`
}

func scratchPath(dir string) string {
	return filepath.Join(dir, scratchDir, scratchFile)
}

func Load(dir string) ([]string, error) {
	data, err := os.ReadFile(scratchPath(dir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var f fileData
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", scratchFile, err)
	}
	return f.Notes, nil
}

func Add(dir, note string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return fmt.Errorf("note cannot be empty")
	}
	notes, err := Load(dir)
	if err != nil {
		return err
	}
	notes = append(notes, note)
	return write(dir, notes)
}

func Clear(dir string) error {
	err := os.Remove(scratchPath(dir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func HasNotes(dir string) bool {
	notes, err := Load(dir)
	return err == nil && len(notes) > 0
}

func write(dir string, notes []string) error {
	dirPath := filepath.Join(dir, scratchDir)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return err
	}
	f := fileData{Notes: notes}
	data, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	return os.WriteFile(scratchPath(dir), data, 0o644)
}
