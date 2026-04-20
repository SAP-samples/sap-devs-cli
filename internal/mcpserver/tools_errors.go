package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerErrorTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("get_known_errors",
			mcp.WithDescription("Look up known SAP error patterns by keyword. Returns cause and fix for matching errors."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query (matches against error pattern, cause, fix, tags)"),
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

		all := content.FlattenKnownErrors(deps.Packs)
		matches := content.FilterKnownErrors(all, query)

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
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}
