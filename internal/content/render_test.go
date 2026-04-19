package content_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestRenderContext_BasicPacks(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Name: "CAP", Context: content.VerbositySections{Core: "## CAP\n\nUse @sap/cds."}},
		{ID: "btp-core", Name: "BTP Core", Context: content.VerbositySections{Core: "## BTP Core\n\nDeploy to Cloud Foundry."}},
	}

	out := content.RenderContext(packs, nil, nil, "full")

	assert.Contains(t, out, "Use @sap/cds.")
	assert.Contains(t, out, "Deploy to Cloud Foundry.")
	// CAP should appear before BTP Core (order preserved)
	assert.Less(t, strings.Index(out, "Use @sap/cds."), strings.Index(out, "Deploy to Cloud Foundry."))
}

func TestRenderContext_WithProfile(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Name: "CAP", Context: content.VerbositySections{Core: "CAP context."}},
	}
	profile := &content.Profile{
		ID:          "cap-developer",
		Name:        "CAP Developer",
		Description: "Building cloud-native apps with SAP CAP on BTP",
	}

	out := content.RenderContext(packs, profile, nil, "full")

	// Exact format check
	assert.Contains(t, out, "**Developer Profile:** CAP Developer — Building cloud-native apps with SAP CAP on BTP")
	assert.Contains(t, out, "CAP context.")
	// Profile line appears before pack content
	profileIdx := strings.Index(out, "**Developer Profile:**")
	packIdx := strings.Index(out, "CAP context.")
	assert.Less(t, profileIdx, packIdx, "profile line should appear before pack content")
}

func TestRenderContext_EmptyPacks(t *testing.T) {
	out := content.RenderContext(nil, nil, nil, "full")
	assert.True(t, strings.HasPrefix(out, "# SAP Developer Context\n"))
	assert.True(t, strings.HasSuffix(out, "\n") && !strings.HasSuffix(out, "\n\n"),
		"output should end with exactly one newline")
	assert.NotContains(t, out, "\n\n\n")
}

func TestRenderContext_SkipsEmptyContext(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Name: "CAP", Context: content.VerbositySections{Core: ""}},
		{ID: "btp", Name: "BTP", Context: content.VerbositySections{Core: "BTP content."}},
	}

	out := content.RenderContext(packs, nil, nil, "full")
	assert.Contains(t, out, "BTP content.")
	// The empty pack should not add extra blank lines
	assert.NotContains(t, out, "\n\n\n")
}

func TestRenderContext_SingleTrailingNewline(t *testing.T) {
	packs := []*content.Pack{{ID: "cap", Context: content.VerbositySections{Core: "## CAP\n\nContent.\n\n\n"}}}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.True(t, strings.HasSuffix(out, "\n"), "output should end with a newline")
	assert.False(t, strings.HasSuffix(out, "\n\n"), "output should not end with double newline")
}

func TestTrimPacks_Unconstrained(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP content"}},
		{ID: "btp-core", Context: content.VerbositySections{Core: "BTP content"}},
	}
	result := content.TrimPacks(packs, 0, "full")
	require.Len(t, result, 2)
	assert.Equal(t, "cap", result[0].ID)
	assert.Equal(t, "btp-core", result[1].ID)
}

func TestTrimPacks_EmptyInput(t *testing.T) {
	result := content.TrimPacks(nil, 0, "full")
	assert.Empty(t, result)
}

func TestTrimPacks_DeduplicatesOverlappingPack(t *testing.T) {
	// cap (high weight) is already included; btp-core declares it overlaps cap → dropped
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP content"}},
		{ID: "btp-core", Context: content.VerbositySections{Core: "BTP content"}, Overlaps: []string{"cap"}},
	}
	result := content.TrimPacks(packs, 0, "full")
	assert.Len(t, result, 1)
	assert.Equal(t, "cap", result[0].ID)
}

func TestTrimPacks_DeduplicatesOnlyWhenHigherWeightPresent(t *testing.T) {
	// btp-core declares overlaps: [cap], but cap is not loaded — btp-core is kept
	packs := []*content.Pack{
		{ID: "btp-core", Context: content.VerbositySections{Core: "BTP content"}, Overlaps: []string{"cap"}},
	}
	result := content.TrimPacks(packs, 0, "full")
	assert.Len(t, result, 1)
	assert.Equal(t, "btp-core", result[0].ID)
}

