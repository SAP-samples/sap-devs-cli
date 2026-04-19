package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMergeMCPServers_AppendsNewIDs(t *testing.T) {
	base := []content.MCPServer{{ID: "cap-mcp", Name: "CAP MCP", PackID: "cap"}}
	additive := []content.MCPServer{{ID: "new-mcp", Name: "New MCP", PackID: "company"}}
	got := content.MergeMCPServers(base, additive, "cap")
	assert.Len(t, got, 2)
	assert.Equal(t, "new-mcp", got[1].ID)
	// PackID re-stamped to base pack ID on new entries too
	assert.Equal(t, "cap", got[1].PackID)
}

func makePack(id, name, context string, tips []content.Tip, resources []content.Resource) *content.Pack {
	return &content.Pack{
		ID:        id,
		Name:      name,
		Context:   content.VerbositySections{Core: context},
		Tips:      tips,
		Resources: resources,
		Tags:      []string{"base-tag"},
		Profiles:  []string{"cap-developer"},
		Overlaps:  []string{},
	}
}

func TestMergeWith_GuardReturnBaseWhenNotAdditive(t *testing.T) {
	base := makePack("cap", "CAP Official", "base context", nil, nil)
	notAdditive := &content.Pack{ID: "cap", Name: "Override", Additive: false}
	result := notAdditive.MergeWith(base)
	assert.Equal(t, base, result, "non-additive MergeWith must return base unchanged")
}

func TestMergeWith_ContextAfter(t *testing.T) {
	base := makePack("cap", "CAP", "base context", nil, nil)
	additive := &content.Pack{ID: "cap", Context: content.VerbositySections{Core: "extra context"}, Additive: true, AdditivePosition: "after"}
	result := additive.MergeWith(base)
	assert.Equal(t, "base context\n\nextra context", result.Context.Core)
}

func TestMergeWith_ContextBefore(t *testing.T) {
	base := makePack("cap", "CAP", "base context", nil, nil)
	additive := &content.Pack{ID: "cap", Context: content.VerbositySections{Core: "extra context"}, Additive: true, AdditivePosition: "before"}
	result := additive.MergeWith(base)
	assert.Equal(t, "extra context\n\nbase context", result.Context.Core)
}

func TestMergeWith_EmptyContextPreservesBase(t *testing.T) {
	base := makePack("cap", "CAP", "base context", nil, nil)
	additive := &content.Pack{ID: "cap", Context: content.VerbositySections{}, Additive: true, AdditivePosition: "after"}
	result := additive.MergeWith(base)
	assert.Equal(t, "base context", result.Context.Core)
}

func TestMergeWith_TipsAfter(t *testing.T) {
	baseTips := []content.Tip{{Title: "Base Tip"}}
	addTips := []content.Tip{{Title: "Additive Tip"}}
	base := makePack("cap", "CAP", "", baseTips, nil)
	additive := &content.Pack{ID: "cap", Tips: addTips, Additive: true, AdditivePosition: "after"}
	result := additive.MergeWith(base)
	require.Len(t, result.Tips, 2)
	assert.Equal(t, "Base Tip", result.Tips[0].Title)
	assert.Equal(t, "Additive Tip", result.Tips[1].Title)
}

func TestMergeWith_TipsBefore(t *testing.T) {
	baseTips := []content.Tip{{Title: "Base Tip"}}
	addTips := []content.Tip{{Title: "Additive Tip"}}
	base := makePack("cap", "CAP", "", baseTips, nil)
	additive := &content.Pack{ID: "cap", Tips: addTips, Additive: true, AdditivePosition: "before"}
	result := additive.MergeWith(base)
	require.Len(t, result.Tips, 2)
	assert.Equal(t, "Additive Tip", result.Tips[0].Title)
	assert.Equal(t, "Base Tip", result.Tips[1].Title)
}

func TestMergeWith_TipsNoAliasing(t *testing.T) {
	baseTips := []content.Tip{{Title: "Base Tip"}}
	base := makePack("cap", "CAP", "", baseTips, nil)
	additive := &content.Pack{ID: "cap", Additive: true, AdditivePosition: "after"}
	result := additive.MergeWith(base)
	result.Tips[0].Title = "mutated"
	assert.Equal(t, "Base Tip", base.Tips[0].Title, "mutation must not affect base tips")
}

