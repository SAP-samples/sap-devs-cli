# sap-devs resources — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs resources list/search/open` so developers can browse and open curated SAP documentation, samples, and links from the terminal.

**Architecture:** Extend the existing `Resource` struct in `internal/content/pack.go` with a runtime `PackID` field set during `LoadPack`. Add three pure helper functions (`FlattenResources`, `FilterResources`, `FindResource`) in a new `internal/content/resources.go`. Wire them into a thin `cmd/resources.go` with three Cobra subcommands. No new internal packages — follows the same pattern as `tip` and `profile`.

**Tech Stack:** Go 1.26, github.com/spf13/cobra (existing), github.com/pkg/browser (new), gopkg.in/yaml.v3 (existing), github.com/stretchr/testify (existing)

> **Note on local testing:** `go test` is blocked by Windows Defender on this machine (test binaries land in `%TEMP%` on C:). Use `go build ./...` + `go vet ./...` for local verification. Tests run correctly in CI (`go test ./...` on ubuntu-latest).

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/content/pack.go` | Modify | Add `PackID string` to `Resource`; set it in `LoadPack` |
| `internal/content/resources.go` | Create | `FlattenResources`, `FilterResources`, `FindResource` |
| `internal/content/resources_test.go` | Create | Unit tests for all three helpers |
| `cmd/resources.go` | Create | `resources list/search/open` Cobra subcommands |
| `go.mod` / `go.sum` | Modify | Add `github.com/pkg/browser` |

---

## Task 1: Add PackID to Resource struct

**Files:**
- Modify: `internal/content/pack.go`

`PackID` is not in the YAML — it is a runtime field set by `LoadPack` so that later display code knows which pack a resource came from.

- [ ] **Step 1: Add `PackID` field to the `Resource` struct**

Open `internal/content/pack.go`. The `Resource` struct currently ends at line ~34. Add `PackID string` as the last field (no yaml tag — it is never serialised):

```go
// Resource is a curated link within a pack.
type Resource struct {
	ID       string   `yaml:"id"`
	Title    string   `yaml:"title"`
	URL      string   `yaml:"url"`
	Type     string   `yaml:"type"`
	Tags     []string `yaml:"tags"`
	Advocate string   `yaml:"advocate,omitempty"`
	PackID   string   // set at load time, not in YAML
}
```

- [ ] **Step 2: Set PackID in LoadPack after unmarshalling resources.yaml**

In `LoadPack`, the existing resources block is:

```go
if data, err := os.ReadFile(filepath.Join(packDir, "resources.yaml")); err == nil {
    _ = yaml.Unmarshal(data, &pack.Resources)
}
```

Change it to:

```go
if data, err := os.ReadFile(filepath.Join(packDir, "resources.yaml")); err == nil {
    _ = yaml.Unmarshal(data, &pack.Resources)
    for i := range pack.Resources {
        pack.Resources[i].PackID = pack.ID
    }
}
```

- [ ] **Step 3: Verify it builds**

Run: `go build ./...`
Expected: clean build, no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/content/pack.go
git commit -m "feat: add PackID runtime field to Resource, set in LoadPack"
```

---

## Task 2: Add resource helper functions (TDD)

**Files:**
- Create: `internal/content/resources_test.go`
- Create: `internal/content/resources.go`

### Step 1 — Write the tests first

- [ ] **Step 1: Create `internal/content/resources_test.go`**

