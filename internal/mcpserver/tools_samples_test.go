package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestGetSamples(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{ID: "cap", Samples: []content.Sample{
				{ID: "cap/bookshop", Label: "CAP Bookshop", URL: "https://github.com/sap-samples/bookshop", Description: "Reference app", Tags: []string{"cap"}},
			}},
		},
	}
	handler := getSamplesHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "bookshop"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var samples []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &samples)
	require.NoError(t, err)
	assert.Len(t, samples, 1)
	assert.Equal(t, "CAP Bookshop", samples[0]["label"])
}

func TestGetSamples_FilterByPack(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{ID: "cap", Samples: []content.Sample{
				{ID: "cap/bookshop", Label: "CAP Bookshop", URL: "https://github.com/sap-samples/bookshop", Description: "Reference app", Tags: []string{"cap"}},
			}},
			{ID: "abap", Samples: []content.Sample{
				{ID: "abap/rap", Label: "RAP Sample", URL: "https://github.com/sap-samples/rap", Description: "RAP reference", Tags: []string{"abap"}},
			}},
		},
	}
	handler := getSamplesHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pack": "cap"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var samples []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &samples)
	require.NoError(t, err)
	assert.Len(t, samples, 1)
	assert.Equal(t, "cap/bookshop", samples[0]["id"])
}

func TestGetSamples_EmptyPacks(t *testing.T) {
	deps := Deps{Packs: nil}
	handler := getSamplesHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "[]", result.Content[0].(mcp.TextContent).Text)
}
