package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var rePackID = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func validPackID(id string) bool {
	return rePackID.MatchString(id)
}

// WizardState holds all answers collected during the creation wizard.
type WizardState struct {
	Layer    Layer
	PackDir  string
	Metadata map[string]any
	// SelectedFiles lists content filenames chosen by the user (e.g. "resources.yaml", "tips.md").
	SelectedFiles []string
	// Entries maps a YAML filename to the single initial entry data collected via BuildForm.
	Entries map[string]map[string]any
}

const contextTemplate = `### Overview

<!-- TODO: Describe what this pack covers -->

### Key Concepts

<!-- TODO: List the essential concepts -->

### Best Practices

<!-- TODO: Add best practices -->
`

const tipsTemplate = `## Tip title here

Tip content here.
`

const constraintsTemplate = `1. First constraint here.
`

// WriteFiles creates the pack directory and writes all files.
func (s *WizardState) WriteFiles() error {
	if err := os.MkdirAll(s.PackDir, 0755); err != nil {
		return fmt.Errorf("create pack directory: %w", err)
	}

	// 1. pack.yaml via SaveObject
	packPath := filepath.Join(s.PackDir, "pack.yaml")
	if err := SaveObject(packPath, s.Metadata); err != nil {
		return fmt.Errorf("write pack.yaml: %w", err)
	}

	// 2. context.md
	contextPath := filepath.Join(s.PackDir, "context.md")
	if err := os.WriteFile(contextPath, []byte(contextTemplate), 0644); err != nil {
		return fmt.Errorf("write context.md: %w", err)
	}

	// 3. Selected content files
	for _, filename := range s.SelectedFiles {
		filePath := filepath.Join(s.PackDir, filename)

		switch filename {
		case "tips.md":
			if err := os.WriteFile(filePath, []byte(tipsTemplate), 0644); err != nil {
				return fmt.Errorf("write %s: %w", filename, err)
			}
		case "constraints.md":
			if err := os.WriteFile(filePath, []byte(constraintsTemplate), 0644); err != nil {
				return fmt.Errorf("write %s: %w", filename, err)
			}
		default:
			// YAML array file: marshal entry or empty array
			var items []map[string]any
			if entry, ok := s.Entries[filename]; ok {
				items = append(items, entry)
			}
			data, err := yaml.Marshal(items)
			if err != nil {
				return fmt.Errorf("marshal %s: %w", filename, err)
			}
			if err := os.WriteFile(filePath, data, 0644); err != nil {
				return fmt.Errorf("write %s: %w", filename, err)
			}
		}
	}

	return nil
}

// Summary returns a human-readable description of what will be created.
func (s *WizardState) Summary() string {
	packID, _ := s.Metadata["id"].(string)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Creating pack %q in %s layer:\n\n", packID, s.Layer))
	sb.WriteString(fmt.Sprintf("  %s/\n", s.PackDir))
	sb.WriteString("    pack.yaml\n")
	sb.WriteString("    context.md\n")

	for _, filename := range s.SelectedFiles {
		if entry, ok := s.Entries[filename]; ok && len(entry) > 0 {
			sb.WriteString(fmt.Sprintf("    %s (1 entry)\n", filename))
		} else {
			sb.WriteString(fmt.Sprintf("    %s\n", filename))
		}
	}

	return sb.String()
}

func checkPackConflict(packDir string) error {
	if _, err := os.Stat(packDir); err == nil {
		return fmt.Errorf("pack directory already exists: %s\nUse 'sap-devs content edit' to modify existing packs", packDir)
	}
	return nil
}