func TestTrimPacks_BudgetDropsPackThatDoesNotFit(t *testing.T) {
	// cap (14 bytes) exceeds 10-byte budget → break; btp-core never reached → result is empty
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "12 chars long!"}},
		{ID: "btp-core", Context: content.VerbositySections{Core: "short"}},
	}
	result := content.TrimPacks(packs, 10, "full")
	assert.Empty(t, result)
}

func TestTrimPacks_BudgetIncludesFittingPacksInOrder(t *testing.T) {
	// cap (12 bytes) fits in 20-byte budget; big (28 bytes) doesn't fit → break
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "11 chars ok!"}},   // 12 bytes
		{ID: "big", Context: content.VerbositySections{Core: "this is too large for budget"}}, // 28 bytes — doesn't fit → break
		{ID: "abap", Context: content.VerbositySections{Core: "small"}},           // never reached
	}
	result := content.TrimPacks(packs, 20, "full")
	assert.Len(t, result, 1)
	assert.Equal(t, "cap", result[0].ID)
}

func TestTrimPacks_EmptyContextAlwaysFits(t *testing.T) {
	// Pack with no context file (size 0) fits any budget
	packs := []*content.Pack{
		{ID: "meta", Context: content.VerbositySections{Core: ""}},
		{ID: "cap", Context: content.VerbositySections{Core: "some content here"}},
	}
	result := content.TrimPacks(packs, 5, "full")
	// meta fits (0 bytes), cap doesn't fit (17 bytes > 5), breaks after cap
	assert.Len(t, result, 1)
	assert.Equal(t, "meta", result[0].ID)
}

func TestTrimPacks_DeduplicateAndBudgetCombined(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "cap content here"}},             // 16 bytes, included
		{ID: "btp-core", Context: content.VerbositySections{Core: "btp"}, Overlaps: []string{"cap"}}, // deduped out
		{ID: "abap", Context: content.VerbositySections{Core: "abap content"}},                // 12 bytes, fits
	}
	result := content.TrimPacks(packs, 100, "full")
	require.Len(t, result, 2)
	assert.Equal(t, "cap", result[0].ID)
	assert.Equal(t, "abap", result[1].ID)
}

func TestRenderContext_DynamicSection_NilIsBackwardCompatible(t *testing.T) {
	packs := []*content.Pack{{ID: "cap", Context: content.VerbositySections{Core: "CAP content."}}}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.Contains(t, out, "CAP content.")
	assert.NotContains(t, out, "sap-devs Runtime Context")
}

func TestRenderContext_DynamicSection_AppearsBeforePackContent(t *testing.T) {
	packs := []*content.Pack{{ID: "cap", Context: content.VerbositySections{Core: "CAP content."}}}
	dyn := &content.DynamicContext{
		CLIVersion:    "1.2.3",
		ActiveProfile: "CAP Developer",
		LoadedPackIDs: []string{"cap"},
	}
	out := content.RenderContext(packs, nil, dyn, "full")
	dynIdx := strings.Index(out, "sap-devs Runtime Context")
	packIdx := strings.Index(out, "CAP content.")
	assert.Greater(t, packIdx, dynIdx, "runtime section must appear before pack content")
}

func TestRenderContext_DynamicSection_VersionAndProfile(t *testing.T) {
	dyn := &content.DynamicContext{
		CLIVersion:    "1.2.3",
		ActiveProfile: "CAP Developer",
		LoadedPackIDs: []string{"cap", "btp"},
	}
	out := content.RenderContext(nil, nil, dyn, "full")
	assert.Contains(t, out, "sap-devs v1.2.3")
	assert.Contains(t, out, "CAP Developer")
	assert.Contains(t, out, "cap, btp")
}

func TestRenderContext_DynamicSection_LastSyncedShown(t *testing.T) {
	synced := time.Now().Add(-2 * time.Hour)
	dyn := &content.DynamicContext{LastSynced: &synced}
	out := content.RenderContext(nil, nil, dyn, "full")
	assert.Contains(t, out, "Last synced:")
	assert.NotContains(t, out, "never")
}

func TestRenderContext_DynamicSection_NeverSyncedWhenNil(t *testing.T) {
	dyn := &content.DynamicContext{}
	out := content.RenderContext(nil, nil, dyn, "full")
	assert.Contains(t, out, "never")
}

