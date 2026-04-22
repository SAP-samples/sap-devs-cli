package mcpserver

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
)

func registerLearnTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_tutorials",
			mcp.WithDescription("Search SAP tutorials from developers.sap.com by keyword. Returns matching tutorials with direct URLs. Over 1,200 tutorials available covering CAP, ABAP, Fiori, BTP, Integration, and more."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query (matches against title, description, tags). Examples: 'CAP getting started', 'Fiori elements', 'ABAP environment'."),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 10, max 50)"),
			),
		),
		searchTutorialsHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("search_learning_journeys",
			mcp.WithDescription("Search SAP Learning Journeys from learning.sap.com. Returns structured learning paths with difficulty level and estimated duration. Use when recommending learning resources."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query (matches against title, description, level). Examples: 'BTP architect', 'ABAP skills', 'integration'."),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 10, max 50)"),
			),
		),
		searchLearningJourneysHandler(deps),
	)
}

type tutorialResult struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Tags        []string `json:"tags"`
	Level       string   `json:"level,omitempty"`
	Time        int      `json:"time,omitempty"`
}

func searchTutorialsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		if len(deps.TutorialIndex) == 0 {
			return wrapResultsWithHint([]tutorialResult{}, 0, "No tutorials loaded. Run `sap-devs sync` to fetch the tutorial index (~1,200 tutorials)."), nil
		}
		matches := tutorials.Search(deps.TutorialIndex, query)
		total := len(matches)
		if limit < total {
			matches = matches[:limit]
		}
		out := make([]tutorialResult, 0, len(matches))
		for _, t := range matches {
			out = append(out, tutorialResult{
				Slug:        t.Slug,
				Title:       t.Title,
				Description: t.Description,
				URL:         t.URL,
				Tags:        t.Tags,
				Level:       t.Level,
				Time:        t.Time,
			})
		}
		return wrapResults(out, total, len(out), "tutorials", query), nil
	}
}

type learningResult struct {
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Level    string `json:"level"`
	Duration string `json:"duration"`
	URL      string `json:"url"`
}

func searchLearningJourneysHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		if len(deps.LearningIndex) == 0 {
			return wrapResultsWithHint([]learningResult{}, 0, "No learning journeys loaded. Run `sap-devs sync` to fetch the catalog (~350 learning journeys)."), nil
		}
		matches := learning.Search(deps.LearningIndex, query)
		total := len(matches)
		if limit < total {
			matches = matches[:limit]
		}
		out := make([]learningResult, 0, len(matches))
		for _, j := range matches {
			out = append(out, learningResult{
				Slug:     j.Slug,
				Title:    j.Title,
				Level:    j.Level,
				Duration: j.DurationHours,
				URL:      j.URL,
			})
		}
		return wrapResults(out, total, len(out), "learning journeys", query), nil
	}
}
