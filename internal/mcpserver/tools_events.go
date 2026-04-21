package mcpserver

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerEventTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_events",
			mcp.WithDescription("Search upcoming SAP community events (CodeJams, Devtoberfest, TechEd, user groups). Returns event details with dates, locations, and registration URLs. Use when users ask about SAP events or learning opportunities near them."),
			mcp.WithString("query",
				mcp.Description("Search query — matches against title, location, and tags. Examples: 'CodeJam', 'ABAP', 'virtual'."),
			),
			mcp.WithString("type",
				mcp.Description("Event type ID to filter by (e.g. 'codejam', 'devtoberfest', 'teched')"),
			),
			mcp.WithString("scope",
				mcp.Description("Filter by scope: 'local', 'regional', 'virtual', 'global'"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 10, max 50)"),
			),
		),
		searchEventsHandler(deps),
	)
}

type eventResult struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	Date     string   `json:"date"`
	EndDate  string   `json:"end_date,omitempty"`
	Location string   `json:"location,omitempty"`
	Scope    string   `json:"scope"`
	URL      string   `json:"url"`
	Tags     []string `json:"tags,omitempty"`
}

func searchEventsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		typeID := req.GetString("type", "")
		scope := req.GetString("scope", "")
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		events := content.FlattenEventInstances(deps.Packs)

		if typeID != "" {
			events = content.FilterEventsByType(events, typeID)
		}
		if query != "" {
			events = content.FilterEventsByQuery(events, query)
		}
		if scope != "" {
			var filtered []content.EventInstance
			for _, e := range events {
				if e.Scope == scope {
					filtered = append(filtered, e)
				}
			}
			events = filtered
		}

		total := len(events)
		if limit < total {
			events = events[:limit]
		}

		out := make([]eventResult, 0, len(events))
		for _, e := range events {
			out = append(out, eventResult{
				ID:       e.ID,
				Type:     e.Type,
				Title:    e.Title,
				Date:     e.DateStr,
				EndDate:  e.EndDateStr,
				Location: e.Location,
				Scope:    e.Scope,
				URL:      e.URL,
				Tags:     e.Tags,
			})
		}
		return wrapResults(out, total, len(out), "events", query), nil
	}
}