func TestMergeWith_MetadataOverrideOnNonEmpty(t *testing.T) {
	base := makePack("cap", "CAP Official", "", nil, nil)
	base.Description = "Official description"
	base.Weight = 100
	additive := &content.Pack{
		ID: "cap", Name: "CAP Company", Description: "Company description",
		Weight: 150, Tags: []string{"extra"}, Additive: true, AdditivePosition: "after",
	}
	result := additive.MergeWith(base)
	assert.Equal(t, "CAP Company", result.Name)
	assert.Equal(t, "Company description", result.Description)
	assert.Equal(t, 150, result.Weight)
	assert.Contains(t, result.Tags, "base-tag")
	assert.Contains(t, result.Tags, "extra")
}

func TestMergeWith_MetadataEmptyFieldsPreserveBase(t *testing.T) {
	base := makePack("cap", "CAP Official", "", nil, nil)
	base.Description = "Official description"
	base.Weight = 100
	additive := &content.Pack{ID: "cap", Name: "", Description: "", Weight: 0, Additive: true, AdditivePosition: "after"}
	result := additive.MergeWith(base)
	assert.Equal(t, "CAP Official", result.Name)
	assert.Equal(t, "Official description", result.Description)
	assert.Equal(t, 100, result.Weight)
}

func TestMergeWith_ProfilesAndOverlapsTakenFromBase(t *testing.T) {
	base := makePack("cap", "CAP", "", nil, nil)
	base.Profiles = []string{"cap-developer"}
	base.Overlaps = []string{"btp-core"}
	additive := &content.Pack{
		ID: "cap", Additive: true, AdditivePosition: "after",
		Profiles: []string{"company-profile"}, Overlaps: []string{"other"},
	}
	result := additive.MergeWith(base)
	assert.Equal(t, []string{"cap-developer"}, result.Profiles)
	assert.Equal(t, []string{"btp-core"}, result.Overlaps)
}

func TestMergeWith_ProfilesNoAliasing(t *testing.T) {
	base := makePack("cap", "CAP", "", nil, nil)
	base.Profiles = []string{"cap-developer"}
	additive := &content.Pack{ID: "cap", Additive: true, AdditivePosition: "after"}
	result := additive.MergeWith(base)
	result.Profiles[0] = "mutated"
	assert.Equal(t, "cap-developer", base.Profiles[0])
}

func TestMergeWith_AdditiveIsFalseOnResult(t *testing.T) {
	base := makePack("cap", "CAP", "", nil, nil)
	additive := &content.Pack{ID: "cap", Additive: true, AdditivePosition: "after"}
	result := additive.MergeWith(base)
	assert.False(t, result.Additive)
	assert.Equal(t, "", result.AdditivePosition, "AdditivePosition must be cleared on merged result")
}

func TestMergeWith_PreambleMDPreservedFromBase(t *testing.T) {
	base := &content.Pack{
		ID:         "base",
		Base:       true,
		PreambleMD: "> Official preamble.",
		Context:    content.VerbositySections{Core: "Base context."},
	}
	additive := &content.Pack{
		ID:         "base",
		Additive:   true,
		PreambleMD: "> Additive preamble.",
	}
	merged := additive.MergeWith(base)
	assert.Equal(t, "> Official preamble.", merged.PreambleMD,
		"additive layer must not override base pack PreambleMD")
}

func TestMergeHooks_ReplacesOnMatchingIDAndRestampsPackID(t *testing.T) {
	base := []content.HookDef{{ID: "lint-hook", Event: "PreToolUse", Command: "old-cmd", PackID: "cap"}}
	additive := []content.HookDef{{ID: "lint-hook", Event: "PreToolUse", Command: "new-cmd", PackID: "company"}}
	got := content.MergeHooks(base, additive, "cap")
	assert.Len(t, got, 1)
	assert.Equal(t, "new-cmd", got[0].Command)
	assert.Equal(t, "cap", got[0].PackID)
}

