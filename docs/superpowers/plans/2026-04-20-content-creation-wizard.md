# Content Creation Wizard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs content create` — a guided TUI wizard that scaffolds a new content pack from scratch with pack.yaml, context.md, and optional content YAML/markdown files with initial entries.

**Architecture:** Two new files: `cmd/content_create.go` (cobra command) and `internal/editor/wizard.go` (wizard orchestration). The wizard collects all data in memory via huh v2 forms, then batch-writes after confirmation. Pack metadata uses hand-built forms; initial entries reuse the existing `BuildForm()` from `internal/editor/form.go`.

**Tech Stack:** Go, cobra, huh v2, gopkg.in/yaml.v3

**Spec:** `docs/superpowers/specs/2026-04-20-content-creation-wizard-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| Create: `internal/editor/wizard.go` | WizardState struct, layer resolution form, pack metadata forms, content file selection, initial entry collection, summary display, batch write. All wizard orchestration lives here. |
| Create: `internal/editor/wizard_test.go` | Unit tests for WizardState.WriteFiles(), pack ID validation, conflict detection, metadata map construction |
| Create: `cmd/content_create.go` | Cobra command definition, calls `findSchemasDir(cwd)` then `editor.RunCreateWizard(cwd, schemasDir)` |
| Modify: `docs/TODO.md` | Mark Phase 2c done |
| Modify: `CLAUDE.md` | Update content command description to mention `content create` |

---

### Task 1: WizardState struct + pack ID validation + WriteFiles

**Files:**
- Create: `internal/editor/wizard.go`
- Create: `internal/editor/wizard_test.go`

This task builds the testable core: the data struct that holds all wizard answers and the batch-write logic that creates files on disk. No interactive forms yet — just the data model and file-writing function.

- [ ] **Step 1: Write failing test for pack ID validation**

Create `internal/editor/wizard_test.go`:

```go
package editor

import (
	"strings"
	"testing"
)