func TestRenderContext_DynamicSection_ProjectTypeShownWhenSet(t *testing.T) {
	dyn := &content.DynamicContext{
		Project: &content.ProjectInfo{
			Type:  "CAP (Node.js)",
			Facts: []content.ProjectFact{{Key: "Project type", Value: "CAP (Node.js)"}},
		},
	}
	out := content.RenderContext(nil, nil, dyn, "full")
	assert.Contains(t, out, "**Project Context (detected):**")
	assert.Contains(t, out, "CAP (Node.js)")
}

func TestRenderContext_DynamicSection_ProjectTypeOmittedWhenEmpty(t *testing.T) {
	dyn := &content.DynamicContext{Project: nil}
	out := content.RenderContext(nil, nil, dyn, "full")
	assert.NotContains(t, out, "Project Context")
}

func TestRenderContext_DynamicSection_MCPServersShown(t *testing.T) {
	dyn := &content.DynamicContext{
		WiredMCPServers: []content.WiredMCPEntry{
			{AdapterName: "Claude Code", ServerIDs: []string{"sap-cap-mcp"}},
		},
	}
	out := content.RenderContext(nil, nil, dyn, "full")
	assert.Contains(t, out, "Claude Code")
	assert.Contains(t, out, "sap-cap-mcp")
}

func TestRenderContext_DynamicSection_MCPServersOmittedWhenNone(t *testing.T) {
	dyn := &content.DynamicContext{}
	out := content.RenderContext(nil, nil, dyn, "full")
	assert.NotContains(t, out, "Wired SAP MCP servers")
}

func TestRenderContext_DynamicSection_CommandsListed(t *testing.T) {
	dyn := &content.DynamicContext{
		Commands: []content.CommandInfo{
			{Name: "inject", Short: "Push SAP context to your AI tools"},
			{Name: "sync", Short: "Pull latest SAP developer content"},
		},
	}
	out := content.RenderContext(nil, nil, dyn, "full")
	assert.Contains(t, out, "`inject`")
	assert.Contains(t, out, "Push SAP context to your AI tools")
	assert.Contains(t, out, "`sync`")
}

func TestFormatOutput_Markdown_NoOp(t *testing.T) {
	input := "## Section\n\n**bold** and *italic*\n"
	assert.Equal(t, input, content.FormatOutput(input, "markdown"))
	assert.Equal(t, input, content.FormatOutput(input, ""))
}

func TestFormatOutput_PlainProse_Headers(t *testing.T) {
	assert.Equal(t, "Title\n", content.FormatOutput("# Title\n", "plain-prose"))
	assert.Equal(t, "Section\n", content.FormatOutput("## Section\n", "plain-prose"))
	assert.Equal(t, "Deep\n", content.FormatOutput("### Deep\n", "plain-prose"))
}

func TestFormatOutput_PlainProse_Bold(t *testing.T) {
	assert.Equal(t, "bold text here\n", content.FormatOutput("**bold text** here\n", "plain-prose"))
}

func TestFormatOutput_PlainProse_Italic(t *testing.T) {
	assert.Equal(t, "italic text here\n", content.FormatOutput("*italic text* here\n", "plain-prose"))
}

func TestFormatOutput_PlainProse_InlineCode(t *testing.T) {
	assert.Equal(t, "run cds watch now\n", content.FormatOutput("run `cds watch` now\n", "plain-prose"))
}

func TestFormatOutput_PlainProse_CodeBlock(t *testing.T) {
	input := "```bash\ncds watch\n```\n"
	out := content.FormatOutput(input, "plain-prose")
	assert.NotContains(t, out, "```")
	assert.Contains(t, out, "cds watch")
}

func TestFormatOutput_PlainProse_MultipleCodeBlocks(t *testing.T) {
	input := "```\nblock one\n```\n\n```\nblock two\n```\n"
	out := content.FormatOutput(input, "plain-prose")
	assert.NotContains(t, out, "```")
	assert.Contains(t, out, "block one")
	assert.Contains(t, out, "block two")
}

func TestFormatOutput_PlainProse_HTMLComments(t *testing.T) {
	input := "<!-- sap-devs:start:X -->\ncontent\n<!-- sap-devs:end:X -->\n"
	out := content.FormatOutput(input, "plain-prose")
	assert.NotContains(t, out, "<!--")
	assert.Contains(t, out, "content")
}

