package mcpserver

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

type testEnvelope struct {
	Count   int               `json:"count"`
	Total   int               `json:"total"`
	Results json.RawMessage   `json:"results"`
	Hint    string            `json:"hint,omitempty"`
}

func (e *testEnvelope) resultSlice(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	require.NoError(t, json.Unmarshal(e.Results, &out))
	return out
}

func unmarshalEnvelope(t *testing.T, result *mcp.CallToolResult) *testEnvelope {
	t.Helper()
	var env testEnvelope
	err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &env)
	require.NoError(t, err)
	return &env
}
