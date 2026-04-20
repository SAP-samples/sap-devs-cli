package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestListPacks(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{ID: "cap", Name: "CAP", Description: "Cloud Application Programming", Tags: []string{"cap", "nodejs"}},
			{ID: "abap", Name: "ABAP", Description: "ABAP Cloud", Tags: []string{"abap"}},
		},
	}
	handler := listPacksHandler(deps)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var packs []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &packs)
	require.NoError(t, err)
	assert.Len(t, packs, 2)
	assert.Equal(t, "cap", packs[0]["id"])
	assert.Equal(t, "CAP", packs[0]["name"])
}

func TestGetContext_SpecificPack(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{ID: "cap", Name: "CAP", Context: content.VerbositySections{Core: "CAP core.", Detail: "CAP detail.", Extended: "CAP extended."}},
		},
	}
	handler := getContextHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pack": "cap"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "CAP core.")
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "CAP extended.")
}

func TestGetContext_UnknownPack(t *testing.T) {
	deps := Deps{Packs: []*content.Pack{}}
	handler := getContextHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pack": "nonexistent"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGetContext_AllPacks(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{ID: "cap", Name: "CAP", Context: content.VerbositySections{Core: "CAP stuff."}},
			{ID: "abap", Name: "ABAP", Context: content.VerbositySections{Core: "ABAP stuff."}},
		},
	}
	handler := getContextHandler(deps)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "CAP stuff.")
	assert.Contains(t, text, "ABAP stuff.")
}

func TestGetTip(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{Tips: []content.Tip{{Title: "Use cds watch", Content: "Run `cds watch` for live reload.", Tags: []string{"cap"}}}},
		},
	}
	handler := getTipHandler(deps)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "cds watch")
}