func TestFormatOutput_PlainProse_NormalizesBlankLines(t *testing.T) {
	input := "a\n\n\n\nb\n"
	out := content.FormatOutput(input, "plain-prose")
	assert.NotContains(t, out, "\n\n\n")
	assert.Contains(t, out, "a")
	assert.Contains(t, out, "b")
}

func TestTrimToBytes_UnderLimit(t *testing.T) {
	s := "hello"
	assert.Equal(t, s, content.TrimToBytes(s, 100))
}

func TestTrimToBytes_ExactLimit(t *testing.T) {
	s := "hello"
	assert.Equal(t, s, content.TrimToBytes(s, 5))
}

func TestTrimToBytes_OverLimit(t *testing.T) {
	s := "hello world"
	out := content.TrimToBytes(s, 5)
	assert.Equal(t, "hello", out)
	assert.LessOrEqual(t, len(out), 5)
}

func TestTrimToBytes_Zero(t *testing.T) {
	// maxBytes <= 0 returns unchanged
	assert.Equal(t, "hello", content.TrimToBytes("hello", 0))
	assert.Equal(t, "hello", content.TrimToBytes("hello", -1))
}

func TestTrimToBytes_UTF8Boundary(t *testing.T) {
	// "café!" = c(1) a(1) f(1) é(2) !(1) = 6 bytes total
	// Cutting at maxBytes=4 falls in the middle of the 2-byte é rune (bytes 3–4)
	// Must return "caf" (3 bytes) — not include the orphaned leading byte of é
	out := content.TrimToBytes("café!", 4)
	assert.Equal(t, "caf", out, "must cut before the straddled rune, not after its leading byte")
	assert.LessOrEqual(t, len(out), 4, "must not exceed maxBytes")
}

func TestTrimPacks_BasePackSurvivesBudget(t *testing.T) {
	// Base pack content is 20 bytes; budget is 5 — base pack must survive anyway
	packs := []*content.Pack{
		{ID: "base", Base: true, Context: content.VerbositySections{Core: "12345678901234567890"}},
		{ID: "cap", Context: content.VerbositySections{Core: "CAP content"}},
	}
	result := content.TrimPacks(packs, 5, "full")
	require.Len(t, result, 1)
	assert.Equal(t, "base", result[0].ID, "base pack must survive even when its content exceeds the budget")
}

func TestTrimPacks_BasePackSurvivesDeduplication(t *testing.T) {
	// Non-base pack declares overlaps: [base] — base pack must NOT be dropped
	packs := []*content.Pack{
		{ID: "base", Base: true, Context: content.VerbositySections{Core: "base content"}},
		{ID: "cap", Context: content.VerbositySections{Core: "CAP content"}, Overlaps: []string{"base"}},
	}
	result := content.TrimPacks(packs, 0, "full")
	// base pack survives; cap is not dropped either (its overlap target was separated out)
	require.Len(t, result, 2)
	assert.Equal(t, "base", result[0].ID)
	assert.Equal(t, "cap", result[1].ID)
}

func TestTrimPacks_BasePackFirst_NonBasePacksAfter(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP content"}},
		{ID: "base", Base: true, Context: content.VerbositySections{Core: "base content"}},
		{ID: "abap", Context: content.VerbositySections{Core: "ABAP content"}},
	}
	result := content.TrimPacks(packs, 0, "full")
	require.Len(t, result, 3)
	assert.Equal(t, "base", result[0].ID, "base pack must be first in output")
}

func TestTrimPacks_BreakOnOversizePreservedForNonBase(t *testing.T) {
	// base pack always included (even though its 17 bytes exceeds the 10-byte budget);
	// first non-base pack is too large → break; second non-base pack (small) is never reached.
	// This verifies: (a) base pack is budget-exempt, (b) break-on-first-oversize preserved for non-base.
	packs := []*content.Pack{
		{ID: "base", Base: true, Context: content.VerbositySections{Core: "base content here"}}, // 17 bytes > budget
		{ID: "big", Context: content.VerbositySections{Core: "this is way too large for budget"}},
		{ID: "small", Context: content.VerbositySections{Core: "hi"}},
	}
	result := content.TrimPacks(packs, 10, "full")
	require.Len(t, result, 1, "only base pack survives; big breaks the loop; small never reached")
	assert.Equal(t, "base", result[0].ID)
}

