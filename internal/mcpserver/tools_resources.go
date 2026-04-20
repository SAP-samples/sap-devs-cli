package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerResourceTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_resources",
			mcp.WithDescription("Search curated SAP resources by keyword. Returns matching resources with URLs."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query (matches against title, type, tags)"),
			),
			mcp.WithString("pack",
				mcp.Description("Filter to resources from a specific pack ID"),
			),
		),
		searchResourcesHandler(deps),
	)
}

type resourceResult struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	URL   string   `json:"url"`
	Type  string   `json:"type"`
	Tags  []string `json:"tags"`
}

func searchResourcesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		packID := req.GetString("pack", "")

		var resources []content.Resource
		if packID != "" {
			found := false
			for _, p := range deps.Packs {
				if p.ID == packID {
					resources = p.Resources
					found = true
					break
				}
			}
			if !found {
				return mcp.NewToolResultError(fmt.Sprintf("pack %q not found", packID)), nil
			}
		} else {
			resources = content.FlattenResources(deps.Packs)
		}
		resources = content.FilterResources(resources, query)

		out := make([]resourceResult, 0, len(resources))
		for _, r := range resources {
			out = append(out, resourceResult{
				ID:    r.ID,
				Title: r.Title,
				URL:   r.URL,
				Type:  r.Type,
				Tags:  r.Tags,
			})
		}
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}
