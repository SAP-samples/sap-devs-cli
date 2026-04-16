package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestUnionStrings_DeduplicatesAndPreservesOrder(t *testing.T) {
	got := content.UnionStrings([]string{"a", "b"}, []string{"b", "c"})
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

func TestUnionStrings_BothEmpty(t *testing.T) {
	assert.Equal(t, []string{}, content.UnionStrings(nil, nil))
}

func TestUnionStrings_OnlyA(t *testing.T) {
	assert.Equal(t, []string{"a"}, content.UnionStrings([]string{"a"}, nil))
}

func TestMergeResources_ReplacesOnMatchingID(t *testing.T) {
	base := []content.Resource{
		{ID: "cap/docs", Title: "CAP Docs", URL: "https://old.example", PackID: "cap"},
		{ID: "cap/community", Title: "Community", URL: "https://community.sap.com", PackID: "cap"},
	}
	additive := []content.Resource{
		{ID: "cap/docs", Title: "CAP Docs Updated", URL: "https://new.example", PackID: "company-cap"},
	}
	got := content.MergeResources(base, additive, "cap")
	assert.Len(t, got, 2)
	// Replaced entry uses additive values
	assert.Equal(t, "CAP Docs Updated", got[0].Title)
	assert.Equal(t, "https://new.example", got[0].URL)
	// PackID re-stamped to base ID
	assert.Equal(t, "cap", got[0].PackID)
	// Unmatched base entry preserved
	assert.Equal(t, "cap/community", got[1].ID)
}

func TestMergeResources_AppendsNewIDs(t *testing.T) {
	base := []content.Resource{{ID: "cap/docs", Title: "Docs", PackID: "cap"}}
	additive := []content.Resource{{ID: "cap/new", Title: "New Resource", PackID: "company"}}
	got := content.MergeResources(base, additive, "cap")
	assert.Len(t, got, 2)
	assert.Equal(t, "cap/new", got[1].ID)
	assert.Equal(t, "cap", got[1].PackID)
}

func TestMergeResources_FreshSlice_NoAliasing(t *testing.T) {
	base := []content.Resource{{ID: "cap/docs", PackID: "cap"}}
	got := content.MergeResources(base, nil, "cap")
	got[0].Title = "mutated"
	assert.Empty(t, base[0].Title, "mutation must not affect original base slice")
}

func TestMergeTools_ReplacesOnMatchingID(t *testing.T) {
	base := []content.ToolDef{{ID: "nodejs", Name: "Node.js", Required: ">=18.0.0"}}
	additive := []content.ToolDef{{ID: "nodejs", Name: "Node.js", Required: ">=20.0.0"}}
	got := content.MergeTools(base, additive)
	assert.Len(t, got, 1)
	assert.Equal(t, ">=20.0.0", got[0].Required)
}

func TestMergeTools_AppendsNewIDs(t *testing.T) {
	base := []content.ToolDef{{ID: "nodejs"}}
	additive := []content.ToolDef{{ID: "bun"}}
	got := content.MergeTools(base, additive)
	assert.Len(t, got, 2)
}

func TestMergeMCPServers_ReplacesOnMatchingIDAndRestampsPackID(t *testing.T) {
	base := []content.MCPServer{{ID: "cap-mcp", Name: "Old", PackID: "cap"}}
	additive := []content.MCPServer{{ID: "cap-mcp", Name: "New", PackID: "company"}}
	got := content.MergeMCPServers(base, additive, "cap")
	assert.Len(t, got, 1)
	assert.Equal(t, "New", got[0].Name)
	assert.Equal(t, "cap", got[0].PackID)
}
