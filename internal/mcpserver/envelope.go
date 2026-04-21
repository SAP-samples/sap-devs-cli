package mcpserver

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// ResultEnvelope wraps list results with count, total, and optional hint for agents.
type ResultEnvelope struct {
	Count   int         `json:"count"`
	Total   int         `json:"total"`
	Results any `json:"results"`
	Hint    string      `json:"hint,omitempty"`
}

func clampLimit(requested, defaultVal, maxVal int) int {
	if requested <= 0 {
		return defaultVal
	}
	if requested > maxVal {
		return maxVal
	}
	return requested
}

func wrapResults(results any, total, count int, entityName, query string) *mcp.CallToolResult {
	env := ResultEnvelope{
		Count:   count,
		Total:   total,
		Results: results,
	}
	if total == 0 && query != "" {
		env.Hint = fmt.Sprintf("No %s matched '%s'. Try broader terms.", entityName, query)
	} else if total == 0 {
		env.Hint = fmt.Sprintf("No %s available.", entityName)
	} else if count < total {
		env.Hint = fmt.Sprintf("Showing %d of %d %s. Refine your query for better results.", count, total, entityName)
	}
	b, err := json.Marshal(env)
	if err != nil {
		env.Results = nil
		env.Hint = "Failed to serialize results."
		b, _ = json.Marshal(env)
	}
	return mcp.NewToolResultText(string(b))
}

func wrapResultsWithHint(results any, total int, hint string) *mcp.CallToolResult {
	env := ResultEnvelope{
		Count:   0,
		Total:   total,
		Results: results,
		Hint:    hint,
	}
	b, _ := json.Marshal(env)
	return mcp.NewToolResultText(string(b))
}
