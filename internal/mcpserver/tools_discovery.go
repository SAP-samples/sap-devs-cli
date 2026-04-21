package mcpserver

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/discovery"
)

func registerDiscoveryTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_discovery",
			mcp.WithDescription("Search SAP Discovery Center missions and BTP services. Missions are guided hands-on experiences; services are the BTP service catalog. Use when users need to explore SAP BTP capabilities or find guided learning missions."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query for missions or services"),
			),
			mcp.WithString("type",
				mcp.Description("Either 'missions' or 'services'. Default: 'missions'."),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 10, max 50)"),
			),
		),
		searchDiscoveryHandler(deps),
	)
}

type missionResult struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Effort      string `json:"effort"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type serviceResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Deprecated  bool   `json:"deprecated"`
}

func searchDiscoveryHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		searchType := req.GetString("type", "missions")
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		client := discovery.NewClient()

		if searchType == "services" {
			services, err := client.FetchServices()
			if err != nil {
				return wrapResults([]serviceResult{}, 0, 0, "services", query), nil
			}
			var filtered []serviceResult
			for _, s := range services {
				if matchesQuery(query, s.Name, s.ShortDescription, s.Category) {
					filtered = append(filtered, serviceResult{
						ID:          s.ID,
						Name:        s.Name,
						Category:    s.Category,
						Description: s.ShortDescription,
						Deprecated:  s.IsDeprecatedService,
					})
				}
			}
			total := len(filtered)
			if limit < total {
				filtered = filtered[:limit]
			}
			return wrapResults(filtered, total, len(filtered), "services", query), nil
		}

		filters := discovery.SearchFilters{Top: limit}
		missions, err := client.SearchMissions(query, filters)
		if err != nil {
			return wrapResults([]missionResult{}, 0, 0, "missions", query), nil
		}
		total := len(missions)
		if limit < total {
			missions = missions[:limit]
		}
		out := make([]missionResult, 0, len(missions))
		for _, m := range missions {
			out = append(out, missionResult{
				ID:          m.ID,
				Name:        m.Name,
				Effort:      m.Effort,
				Category:    m.Category,
				Description: m.UCLongDescription,
			})
		}
		return wrapResults(out, total, len(out), "missions", query), nil
	}
}

func matchesQuery(query string, fields ...string) bool {
	q := strings.ToLower(query)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}
