package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
)

func registerLearnTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_tutorials",
			mcp.WithDescription("Search SAP tutorials by keyword. Returns matching tutorials with URLs."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query (matches against title, description, tags)"),
			),
		),
		searchTutorialsHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("search_learning_journeys",
			mcp.WithDescription("Search SAP Learning Journeys by keyword. Returns matching journeys with level and duration."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query (matches against title, description, level)"),
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
}

func searchTutorialsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}
		if len(deps.TutorialIndex) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		matches := tutorials.Search(deps.TutorialIndex, query)
		out := make([]tutorialResult, 0, len(matches))
		for _, t := range matches {
			out = append(out, tutorialResult{
				Slug:        t.Slug,
				Title:       t.Title,
				Description: t.Description,
				URL:         t.URL,
				Tags:        t.Tags,
			})
		}
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
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
		if len(deps.LearningIndex) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		matches := learning.Search(deps.LearningIndex, query)
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
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}