func TestTrimPacks_AllBasePacks_AllSurvive(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base1", Base: true, Context: content.VerbositySections{Core: "base one content"}},
		{ID: "base2", Base: true, Context: content.VerbositySections{Core: "base two content"}},
	}
	result := content.TrimPacks(packs, 5, "full") // tiny budget — ignored for base packs
	require.Len(t, result, 2)
}

func TestRenderContext_Preamble_PrecedesSameBasePackContext(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Base: true, PreambleMD: "> Preamble.", Context: content.VerbositySections{Core: "## Base context."}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	preambleIdx := strings.Index(out, "> Preamble.")
	baseCtxIdx := strings.Index(out, "## Base context.")
	require.NotEqual(t, -1, preambleIdx, "preamble must be present")
	assert.Less(t, preambleIdx, baseCtxIdx, "preamble must precede same base pack Context")
}

func TestRenderContext_Preamble_AppearsBeforeContext(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Base: true, PreambleMD: "> Prefer sap-devs.", Context: content.VerbositySections{Core: "## Base context."}},
		{ID: "cap", Context: content.VerbositySections{Core: "## CAP context."}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	preambleIdx := strings.Index(out, "> Prefer sap-devs.")
	baseCtxIdx := strings.Index(out, "## Base context.")
	capCtxIdx := strings.Index(out, "## CAP context.")
	require.NotEqual(t, -1, preambleIdx, "preamble must be present")
	assert.Less(t, preambleIdx, baseCtxIdx, "preamble must appear before base Context")
	assert.Less(t, preambleIdx, capCtxIdx, "preamble must appear before non-base Context")
}

func TestRenderContext_Preamble_NonBasePackPreambleSuppressed(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Base: false, PreambleMD: "> Should not appear.", Context: content.VerbositySections{Core: "## CAP context."}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.NotContains(t, out, "> Should not appear.", "non-base pack preamble must be suppressed")
	assert.Contains(t, out, "## CAP context.")
}

func TestRenderContext_Preamble_TwoBasePacks(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base1", Base: true, PreambleMD: "> Preamble one.", Context: content.VerbositySections{Core: "## Base one context."}},
		{ID: "base2", Base: true, PreambleMD: "> Preamble two.", Context: content.VerbositySections{Core: "## Base two context."}},
		{ID: "cap", Context: content.VerbositySections{Core: "## CAP context."}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	p1Idx := strings.Index(out, "> Preamble one.")
	p2Idx := strings.Index(out, "> Preamble two.")
	ctx1Idx := strings.Index(out, "## Base one context.")
	ctx2Idx := strings.Index(out, "## Base two context.")
	capIdx := strings.Index(out, "## CAP context.")
	require.NotEqual(t, -1, p1Idx, "preamble 1 must be present")
	require.NotEqual(t, -1, p2Idx, "preamble 2 must be present")
	assert.Less(t, p1Idx, ctx1Idx, "preamble 1 before base1 Context")
	assert.Less(t, p1Idx, ctx2Idx, "preamble 1 before base2 Context")
	assert.Less(t, p2Idx, capIdx, "preamble 2 before CAP Context")
}

func TestRenderContext_CanonicalPatterns_AppearsWhenInjectableSamplesExist(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP context."}, Samples: []content.Sample{
			{ID: "cap/handler", Label: "CAP handler", URL: "https://github.com/SAP-samples/test/blob/main/handler.js", Description: "Handler pattern", Inject: true},
			{ID: "cap/schema", Label: "CDS schema", URL: "https://github.com/SAP-samples/test/blob/main/schema.cds", Description: "Schema example", Inject: false},
		}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.Contains(t, out, "## Canonical Patterns")
	assert.Contains(t, out, "CAP handler")
	assert.Contains(t, out, "Handler pattern")
	assert.Contains(t, out, "https://github.com/SAP-samples/test/blob/main/handler.js")
	assert.NotContains(t, out, "CDS schema", "non-injectable samples must not appear")
}

