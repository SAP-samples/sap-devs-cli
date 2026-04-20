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

func TestGetKnownErrors(t *testing.T) {
	deps := Deps{
		Packs: []*content.Pack{
			{KnownErrors: []content.KnownError{
				{ID: "cap/cds-build", Pattern: "cds build failed", Cause: "Missing dependency", Fix: "Run npm install", Tags: []string{"cap"}},
				{ID: "abap/access", Pattern: "Access not permitted", Cause: "Non-released API", Fix: "Use released API", Tags: []string{"abap"}},
			}},
		},
	}
	handler := getKnownErrorsHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "access"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var errors []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &errors)
	require.NoError(t, err)
	assert.Len(t, errors, 1)
	assert.Equal(t, "abap/access", errors[0]["id"])
}
