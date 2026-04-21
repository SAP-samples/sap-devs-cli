package mcpserver

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerErrorTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("get_known_errors",
			mcp.WithDescription("Look up known SAP error patterns by keyword. Returns root cause analysis and fix instructions. ALWAYS use this when a user encounters an SAP error message before attempting to diagnose from training data."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query — matches against error message patterns, root causes, fixes, and tags. Paste the actual error message or key phrase for best results."),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 10, max 50)"),
			),
		),
		getKnownErrorsHandler(deps),
	)
}

type knownErrorResult struct {
	ID      string   `json:"id"`
	Pattern string   `json:"pattern"`
	Cause   string   `json:"cause"`
	Fix     string   `json:"fix"`
	Docs    string   `json:"docs,omitempty"`
	Tags    []string `json:"tags"`
}

func getKnownErrorsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		all := content.FlattenKnownErrors(deps.Packs)
		matches := content.FilterKnownErrors(all, query)
		total := len(matches)
		if limit < total {
			matches = matches[:limit]
		}

		out := make([]knownErrorResult, 0, len(matches))
		for _, e := range matches {
			out = append(out, knownErrorResult{
				ID:      e.ID,
				Pattern: e.Pattern,
				Cause:   e.Cause,
				Fix:     e.Fix,
				Docs:    e.Docs,
				Tags:    e.Tags,
			})
		}
		return wrapResults(out, total, len(out), "error patterns", query), nil
	}
}
