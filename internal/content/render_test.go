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

	assert.Contains(t, out, "CAP Developer")
	assert.Contains(t, out, "CAP context.")
}

func TestRenderContext_EmptyPacks(t *testing.T) {
	out := content.RenderContext(nil, nil)
	assert.NotEmpty(t, out) // Always emits the SAP header
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
