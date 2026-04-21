package mcpserver

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
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

	env := unmarshalEnvelope(t, result)
	errors := env.resultSlice(t)
	assert.Len(t, errors, 1)
	assert.Equal(t, "abap/access", errors[0]["id"])
}
