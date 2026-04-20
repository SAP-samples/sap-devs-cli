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

func TestSearchResources(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{ID: "cap", Resources: []content.Resource{
				{ID: "cap/help", Title: "CAP Help Portal", URL: "https://help.sap.com/cap", Type: "docs", Tags: []string{"cap"}},
				{ID: "cap/samples", Title: "CAP Samples", URL: "https://github.com/sap-samples/cap", Type: "samples", Tags: []string{"cap"}},
			}},
			{ID: "abap", Resources: []content.Resource{
				{ID: "abap/rap", Title: "RAP Guide", URL: "https://help.sap.com/rap", Type: "docs", Tags: []string{"abap"}},
			}},
		},
	}
	handler := searchResourcesHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "CAP"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var resources []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resources)
	require.NoError(t, err)
	assert.Len(t, resources, 2)
}

func TestSearchResources_RequiresQuery(t *testing.T) {
	deps := Deps{Packs: []*content.Pack{}}
	handler := searchResourcesHandler(deps)
	result, err := handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
