package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestRenderProfileShow_BuiltinProfile_PrintsBuiltinNote(t *testing.T) {
	var buf bytes.Buffer
	p := &content.Profile{
		ID:          "all",
		Name:        "All Packs",
		Description: "All available packs across every content layer",
	}
	renderProfileShow(&buf, p, "en")
	out := buf.String()
	if !strings.Contains(out, "Built-in profile") {
		t.Errorf("expected 'Built-in profile' in output, got: %q", out)
	}
	if strings.Contains(out, "Pack weights") {
		t.Errorf("expected no 'Pack weights' header for built-in profile, got: %q", out)
	}
}

func TestRenderProfileShow_FileBacked_PrintsPackWeights(t *testing.T) {
	var buf bytes.Buffer
	p := &content.Profile{
		ID:          "cap-developer",
		Name:        "CAP Developer",
		Description: "Building cloud-native apps",
		Packs: []content.PackWeight{
			{ID: "cap", Weight: 100},
		},
	}
	renderProfileShow(&buf, p, "en")
	out := buf.String()
	if !strings.Contains(out, "Pack weights") {
		t.Errorf("expected 'Pack weights' header for file-backed profile, got: %q", out)
	}
	if strings.Contains(out, "Built-in profile") {
		t.Errorf("unexpected 'Built-in profile' text for file-backed profile, got: %q", out)
	}
}