func TestValidPackID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"cap", true},
		{"my-pack", true},
		{"btp-core", true},
		{"a1b2", true},
		{"x", true},
		{"", false},
		{"Cap", false},       // uppercase
		{"1pack", false},     // starts with digit
		{"-pack", false},     // starts with dash
		{"my_pack", false},   // underscore
		{"my pack", false},   // space
		{"MY-PACK", false},   // all uppercase
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := validPackID(tt.id)
			if got != tt.valid {
				t.Errorf("validPackID(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/editor/ -run TestValidPackID -v`
Expected: FAIL — `validPackID` undefined

- [ ] **Step 3: Write pack ID validator + WizardState struct**

Create `internal/editor/wizard.go` with the struct definition and validator:

```go
package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/editor/ -run TestValidPackID -v`
Expected: PASS

- [ ] **Step 5: Write failing test for WriteFiles**

Add to `internal/editor/wizard_test.go`:

```go
func TestWizardState_WriteFiles(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "my-pack")

	state := &WizardState{
		Layer:   LayerUser,
		PackDir: packDir,
		Metadata: map[string]any{
			"id":          "my-pack",
			"name":        "My Pack",
			"description": "A test pack",
			"tags":        []any{"test", "demo"},
			"weight":      50,
		},
		SelectedFiles: []string{"resources.yaml", "tips.md"},
		Entries: map[string]map[string]any{
			"resources.yaml": {
				"id":   "res-1",
				"name": "Test Resource",
				"url":  "https://example.com",
			},
		},
	}

	if err := state.WriteFiles(); err != nil {
		t.Fatalf("WriteFiles() error: %v", err)
	}

	// Verify pack.yaml exists and is valid YAML
	packYAML, err := os.ReadFile(filepath.Join(packDir, "pack.yaml"))
	if err != nil {
		t.Fatalf("pack.yaml not written: %v", err)
	}
	var packData map[string]any
	if err := yaml.Unmarshal(packYAML, &packData); err != nil {
		t.Fatalf("pack.yaml invalid YAML: %v", err)
	}
	if packData["id"] != "my-pack" {
		t.Errorf("pack.yaml id = %v, want my-pack", packData["id"])
	}

	// Verify context.md exists with standard sections
	contextMD, err := os.ReadFile(filepath.Join(packDir, "context.md"))
	if err != nil {
		t.Fatalf("context.md not written: %v", err)
	}
	contextStr := string(contextMD)
	for _, section := range []string{"### Overview", "### Key Concepts", "### Best Practices"} {
		if !containsStr(contextStr, section) {
			t.Errorf("context.md missing section %q", section)
		}
	}

	// Verify resources.yaml has one entry
	resYAML, err := os.ReadFile(filepath.Join(packDir, "resources.yaml"))
	if err != nil {
		t.Fatalf("resources.yaml not written: %v", err)
	}
	var resData []map[string]any
	if err := yaml.Unmarshal(resYAML, &resData); err != nil {
		t.Fatalf("resources.yaml invalid YAML: %v", err)
	}
	if len(resData) != 1 {
		t.Errorf("resources.yaml has %d entries, want 1", len(resData))
	}

	// Verify tips.md has placeholder template
	tipsMD, err := os.ReadFile(filepath.Join(packDir, "tips.md"))
	if err != nil {
		t.Fatalf("tips.md not written: %v", err)
	}
	if !containsStr(string(tipsMD), "## Tip title here") {
		t.Error("tips.md missing placeholder template")
	}
}

func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/editor/ -run TestWizardState_WriteFiles -v`
Expected: FAIL — `WriteFiles` method undefined

- [ ] **Step 7: Implement WriteFiles**

Add to `internal/editor/wizard.go`:

```go
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
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/editor/ -run TestWizardState_WriteFiles -v`
Expected: PASS

- [ ] **Step 9: Write failing test for conflict detection**

Add to `internal/editor/wizard_test.go`:

```go
func TestCheckPackConflict(t *testing.T) {
	dir := t.TempDir()

	// No conflict — directory doesn't exist
	err := checkPackConflict(filepath.Join(dir, "new-pack"))
	if err != nil {
		t.Errorf("expected no conflict, got: %v", err)
	}

	// Create existing pack directory
	existing := filepath.Join(dir, "existing-pack")
	os.MkdirAll(existing, 0755)

	err = checkPackConflict(existing)
	if err == nil {
		t.Error("expected conflict error, got nil")
	}
}
```

- [ ] **Step 10: Run test to verify it fails**

Run: `go test ./internal/editor/ -run TestCheckPackConflict -v`
Expected: FAIL — `checkPackConflict` undefined

- [ ] **Step 11: Implement conflict detection**

Add to `internal/editor/wizard.go`:

```go
func checkPackConflict(packDir string) error {
	if _, err := os.Stat(packDir); err == nil {
		return fmt.Errorf("pack directory already exists: %s\nUse 'sap-devs content edit' to modify existing packs", packDir)
	}
	return nil
}
```

- [ ] **Step 12: Run test to verify it passes**

Run: `go test ./internal/editor/ -run TestCheckPackConflict -v`
Expected: PASS

- [ ] **Step 13: Write failing test for empty YAML files (no entry)**

Add to `internal/editor/wizard_test.go`:

```go
func TestWizardState_WriteFiles_EmptyYAML(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "empty-pack")

	state := &WizardState{
		Layer:   LayerUser,
		PackDir: packDir,
		Metadata: map[string]any{
			"id":          "empty-pack",
			"name":        "Empty Pack",
			"description": "No entries",
			"tags":        []any{"test"},
		},
		SelectedFiles: []string{"tools.yaml"},
		Entries:       map[string]map[string]any{},
	}

	if err := state.WriteFiles(); err != nil {
		t.Fatalf("WriteFiles() error: %v", err)
	}

	toolsYAML, err := os.ReadFile(filepath.Join(packDir, "tools.yaml"))
	if err != nil {
		t.Fatalf("tools.yaml not written: %v", err)
	}
	var toolsData []map[string]any
	if err := yaml.Unmarshal(toolsYAML, &toolsData); err != nil {
		t.Fatalf("tools.yaml invalid YAML: %v", err)
	}
	if len(toolsData) != 0 {
		t.Errorf("tools.yaml has %d entries, want 0", len(toolsData))
	}
}
```

- [ ] **Step 14: Run test to verify it passes (already covered by WriteFiles implementation)**

Run: `go test ./internal/editor/ -run TestWizardState_WriteFiles_EmptyYAML -v`
Expected: PASS (empty `items` slice marshals to `[]`)

- [ ] **Step 15: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build, no errors

- [ ] **Step 16: Commit**

```bash
git add internal/editor/wizard.go internal/editor/wizard_test.go
git commit -m "feat(wizard): add WizardState struct, pack ID validation, WriteFiles, conflict detection"
```

---

### Task 2: Layer detection form + available layers logic

**Files:**
- Modify: `internal/editor/wizard.go`

This task adds the first interactive step: auto-detecting the layer and letting the user override it with a huh select. It also adds the `resolvePackDir` helper that computes the pack directory path from a layer + pack ID.

- [ ] **Step 1: Write failing test for resolvePackDir**

Add to `internal/editor/wizard_test.go`:

```go
func TestResolvePackDir(t *testing.T) {
	tests := []struct {
		layer  Layer
		cwd    string
		packID string
		expect string
	}{
		{LayerOfficial, "/repo", "my-pack", filepath.Join("/repo", "content", "packs", "my-pack")},
		{LayerCompany, "/company", "my-pack", filepath.Join("/company", "content", "packs", "my-pack")},
		{LayerProject, "/project", "my-pack", filepath.Join("/project", ".sap-devs", "packs", "my-pack")},
	}
	for _, tt := range tests {
		t.Run(tt.layer.String(), func(t *testing.T) {
			got, err := resolvePackDir(tt.layer, tt.cwd, tt.packID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expect {
				t.Errorf("resolvePackDir(%v, %q, %q) = %q, want %q", tt.layer, tt.cwd, tt.packID, got, tt.expect)
			}
		})
	}
}
```

Note: The `LayerUser` case depends on `xdg.New()` which varies by platform, so we test only the deterministic cases.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/editor/ -run TestResolvePackDir -v`
Expected: FAIL — `resolvePackDir` undefined

- [ ] **Step 3: Implement resolvePackDir and layer form**

Add to `internal/editor/wizard.go`:

```go
import (
	"charm.land/huh/v2"
	"github.com/SAP-samples/sap-devs-cli/internal/theme"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

func resolvePackDir(layer Layer, cwd, packID string) (string, error) {
	switch layer {
	case LayerOfficial, LayerCompany:
		return filepath.Join(cwd, "content", "packs", packID), nil
	case LayerProject:
		return filepath.Join(cwd, ".sap-devs", "packs", packID), nil
	case LayerUser:
		paths, err := xdg.New()
		if err != nil {
			return "", fmt.Errorf("cannot resolve user data directory: %w", err)
		}
		return filepath.Join(paths.DataDir, "packs", packID), nil
	}
	return "", fmt.Errorf("unknown layer: %v", layer)
}

// availableLayers returns the layer options available for the current CWD.
// "user" and "project" are always available. "official" and "company" are
// only offered when the CWD is detected as the corresponding repo checkout.
func availableLayers(cwd string) []Layer {
	var layers []Layer
	if _, err := os.Stat(filepath.Join(cwd, "content", "packs")); err == nil {
		if isOfficialRepo(cwd) {
			layers = append(layers, LayerOfficial)
		}
		if isCompanyRepo(cwd) {
			layers = append(layers, LayerCompany)
		}
	}
	layers = append(layers, LayerUser, LayerProject)
	return layers
}

func runLayerForm(cwd string) (Layer, error) {
	detected, _ := detectLayer(cwd)
	available := availableLayers(cwd)

	layerStr := detected.String()
	opts := make([]huh.Option[string], 0, len(available))
	for _, l := range available {
		opts = append(opts, huh.NewOption(l.String(), l.String()))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Content layer").
				Description("Where should the new pack be created?").
				Options(opts...).
				Value(&layerStr),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := form.Run(); err != nil {
		return 0, err
	}

	switch layerStr {
	case "official":
		return LayerOfficial, nil
	case "company":
		return LayerCompany, nil
	case "user":
		return LayerUser, nil
	case "project":
		return LayerProject, nil
	}
	return LayerUser, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/editor/ -run TestResolvePackDir -v`
Expected: PASS

- [ ] **Step 5: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 6: Commit**

```bash
git add internal/editor/wizard.go internal/editor/wizard_test.go
git commit -m "feat(wizard): add layer detection form with available layers logic"
```

---

### Task 3: Pack metadata form (two-phase with conditional additive_position)

**Files:**
- Modify: `internal/editor/wizard.go`
- Modify: `internal/editor/wizard_test.go`

This task builds the hand-crafted pack metadata form. It collects id, name, description, tags, weight, and additive in the first phase. If additive is true, a second form collects additive_position. The result is a `map[string]any` ready for pack.yaml.

- [ ] **Step 1: Write failing test for buildMetadataMap**

Add to `internal/editor/wizard_test.go`:

```go
func TestBuildMetadataMap(t *testing.T) {
	t.Run("non-additive", func(t *testing.T) {
		m := buildMetadataMap("my-pack", "My Pack", "A description", "cap, btp", "50", false, "")
		if m["id"] != "my-pack" {
			t.Errorf("id = %v", m["id"])
		}
		tags, ok := m["tags"].([]any)
		if !ok || len(tags) != 2 {
			t.Errorf("tags = %v", m["tags"])
		}
		if m["weight"] != 50 {
			t.Errorf("weight = %v (type %T)", m["weight"], m["weight"])
		}
		if _, hasPos := m["additive_position"]; hasPos {
			t.Error("additive_position should be omitted when additive is false")
		}
	})

	t.Run("additive", func(t *testing.T) {
		m := buildMetadataMap("my-pack", "My Pack", "A description", "cap", "50", true, "before")
		if m["additive"] != true {
			t.Errorf("additive = %v", m["additive"])
		}
		if m["additive_position"] != "before" {
			t.Errorf("additive_position = %v", m["additive_position"])
		}
	})

	t.Run("default weight", func(t *testing.T) {
		m := buildMetadataMap("my-pack", "My Pack", "Desc", "tag", "", false, "")
		if m["weight"] != 50 {
			t.Errorf("weight = %v, want 50 (default)", m["weight"])
		}
	})

	t.Run("custom weight", func(t *testing.T) {
		m := buildMetadataMap("my-pack", "My Pack", "Desc", "tag", "100", false, "")
		if m["weight"] != 100 {
			t.Errorf("weight = %v, want 100", m["weight"])
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/editor/ -run TestBuildMetadataMap -v`
Expected: FAIL — `buildMetadataMap` undefined

- [ ] **Step 3: Implement buildMetadataMap**

Add to `internal/editor/wizard.go`:

```go
import "strconv"

func buildMetadataMap(id, name, description, tagsRaw, weightRaw string, additive bool, additivePosition string) map[string]any {
	tags := splitTags(tagsRaw)
	anyTags := make([]any, len(tags))
	for i, t := range tags {
		anyTags[i] = t
	}

	weight := 50
	if weightRaw != "" {
		if n, err := strconv.Atoi(weightRaw); err == nil {
			weight = n
		}
	}

	m := map[string]any{
		"id":          id,
		"name":        name,
		"description": description,
		"tags":        anyTags,
		"weight":      weight,
	}

	if additive {
		m["additive"] = true
		if additivePosition != "" {
			m["additive_position"] = additivePosition
		} else {
			m["additive_position"] = "after"
		}
	}

	return m
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/editor/ -run TestBuildMetadataMap -v`
Expected: PASS

- [ ] **Step 5: Implement the interactive metadata form functions**

Add to `internal/editor/wizard.go`:

```go
import "strings"

// metadataFormResult holds the raw form field values from the pack metadata form.
type metadataFormResult struct {
	ID          string
	Name        string
	Description string
	TagsRaw     string
	WeightRaw   string
	Additive    bool
}

func runMetadataForm() (*metadataFormResult, error) {
	r := &metadataFormResult{WeightRaw: "50"}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Pack ID *").
				Placeholder("my-pack").
				Value(&r.ID).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("required")
					}
					if !validPackID(s) {
						return fmt.Errorf("must match ^[a-z][a-z0-9-]*$")
					}
					return nil
				}),
			huh.NewInput().
				Title("Name *").
				Placeholder("My Content Pack").
				Value(&r.Name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Description *").
				Placeholder("A brief description of this pack").
				Value(&r.Description).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Tags *").
				Placeholder("tag1, tag2, tag3").
				Value(&r.TagsRaw).
				Validate(func(s string) error {
					parts := splitTags(s)
					if len(parts) == 0 {
						return fmt.Errorf("at least one tag required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Weight").
				Placeholder("50").
				Value(&r.WeightRaw).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					if _, err := strconv.Atoi(s); err != nil {
						return fmt.Errorf("must be an integer")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("Additive").
				Description("Augment same-ID pack from a lower layer instead of replacing it?").
				Value(&r.Additive),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := form.Run(); err != nil {
		return nil, err
	}
	return r, nil
}

func runAdditivePositionForm() (string, error) {
	position := "after"
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Additive position").
				Description("Where should additive content appear relative to the base pack?").
				Options(
					huh.NewOption("after", "after"),
					huh.NewOption("before", "before"),
				).
				Value(&position),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := form.Run(); err != nil {
		return "", err
	}
	return position, nil
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 7: Commit**

```bash
git add internal/editor/wizard.go internal/editor/wizard_test.go
git commit -m "feat(wizard): add pack metadata form with two-phase additive handling"
```

---

### Task 4: Content file selection form

**Files:**
- Modify: `internal/editor/wizard.go`

This task adds the huh multi-select form that lets the user choose which content files to scaffold. Also defines the `contentFileOption` struct for the available files list.

- [ ] **Step 1: Implement content file selection form**

Add to `internal/editor/wizard.go`:

```go
type contentFileOption struct {
	Filename    string
	Description string
}

var defaultContentFiles = []contentFileOption{
	{"resources.yaml", "Curated links and documentation"},
	{"tools.yaml", "Required/recommended developer tools"},
	{"mcp.yaml", "MCP server definitions"},
	{"samples.yaml", "Canonical code sample references"},
	{"known_errors.yaml", "Common error patterns with fixes"},
	{"tips.md", "Developer tips (H2-delimited)"},
	{"constraints.md", "Behavioral rules for AI agents"},
}

func runContentFileForm() ([]string, error) {
	var selected []string

	opts := make([]huh.Option[string], 0, len(defaultContentFiles))
	for _, f := range defaultContentFiles {
		opts = append(opts, huh.NewOption(
			fmt.Sprintf("%s — %s", f.Filename, f.Description),
			f.Filename,
		))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Content files to scaffold").
				Description("Select files to include in the pack (Space to toggle, Enter to confirm)").
				Options(opts...).
				Value(&selected),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := form.Run(); err != nil {
		return nil, err
	}
	return selected, nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add internal/editor/wizard.go
git commit -m "feat(wizard): add content file selection multi-select form"
```

---

### Task 5: Initial entry collection via BuildForm

**Files:**
- Modify: `internal/editor/wizard.go`

This task adds the logic for collecting one initial entry per selected YAML file using the existing `BuildForm()` infrastructure. For each schema-backed YAML file, it loads the schema, builds a form, and collects the entry. The user can press Esc to skip a file (empty array, no entry).

- [ ] **Step 1: Implement initial entry collection**

Add to `internal/editor/wizard.go`:

```go
import (
	"errors"

	"charm.land/huh/v2"
	"github.com/SAP-samples/sap-devs-cli/internal/schema"
)

// isMarkdownFile returns true for content files that are markdown (not YAML).
func isMarkdownFile(filename string) bool {
	return filename == "tips.md" || filename == "constraints.md"
}

// collectInitialEntries runs BuildForm for each selected YAML file and collects
// one entry per file. Returns a map of filename -> entry data.
// Esc/abort on a single file skips that entry (file still scaffolded as empty array).
func collectInitialEntries(schemasDir string, selectedFiles []string) (map[string]map[string]any, error) {
	entries := make(map[string]map[string]any)

	for _, filename := range selectedFiles {
		if isMarkdownFile(filename) {
			continue
		}

		schemaName, ok := schema.SchemaForFile(filename)
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: no schema found for %s, skipping initial entry\n", filename)
			continue
		}

		s, err := schema.Load(schemasDir, schemaName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot load schema for %s: %v\n", filename, err)
			continue
		}

		if s.ItemSpec == nil {
			continue
		}

		fmt.Printf("\n  Initial entry for %s (Esc to skip):\n\n", filename)

		form, bindings := BuildForm(s.ItemSpec, make(map[string]any))
		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				continue
			}
			return nil, err
		}

		entry := bindings.ToMap(s.ItemSpec)
		entries[filename] = entry
	}

	return entries, nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add internal/editor/wizard.go
git commit -m "feat(wizard): add initial entry collection via BuildForm"
```

---

### Task 6: Summary display + confirmation form

**Files:**
- Modify: `internal/editor/wizard.go`
- Modify: `internal/editor/wizard_test.go`

This task adds the summary display (showing what will be created) and the final confirmation prompt.

- [ ] **Step 1: Write failing test for summary string generation**

Add to `internal/editor/wizard_test.go`:

```go
func TestWizardState_Summary(t *testing.T) {
	state := &WizardState{
		Layer:   LayerUser,
		PackDir: "/home/user/.local/share/sap-devs/packs/my-pack",
		Metadata: map[string]any{
			"id": "my-pack",
		},
		SelectedFiles: []string{"resources.yaml", "tips.md"},
		Entries: map[string]map[string]any{
			"resources.yaml": {"id": "res-1"},
		},
	}

	summary := state.Summary()
	if !containsStr(summary, "my-pack") {
		t.Error("summary missing pack ID")
	}
	if !containsStr(summary, "user") {
		t.Error("summary missing layer name")
	}
	if !containsStr(summary, "pack.yaml") {
		t.Error("summary missing pack.yaml")
	}
	if !containsStr(summary, "context.md") {
		t.Error("summary missing context.md")
	}
	if !containsStr(summary, "resources.yaml (1 entry)") {
		t.Error("summary missing resources.yaml entry count")
	}
	if !containsStr(summary, "tips.md") {
		t.Error("summary missing tips.md")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/editor/ -run TestWizardState_Summary -v`
Expected: FAIL — `Summary` method undefined

- [ ] **Step 3: Implement Summary and confirmation**

Add to `internal/editor/wizard.go`:

```go
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

func runConfirmForm(summary string) (bool, error) {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Proceed?").
				Description(summary).
				Affirmative("Create").
				Negative("Cancel").
				Value(&confirmed),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/editor/ -run TestWizardState_Summary -v`
Expected: PASS

- [ ] **Step 5: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 6: Commit**

```bash
git add internal/editor/wizard.go internal/editor/wizard_test.go
git commit -m "feat(wizard): add summary display and confirmation form"
```

---

### Task 7: RunCreateWizard orchestration

**Files:**
- Modify: `internal/editor/wizard.go`

This task wires all the individual steps into the main `RunCreateWizard` entry point function, called by the cobra command.

- [ ] **Step 1: Implement RunCreateWizard**

Add to `internal/editor/wizard.go`:

```go
// RunCreateWizard runs the full content pack creation wizard.
func RunCreateWizard(cwd, schemasDir string) error {
	// Step 1: Layer selection
	layer, err := runLayerForm(cwd)
	if err != nil {
		return err
	}

	// Step 2: Pack metadata
	meta, err := runMetadataForm()
	if err != nil {
		return err
	}

	// Resolve pack directory and check for conflicts
	packDir, err := resolvePackDir(layer, cwd, meta.ID)
	if err != nil {
		return err
	}
	if err := checkPackConflict(packDir); err != nil {
		return err
	}

	// Build additive_position if needed
	var additivePosition string
	if meta.Additive {
		pos, err := runAdditivePositionForm()
		if err != nil {
			return err
		}
		additivePosition = pos
	}

	metadata := buildMetadataMap(
		meta.ID, meta.Name, meta.Description,
		meta.TagsRaw, meta.WeightRaw,
		meta.Additive, additivePosition,
	)

	// Step 4: Content file selection
	selectedFiles, err := runContentFileForm()
	if err != nil {
		return err
	}

	// Step 5: Initial entries for selected YAML files
	entries, err := collectInitialEntries(schemasDir, selectedFiles)
	if err != nil {
		return err
	}

	// Assemble state
	state := &WizardState{
		Layer:         layer,
		PackDir:       packDir,
		Metadata:      metadata,
		SelectedFiles: selectedFiles,
		Entries:       entries,
	}

	// Step 6: Summary and confirmation
	confirmed, err := runConfirmForm(state.Summary())
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Aborted — no files written.")
		return nil
	}

	// Write all files
	if err := state.WriteFiles(); err != nil {
		return err
	}

	packID, _ := metadata["id"].(string)
	fmt.Printf("\nPack %q created at %s\n", packID, packDir)
	fmt.Printf("Edit with: sap-devs content edit %s/pack.yaml\n", packID)
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add internal/editor/wizard.go
git commit -m "feat(wizard): add RunCreateWizard orchestration"
```

---

### Task 8: Cobra command wiring

**Files:**
- Create: `cmd/content_create.go`

This task creates the cobra command that wires `sap-devs content create` to `editor.RunCreateWizard`.

- [ ] **Step 1: Create cmd/content_create.go**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/editor"
)

var contentCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new content pack",
	Long:  "Guided wizard to scaffold a new content pack with pack.yaml, context.md, and optional content files.",
	RunE:  runContentCreate,
}

func init() {
	contentCmd.AddCommand(contentCreateCmd)
}

func runContentCreate(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	schemasDir, err := findSchemasDir(cwd)
	if err != nil {
		return fmt.Errorf("cannot locate schemas directory: %w", err)
	}

	return editor.RunCreateWizard(cwd, schemasDir)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build — `sap-devs content create` now registered

- [ ] **Step 3: Verify command appears in help**

Run: `go run . content --help`
Expected: Output includes `create` in the available commands list

- [ ] **Step 4: Commit**

```bash
git add cmd/content_create.go
git commit -m "feat: wire sap-devs content create command"
```

---

### Task 9: Manual integration test

**Files:** None (testing only)

Run the wizard end-to-end in a temporary directory to verify the complete flow.

- [ ] **Step 1: Run the wizard**

```bash
cd /tmp && mkdir wizard-test && cd wizard-test
SAP_DEVS_DEV=1 go run /path/to/worktree content create
```

Exercise the flow:
1. Layer form → select "user"
2. Pack metadata → enter id "test-pack", name, description, tags "test, demo", weight 50, additive false
3. Content files → select resources.yaml and tips.md
4. Initial entry for resources.yaml → fill in one entry
5. Summary → confirm
6. Verify files exist in the user layer directory

- [ ] **Step 2: Verify the created files**

Check that:
- `pack.yaml` has correct fields, no `additive_position` key
- `context.md` has the three standard sections
- `resources.yaml` has one entry as a YAML array
- `tips.md` has the placeholder template

- [ ] **Step 3: Run the wizard again with the same pack ID**

Expected: conflict detection error — "pack directory already exists"

- [ ] **Step 4: Test additive flow**

Run the wizard again with a new pack ID, set additive=true, verify the second form appears for additive_position, and verify `pack.yaml` contains both `additive: true` and `additive_position`.

- [ ] **Step 5: Test Esc handling**

Run the wizard, press Esc during the initial entry form for a YAML file, verify the file is still scaffolded as an empty array.

---

### Task 10: Documentation updates

**Files:**
- Modify: `docs/TODO.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update TODO.md**

Mark the Phase 2c item as done:
```
- Content creation wizard — guided flow for adding a new pack from scratch - DONE
```

- [ ] **Step 2: Update CLAUDE.md**

Update the `content` command description in the CLI Commands table to mention `content create`:
```
| `content` | Manage content YAML files; `content edit/validate/list/create` with `--pack`/`--layer`/`--json` filtering; edit includes undo/redo, pre-save diff review, Shift+J/K reordering, and bulk editing; create scaffolds a new pack via guided wizard |
```

- [ ] **Step 3: Verify build one final time**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add docs/TODO.md CLAUDE.md
git commit -m "docs: update TODO.md and CLAUDE.md for content creation wizard"
```