func TestMergeHooks_AppendsNewIDs(t *testing.T) {
	base := []content.HookDef{{ID: "lint-hook", Event: "PreToolUse", Command: "cmd-a", PackID: "cap"}}
	additive := []content.HookDef{{ID: "format-hook", Event: "PostToolUse", Command: "cmd-b", PackID: "company"}}
	got := content.MergeHooks(base, additive, "cap")
	assert.Len(t, got, 2)
	assert.Equal(t, "format-hook", got[1].ID)
	assert.Equal(t, "cap", got[1].PackID)
}

func TestMergeWith_HooksFromAdditivePack(t *testing.T) {
	base := makePack("cap", "CAP", "", nil, nil)
	base.Hooks = []content.HookDef{{ID: "base-hook", Event: "PreToolUse", Command: "base-cmd", PackID: "cap"}}
	additive := &content.Pack{
		ID:       "cap",
		Additive: true,
		Hooks:    []content.HookDef{{ID: "extra-hook", Event: "PostToolUse", Command: "extra-cmd"}},
	}
	result := additive.MergeWith(base)
	require.Len(t, result.Hooks, 2)
	assert.Equal(t, "base-hook", result.Hooks[0].ID)
	assert.Equal(t, "extra-hook", result.Hooks[1].ID)
	// PackID re-stamped to base pack ID
	assert.Equal(t, "cap", result.Hooks[1].PackID)
}

func TestMergeSamples_ReplacesOnMatchingIDAndRestampsPackID(t *testing.T) {
	base := []content.Sample{
		{ID: "cap/handler", Label: "Old", URL: "https://old.example", PackID: "cap"},
		{ID: "cap/schema", Label: "Schema", URL: "https://schema.example", PackID: "cap"},
	}
	additive := []content.Sample{
		{ID: "cap/handler", Label: "New", URL: "https://new.example", PackID: "company"},
	}
	got := content.MergeSamples(base, additive, "cap")
	assert.Len(t, got, 2)
	assert.Equal(t, "New", got[0].Label)
	assert.Equal(t, "https://new.example", got[0].URL)
	assert.Equal(t, "cap", got[0].PackID)
	assert.Equal(t, "cap/schema", got[1].ID)
}

func TestMergeSamples_AppendsNewIDs(t *testing.T) {
	base := []content.Sample{{ID: "cap/handler", Label: "Handler", PackID: "cap"}}
	additive := []content.Sample{{ID: "cap/new", Label: "New Sample", PackID: "company"}}
	got := content.MergeSamples(base, additive, "cap")
	assert.Len(t, got, 2)
	assert.Equal(t, "cap/new", got[1].ID)
	assert.Equal(t, "cap", got[1].PackID)
}

func TestMergeWith_Constraints_After(t *testing.T) {
	base := makePack("cap", "CAP", "", nil, nil)
	base.Constraints = content.VerbositySections{Core: "1. Base constraint"}
	additive := &content.Pack{
		ID: "cap", Constraints: content.VerbositySections{Core: "2. Additive constraint"},
		Additive: true, AdditivePosition: "after",
	}
	result := additive.MergeWith(base)
	assert.Equal(t, "1. Base constraint\n\n2. Additive constraint", result.Constraints.Core)
}

func TestMergeWith_Constraints_Before(t *testing.T) {
	base := makePack("cap", "CAP", "", nil, nil)
	base.Constraints = content.VerbositySections{Core: "1. Base constraint"}
	additive := &content.Pack{
		ID: "cap", Constraints: content.VerbositySections{Core: "2. Additive constraint"},
		Additive: true, AdditivePosition: "before",
	}
	result := additive.MergeWith(base)
	assert.Equal(t, "2. Additive constraint\n\n1. Base constraint", result.Constraints.Core)
}

func TestMergeWith_Constraints_EmptyAdditivePreservesBase(t *testing.T) {
	base := makePack("cap", "CAP", "", nil, nil)
	base.Constraints = content.VerbositySections{Core: "1. Base constraint"}
	additive := &content.Pack{
		ID: "cap", Constraints: content.VerbositySections{},
		Additive: true, AdditivePosition: "after",
	}
	result := additive.MergeWith(base)
	assert.Equal(t, "1. Base constraint", result.Constraints.Core)
}