func TestRenderContext_CanonicalPatterns_OmittedWhenNoInjectableSamples(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP context."}, Samples: []content.Sample{
			{ID: "cap/schema", Label: "CDS schema", URL: "https://example.com", Description: "Schema", Inject: false},
		}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.NotContains(t, out, "Canonical Patterns")
}

func TestRenderContext_CanonicalPatterns_OmittedWhenNoSamples(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP context."}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.NotContains(t, out, "Canonical Patterns")
}

func TestRenderContext_CanonicalPatterns_AppearsAfterPackContent(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP context."}, Samples: []content.Sample{
			{ID: "cap/handler", Label: "Handler", URL: "https://example.com", Description: "Desc", Inject: true},
		}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	packIdx := strings.Index(out, "CAP context.")
	patternsIdx := strings.Index(out, "## Canonical Patterns")
	assert.Greater(t, patternsIdx, packIdx, "Canonical Patterns must appear after pack content")
}

func TestRenderContext_Constraints_AppearsWhenPresent(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP context."}, Constraints: content.VerbositySections{Core: "1. Never write raw SQL"}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.Contains(t, out, "## Constraints")
	assert.Contains(t, out, "1. Never write raw SQL")
}

func TestRenderContext_Constraints_OmittedWhenEmpty(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP context."}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.NotContains(t, out, "## Constraints")
}

func TestRenderContext_Constraints_AfterPreambleBeforeContext(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Base: true, PreambleMD: "> Preamble.", Context: content.VerbositySections{Core: "## Base context."}, Constraints: content.VerbositySections{Core: "1. Base constraint"}},
		{ID: "cap", Context: content.VerbositySections{Core: "## CAP context."}, Constraints: content.VerbositySections{Core: "2. CAP constraint"}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	preambleIdx := strings.Index(out, "> Preamble.")
	constraintsIdx := strings.Index(out, "## Constraints")
	baseCtxIdx := strings.Index(out, "## Base context.")
	capCtxIdx := strings.Index(out, "## CAP context.")
	require.NotEqual(t, -1, preambleIdx)
	require.NotEqual(t, -1, constraintsIdx)
	assert.Less(t, preambleIdx, constraintsIdx, "preamble before constraints")
	assert.Less(t, constraintsIdx, baseCtxIdx, "constraints before base context")
	assert.Less(t, constraintsIdx, capCtxIdx, "constraints before cap context")
}

func TestRenderContext_Constraints_MultiplePacksMerged(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP context."}, Constraints: content.VerbositySections{Core: "1. CAP constraint"}},
		{ID: "abap", Context: content.VerbositySections{Core: "ABAP context."}, Constraints: content.VerbositySections{Core: "1. ABAP constraint"}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.Contains(t, out, "1. CAP constraint")
	assert.Contains(t, out, "1. ABAP constraint")
	assert.Equal(t, 1, strings.Count(out, "## Constraints"))
}

func TestRenderContext_Constraints_SkipsEmptyPacks(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP context."}, Constraints: content.VerbositySections{Core: "1. CAP constraint"}},
		{ID: "btp", Context: content.VerbositySections{Core: "BTP context."}, Constraints: content.VerbositySections{Core: ""}},
		{ID: "abap", Context: content.VerbositySections{Core: "ABAP context."}, Constraints: content.VerbositySections{Core: "  \n  "}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.Contains(t, out, "## Constraints")
	assert.Contains(t, out, "1. CAP constraint")
	assert.NotContains(t, out, "\n\n\n\n")
}

func TestTrimPacks_BudgetIncludesConstraints(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "hello"}, Constraints: content.VerbositySections{Core: "constraint"}},
	}
	result := content.TrimPacks(packs, 10, "full")
	assert.Empty(t, result, "pack with Context+Constraints exceeding budget must be trimmed")
}

func TestTrimPacks_BudgetFitsWithConstraints(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "hello"}, Constraints: content.VerbositySections{Core: "world"}},
	}
	result := content.TrimPacks(packs, 10, "full")
	require.Len(t, result, 1)
	assert.Equal(t, "cap", result[0].ID)
}

func TestRenderContext_ScratchNotes_RenderedAsCurrentContext(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "## CAP context."}},
	}
	dyn := &content.DynamicContext{
		ScratchNotes: []string{"implementing draft for Books", "HANA only in dev space"},
	}
	out := content.RenderContext(packs, nil, dyn, "full")
	assert.Contains(t, out, "## Current Context")
	assert.Contains(t, out, "- implementing draft for Books")
	assert.Contains(t, out, "- HANA only in dev space")
}

