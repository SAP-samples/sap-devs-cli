package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
		{"Cap", false},
		{"1pack", false},
		{"-pack", false},
		{"my_pack", false},
		{"my pack", false},
		{"MY-PACK", false},
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

	// Verify pack.yaml
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

	// Verify context.md
	contextMD, err := os.ReadFile(filepath.Join(packDir, "context.md"))
	if err != nil {
		t.Fatalf("context.md not written: %v", err)
	}
	for _, section := range []string{"### Overview", "### Key Concepts", "### Best Practices"} {
		if !strings.Contains(string(contextMD), section) {
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

	// Verify tips.md
	tipsMD, err := os.ReadFile(filepath.Join(packDir, "tips.md"))
	if err != nil {
		t.Fatalf("tips.md not written: %v", err)
	}
	if !strings.Contains(string(tipsMD), "## Tip title here") {
		t.Error("tips.md missing placeholder template")
	}
}

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

func TestCheckPackConflict(t *testing.T) {
	dir := t.TempDir()

	// No conflict
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
	if !strings.Contains(summary, "my-pack") {
		t.Error("summary missing pack ID")
	}
	if !strings.Contains(summary, "user") {
		t.Error("summary missing layer name")
	}
	if !strings.Contains(summary, "pack.yaml") {
		t.Error("summary missing pack.yaml")
	}
	if !strings.Contains(summary, "context.md") {
		t.Error("summary missing context.md")
	}
	if !strings.Contains(summary, "resources.yaml (1 entry)") {
		t.Error("summary missing resources.yaml entry count")
	}
	if !strings.Contains(summary, "tips.md") {
		t.Error("summary missing tips.md")
	}
}

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
			t.Errorf("weight = %v, want 50", m["weight"])
		}
	})

	t.Run("custom weight", func(t *testing.T) {
		m := buildMetadataMap("my-pack", "My Pack", "Desc", "tag", "100", false, "")
		if m["weight"] != 100 {
			t.Errorf("weight = %v, want 100", m["weight"])
		}
	})
}

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
