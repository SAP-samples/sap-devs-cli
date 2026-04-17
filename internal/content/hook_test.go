package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestFlattenHooks_ReturnsAllHooks(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Hooks: []content.HookDef{
			{ID: "tip-on-session-start", Event: "sessionStart", Command: "sap-devs tip --markdown", Tools: []string{"claude-code"}, PackID: "base"},
		}},
		{ID: "cap", Hooks: []content.HookDef{
			{ID: "cap-hook", Event: "sessionStart", Command: "sap-devs tip --plain", Tools: []string{"cursor"}, PackID: "cap"},
		}},
	}
	hooks := content.FlattenHooks(packs)
	require.Len(t, hooks, 2)
	assert.Equal(t, "tip-on-session-start", hooks[0].ID)
	assert.Equal(t, "cap-hook", hooks[1].ID)
}

func TestFlattenHooks_EmptyPacks(t *testing.T) {
	hooks := content.FlattenHooks(nil)
	assert.Empty(t, hooks)
}

func TestFlattenHooks_PackWithNoHooks(t *testing.T) {
	packs := []*content.Pack{{ID: "base"}}
	hooks := content.FlattenHooks(packs)
	assert.Empty(t, hooks)
}

func TestFindHookDef_Found(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Hooks: []content.HookDef{
			{ID: "tip-on-session-start", Event: "sessionStart", PackID: "base"},
		}},
	}
	h := content.FindHookDef(packs, "tip-on-session-start")
	require.NotNil(t, h)
	assert.Equal(t, "tip-on-session-start", h.ID)
}

func TestFindHookDef_NotFound(t *testing.T) {
	packs := []*content.Pack{
		{ID: "base", Hooks: []content.HookDef{
			{ID: "tip-on-session-start", Event: "sessionStart", PackID: "base"},
		}},
	}
	h := content.FindHookDef(packs, "nonexistent")
	assert.Nil(t, h)
}

func TestFindHookDef_NilPacks(t *testing.T) {
	h := content.FindHookDef(nil, "anything")
	assert.Nil(t, h)
}