func TestRenderContext_ScratchNotes_BeforeRuntimeContext(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "## CAP context."}},
	}
	now := time.Now()
	dyn := &content.DynamicContext{
		CLIVersion:   "1.0.0",
		LastSynced:   &now,
		ScratchNotes: []string{"working on auth"},
	}
	out := content.RenderContext(packs, nil, dyn, "full")
	scratchIdx := strings.Index(out, "## Current Context")
	runtimeIdx := strings.Index(out, "## sap-devs Runtime Context")
	require.NotEqual(t, -1, scratchIdx, "scratch section must be present")
	require.NotEqual(t, -1, runtimeIdx, "runtime section must be present")
	assert.Less(t, scratchIdx, runtimeIdx, "scratch notes must precede runtime context")
}

func TestRenderContext_ScratchNotes_OmittedWhenEmpty(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "## CAP context."}},
	}
	dyn := &content.DynamicContext{ScratchNotes: nil}
	out := content.RenderContext(packs, nil, dyn, "full")
	assert.NotContains(t, out, "## Current Context")
}

func TestRenderContext_ScratchNotes_SanitizesNewlines(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "## CAP context."}},
	}
	dyn := &content.DynamicContext{
		ScratchNotes: []string{"line one\nline two", "cr\ronly", "win\r\nstyle"},
	}
	out := content.RenderContext(packs, nil, dyn, "full")
	assert.Contains(t, out, "- line one line two")
	assert.Contains(t, out, "- cr only")
	assert.Contains(t, out, "- win style")
}

func TestRenderContext_ScratchNotes_TruncatesLongNotes(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "## CAP context."}},
	}
	longNote := strings.Repeat("a", 600)
	dyn := &content.DynamicContext{
		ScratchNotes: []string{longNote},
	}
	out := content.RenderContext(packs, nil, dyn, "full")
	assert.Contains(t, out, "...")
	assert.NotContains(t, out, strings.Repeat("a", 501))
}

func TestRenderContext_WhatsNew_RenderedWhenPresent(t *testing.T) {
	syncDate := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	dyn := &content.DynamicContext{
		WhatsNew: []content.WhatsNewEntry{
			{Pack: "cap", Text: "CAP 9.8: native SQLite support"},
			{Pack: "abap", Text: "New Tier-1 API"},
		},
		WhatsNewDate: &syncDate,
	}
	out := content.RenderContext(nil, nil, dyn, "full")
	assert.Contains(t, out, "## What's New (since last sync, 2026-04-17)")
	assert.Contains(t, out, "- CAP 9.8: native SQLite support")
	assert.Contains(t, out, "- New Tier-1 API")
}

func TestRenderContext_WhatsNew_OmittedWhenEmpty(t *testing.T) {
	dyn := &content.DynamicContext{WhatsNew: nil}
	out := content.RenderContext(nil, nil, dyn, "full")
	assert.NotContains(t, out, "What's New")
}

func TestRenderContext_WhatsNew_NilDate(t *testing.T) {
	dyn := &content.DynamicContext{
		WhatsNew: []content.WhatsNewEntry{{Pack: "cap", Text: "test change"}},
	}
	out := content.RenderContext(nil, nil, dyn, "full")
	assert.Contains(t, out, "## What's New\n")
	assert.NotContains(t, out, "since last sync")
}

func TestRenderContext_WhatsNew_BeforeScratchNotes(t *testing.T) {
	syncDate := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	dyn := &content.DynamicContext{
		WhatsNew:     []content.WhatsNewEntry{{Pack: "cap", Text: "test change"}},
		WhatsNewDate: &syncDate,
		ScratchNotes: []string{"working on auth"},
	}
	out := content.RenderContext(nil, nil, dyn, "full")
	whatsNewIdx := strings.Index(out, "## What's New")
	scratchIdx := strings.Index(out, "## Current Context")
	require.NotEqual(t, -1, whatsNewIdx, "What's New must be present")
	require.NotEqual(t, -1, scratchIdx, "scratch notes must be present")
	assert.Less(t, whatsNewIdx, scratchIdx, "What's New must appear before scratch notes")
}

