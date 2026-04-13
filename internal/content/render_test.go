package content_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestRenderContext_BasicPacks(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Name: "CAP", ContextMD: "## CAP\n\nUse @sap/cds."},
		{ID: "btp-core", Name: "BTP Core", ContextMD: "## BTP Core\n\nDeploy to Cloud Foundry."},
	}

	out := content.RenderContext(packs, nil)

	assert.Contains(t, out, "Use @sap/cds.")
	assert.Contains(t, out, "Deploy to Cloud Foundry.")
	// CAP should appear before BTP Core (order preserved)
	assert.Less(t, strings.Index(out, "Use @sap/cds."), strings.Index(out, "Deploy to Cloud Foundry."))
}

func TestRenderContext_WithProfile(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Name: "CAP", ContextMD: "CAP context."},
	}
	profile := &content.Profile{
		ID:          "cap-developer",
		Name:        "CAP Developer",
		Description: "Building cloud-native apps with SAP CAP on BTP",
	}

	out := content.RenderContext(packs, profile)

	// Exact format check
	assert.Contains(t, out, "**Developer Profile:** CAP Developer — Building cloud-native apps with SAP CAP on BTP")
	assert.Contains(t, out, "CAP context.")
	// Profile line appears before pack content
	profileIdx := strings.Index(out, "**Developer Profile:**")
	packIdx := strings.Index(out, "CAP context.")
	assert.Less(t, profileIdx, packIdx, "profile line should appear before pack content")
}

func TestRenderContext_EmptyPacks(t *testing.T) {
	out := content.RenderContext(nil, nil)
	assert.True(t, strings.HasPrefix(out, "# SAP Developer Context\n"))
	assert.True(t, strings.HasSuffix(out, "\n") && !strings.HasSuffix(out, "\n\n"),
		"output should end with exactly one newline")
	assert.NotContains(t, out, "\n\n\n")
}

func TestRenderContext_SkipsEmptyContext(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Name: "CAP", ContextMD: ""},
		{ID: "btp", Name: "BTP", ContextMD: "BTP content."},
	}

	out := content.RenderContext(packs, nil)
	assert.Contains(t, out, "BTP content.")
	// The empty pack should not add extra blank lines
	assert.NotContains(t, out, "\n\n\n")
}

func TestRenderContext_SingleTrailingNewline(t *testing.T) {
	packs := []*content.Pack{{ID: "cap", ContextMD: "## CAP\n\nContent.\n\n\n"}}
	out := content.RenderContext(packs, nil)
	assert.True(t, strings.HasSuffix(out, "\n"), "output should end with a newline")
	assert.False(t, strings.HasSuffix(out, "\n\n"), "output should not end with double newline")
}