```go
package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// fixture builds an in-memory pack slice for testing without touching the filesystem.
func fixturePacks() []*content.Pack {
	return []*content.Pack{
		{
			ID: "cap",
			Resources: []content.Resource{
				{ID: "cap/docs", Title: "CAP Documentation", URL: "https://cap.cloud.sap/docs", Type: "official-docs", Tags: []string{"reference"}, PackID: "cap"},
				{ID: "cap/samples", Title: "CAP Samples on GitHub", URL: "https://github.com/SAP-samples/cap", Type: "sample", Tags: []string{"examples", "reference"}, PackID: "cap"},
			},
		},
		{
			ID: "abap",
			Resources: []content.Resource{
				{ID: "abap/adt", Title: "ABAP Development Tools", URL: "https://tools.hana.ondemand.com", Type: "tool", Tags: []string{"ide"}, PackID: "abap"},
			},
		},
	}
}

func TestFlattenResources(t *testing.T) {
	got := content.FlattenResources(fixturePacks())
	require.Len(t, got, 3)
	assert.Equal(t, "cap/docs", got[0].ID)
	assert.Equal(t, "cap", got[0].PackID)
	assert.Equal(t, "cap/samples", got[1].ID)
	assert.Equal(t, "cap", got[1].PackID)
	assert.Equal(t, "abap/adt", got[2].ID)
	assert.Equal(t, "abap", got[2].PackID)
}

func TestFlattenResources_NilInput(t *testing.T) {
	got := content.FlattenResources(nil)
	assert.Empty(t, got)
}

func TestFilterResources_TitleMatch(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FilterResources(resources, "documentation")
	require.Len(t, got, 1)
	assert.Equal(t, "cap/docs", got[0].ID)
}

func TestFilterResources_TagMatch(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FilterResources(resources, "ide")
	require.Len(t, got, 1)
	assert.Equal(t, "abap/adt", got[0].ID)
}

func TestFilterResources_TypeMatch(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FilterResources(resources, "sample")
	require.Len(t, got, 1)
	assert.Equal(t, "cap/samples", got[0].ID)
}

func TestFilterResources_CaseInsensitive(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	// "CAP" matches "CAP Documentation" and "CAP Samples on GitHub"
	got := content.FilterResources(resources, "CAP")
	assert.Len(t, got, 2)
}

func TestFilterResources_NoMatch(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FilterResources(resources, "zzznomatch")
	assert.Empty(t, got)
}

func TestFindResource_Found(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FindResource(resources, "cap/samples")
	require.NotNil(t, got)
	assert.Equal(t, "CAP Samples on GitHub", got.Title)
}

func TestFindResource_NotFound(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FindResource(resources, "nonexistent/id")
	assert.Nil(t, got)
}

// TestLoadPackSetsPackID verifies that LoadPack populates PackID on each resource.
func TestLoadPackSetsPackID(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(`
id: mypak
name: My Pack
description: Test pack
`), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.yaml"), []byte(`
- id: mypak/link
  title: My Link
  url: https://example.com
  type: official-docs
  tags: [test]
`), 0644))

	pack, err := content.LoadPack(dir)
	require.NoError(t, err)
	require.Len(t, pack.Resources, 1)
	assert.Equal(t, "mypak", pack.Resources[0].PackID)
}
```

- [ ] **Step 2: Verify tests fail to compile (functions not yet defined)**

Run: `go build ./...`
Expected: compile error — `content.FlattenResources`, `content.FilterResources`, `content.FindResource` undefined.

- [ ] **Step 3: Create `internal/content/resources.go`**

```go
package content

import "strings"

// FlattenResources collects all resources from all packs into a single slice.
func FlattenResources(packs []*Pack) []Resource {
	var out []Resource
	for _, p := range packs {
		out = append(out, p.Resources...)
	}
	return out
}

// FilterResources returns resources whose title, type, or any tag contains query
// (case-insensitive substring match).
func FilterResources(resources []Resource, query string) []Resource {
	q := strings.ToLower(query)
	var out []Resource
	for _, r := range resources {
		if strings.Contains(strings.ToLower(r.Title), q) ||
			strings.Contains(strings.ToLower(r.Type), q) {
			out = append(out, r)
			continue
		}
		for _, tag := range r.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// FindResource returns a pointer to the first resource with an exact ID match, or nil.
func FindResource(resources []Resource, id string) *Resource {
	for i := range resources {
		if resources[i].ID == id {
			return &resources[i]
		}
	}
	return nil
}
```

- [ ] **Step 4: Verify it builds**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 5: Verify vet**

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 6: Commit**

```bash
git add internal/content/resources.go internal/content/resources_test.go
git commit -m "feat: add FlattenResources, FilterResources, FindResource helpers"
```

---

## Task 3: Add github.com/pkg/browser dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/pkg/browser`
Expected: `go.mod` and `go.sum` updated. The package will appear as a direct dependency.

- [ ] **Step 2: Tidy**

Run: `go mod tidy`
Expected: no changes needed beyond what `go get` did, or minor indirect dep additions.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add github.com/pkg/browser dependency"
```

---

## Task 4: Create cmd/resources.go

**Files:**
- Create: `cmd/resources.go`

Study `cmd/profile.go` for the subcommand registration pattern and `cmd/tip.go` for the profile-loading pattern before writing this file. The module path is `github.tools.sap/developer-relations/sap-devs-cli`.

- [ ] **Step 1: Create `cmd/resources.go`**

```go
package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var resourcesCmd = &cobra.Command{
	Use:   "resources",
	Short: "Browse curated SAP resources",
}

var resourcesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List curated resources for your active profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		profileCfg, err := config.LoadProfile(paths.ConfigDir)
		if err != nil {
			return err
		}
		if profileCfg.ID == "" {
			return fmt.Errorf("no profile set — run 'sap-devs profile set <name>' first")
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		activeProfile, err := loader.FindProfile(profileCfg.ID)
		if err != nil {
			return err
		}
		if activeProfile == nil {
			return fmt.Errorf("profile %q not found — run 'sap-devs sync' to refresh content", profileCfg.ID)
		}
		// Note: returning fmt.Errorf (not fmt.Println+return nil) is intentional here —
		// a missing/unconfigured profile is an error condition that should exit 1.
		packs, err := loader.LoadPacks(activeProfile)
		if err != nil {
			return err
		}
		resources := content.FlattenResources(packs)
		if len(resources) == 0 {
			fmt.Println("No resources found for your current profile.")
			return nil
		}
		printResourceTable(resources, false)
		return nil
	},
}

var resourcesSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across all SAP resources",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil)
		if err != nil {
			return err
		}
		resources := content.FilterResources(content.FlattenResources(packs), args[0])
		if len(resources) == 0 {
			fmt.Printf("No resources found matching %q.\n", args[0])
			return nil
		}
		printResourceTable(resources, true)
		return nil
	},
}

var resourcesOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: "Open a resource URL in the default browser",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil)
		if err != nil {
			return err
		}
		r := content.FindResource(content.FlattenResources(packs), args[0])
		if r == nil {
			return fmt.Errorf("resource %q not found — use 'sap-devs resources list' or 'sap-devs resources search' to browse", args[0])
		}
		if err := browser.OpenURL(r.URL); err != nil {
			fmt.Printf("Could not open browser: %v. URL: %s\n", err, r.URL)
			return nil
		}
		fmt.Printf("Opening: %s — %s\n", r.Title, r.URL)
		return nil
	},
}

// printResourceTable prints an aligned table of resources.
// showPack adds a PACK column between ID and TYPE (used by search).
func printResourceTable(resources []content.Resource, showPack bool) {
	if showPack {
		fmt.Printf("%-38s %-12s %-15s %s\n", "ID", "PACK", "TYPE", "TITLE")
		fmt.Println(strings.Repeat("-", 90))
		for _, r := range resources {
			fmt.Printf("%-38s %-12s %-15s %s\n", r.ID, r.PackID, r.Type, r.Title)
		}
	} else {
		fmt.Printf("%-38s %-15s %s\n", "ID", "TYPE", "TITLE")
		fmt.Println(strings.Repeat("-", 75))
		for _, r := range resources {
			fmt.Printf("%-38s %-15s %s\n", r.ID, r.Type, r.Title)
		}
	}
}

func init() {
	resourcesCmd.AddCommand(resourcesListCmd, resourcesSearchCmd, resourcesOpenCmd)
	rootCmd.AddCommand(resourcesCmd)
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 3: Verify vet**

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 4: Commit**

```bash
git add cmd/resources.go
git commit -m "feat: add resources list/search/open commands"
```

---

## Task 5: Final Verification

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 2: Full vet**

Run: `go vet ./...`
Expected: clean.

- [ ] **Step 3: Build the binary**

Run: `go build -o sap-devs.exe .`
Expected: binary produced.

- [ ] **Step 4: Smoke test — no profile**

Run: `./sap-devs.exe resources list`
Expected: `Error: no profile set — run 'sap-devs profile set <name>' first`

- [ ] **Step 5: Smoke test — search**

Run: `./sap-devs.exe resources search cap`
Expected: table with ID / PACK / TYPE / TITLE columns, listing CAP-related resources (or "No resources found matching" if content cache is empty — both are valid).

- [ ] **Step 6: Smoke test — unknown resource**

Run: `./sap-devs.exe resources open nonexistent/id`
Expected: `Error: resource "nonexistent/id" not found — use 'sap-devs resources list'...`

- [ ] **Step 7: Clean up binary and commit**

```bash
rm -f sap-devs.exe
git add -A
git commit -m "chore: plan3 resources complete"
```