func TestRenderContext_WhatsNew_BeforeRuntimeContext(t *testing.T) {
	syncDate := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	dyn := &content.DynamicContext{
		CLIVersion:   "1.0.0",
		WhatsNew:     []content.WhatsNewEntry{{Pack: "cap", Text: "test change"}},
		WhatsNewDate: &syncDate,
	}
	out := content.RenderContext(nil, nil, dyn, "full")
	whatsNewIdx := strings.Index(out, "## What's New")
	runtimeIdx := strings.Index(out, "## sap-devs Runtime Context")
	require.NotEqual(t, -1, whatsNewIdx, "What's New must be present")
	require.NotEqual(t, -1, runtimeIdx, "runtime section must be present")
	assert.Less(t, whatsNewIdx, runtimeIdx, "What's New must appear before runtime context")
}

func TestRenderContext_VerbosityMinimal_ExcludesDetailAndExtended(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{
			Core:     "## CAP\n\nCore stuff.",
			Detail:   "\n\n### Best Practices\n\nDetail stuff.",
			Extended: "\n\n### Release Notes\n\nExtended stuff.",
		}},
	}
	out := content.RenderContext(packs, nil, nil, "minimal")
	assert.Contains(t, out, "Core stuff.")
	assert.NotContains(t, out, "Detail stuff.")
	assert.NotContains(t, out, "Extended stuff.")
}

func TestRenderContext_VerbosityStandard_IncludesDetailExcludesExtended(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{
			Core:     "## CAP\n\nCore stuff.",
			Detail:   "\n\n### Best Practices\n\nDetail stuff.",
			Extended: "\n\n### Release Notes\n\nExtended stuff.",
		}},
	}
	out := content.RenderContext(packs, nil, nil, "standard")
	assert.Contains(t, out, "Core stuff.")
	assert.Contains(t, out, "Detail stuff.")
	assert.NotContains(t, out, "Extended stuff.")
}

func TestRenderContext_VerbosityFull_IncludesAll(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{
			Core:     "## CAP\n\nCore stuff.",
			Detail:   "\n\n### Best Practices\n\nDetail stuff.",
			Extended: "\n\n### Release Notes\n\nExtended stuff.",
		}},
	}
	out := content.RenderContext(packs, nil, nil, "full")
	assert.Contains(t, out, "Core stuff.")
	assert.Contains(t, out, "Detail stuff.")
	assert.Contains(t, out, "Extended stuff.")
}

func TestRenderContext_VerbosityMinimal_ExcludesCanonicalPatterns(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP context."}, Samples: []content.Sample{
			{ID: "cap/handler", Label: "Handler", URL: "https://example.com", Description: "Desc", Inject: true},
		}},
	}
	out := content.RenderContext(packs, nil, nil, "minimal")
	assert.NotContains(t, out, "Canonical Patterns")
}

func TestRenderContext_VerbosityMinimal_ExcludesLearningAndKnownErrors(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP context."},
			LearningForInject: []content.LearningJourneyInjection{{Title: "Learn", URL: "https://example.com", Level: "BEGINNER", Duration: "1h"}},
			KnownErrors: []content.KnownError{{ID: "e1", Pattern: "ERR", Cause: "cause", Fix: "fix"}},
		},
	}
	out := content.RenderContext(packs, nil, nil, "minimal")
	assert.NotContains(t, out, "Recommended Learning")
	assert.NotContains(t, out, "Known Errors")
}

func TestRenderContext_VerbosityStandard_IncludesCanonicalPatternsExcludesExtended(t *testing.T) {
	packs := []*content.Pack{
		{ID: "cap", Context: content.VerbositySections{Core: "CAP context."},
			Samples: []content.Sample{{ID: "s1", Label: "S", URL: "https://example.com", Description: "D", Inject: true}},
			LearningForInject: []content.LearningJourneyInjection{{Title: "Learn", URL: "https://example.com", Level: "BEGINNER", Duration: "1h"}},
			KnownErrors: []content.KnownError{{ID: "e1", Pattern: "ERR", Cause: "cause", Fix: "fix"}},
		},
	}
	out := content.RenderContext(packs, nil, nil, "standard")
	assert.Contains(t, out, "Canonical Patterns")
	assert.NotContains(t, out, "Recommended Learning")
	assert.NotContains(t, out, "Known Errors")
}
