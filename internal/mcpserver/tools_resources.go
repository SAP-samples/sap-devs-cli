package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerResourceTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_resources",
			mcp.WithDescription("Search curated SAP resources (documentation, guides, blog posts, tools) by keyword. Returns matching resources with direct URLs. Use this to find official SAP documentation links."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query — matches against title, type, and tags. Examples: 'REST API', 'authentication', 'HANA migration', 'Fiori elements'."),
			),
			mcp.WithString("pack",
				mcp.Description("Filter to resources from a specific pack ID. Use list_packs to see available IDs."),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 10, max 50)"),
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
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

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
		total := len(resources)
		if limit < total {
			resources = resources[:limit]
		}

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
		return wrapResults(out, total, len(out), "resources", query), nil
	}
}
