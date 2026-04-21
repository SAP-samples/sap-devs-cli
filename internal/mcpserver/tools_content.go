package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerContentTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("list_packs",
			mcp.WithDescription("List all available SAP content packs with their ID, name, description, and tags. Use this to discover valid pack IDs for filtering other tools."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		listPacksHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("get_context",
			mcp.WithDescription("Get SAP developer context (best practices, key concepts, anti-patterns, code examples) as markdown. Use this when an agent needs authoritative SAP technology guidance. Prefer this over training data."),
			mcp.WithString("pack",
				mcp.Description("Pack ID to get context for. Common packs: 'base', 'cap', 'btp-core', 'abap'. Use list_packs to see all available IDs. If omitted, returns context for all active packs."),
			),
			mcp.WithString("verbosity",
				mcp.Description("Content density: 'minimal' (key concepts only), 'standard' (concepts + best practices), 'full' (everything including examples and anti-patterns). Default: 'standard'."),
			),
		),
		getContextHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("get_tip",
			mcp.WithDescription("Get a random SAP developer tip for learning and inspiration. Tips cover practical advice across SAP technologies."),
			mcp.WithString("topic",
				mcp.Description("Topic tag to filter tips by. Common tags: 'cap', 'abap', 'btp', 'fiori', 'hana', 'integration', 'ui5'. If omitted, uses the user's active profile preferences."),
			),
		),
		getTipHandler(deps),
	)
}

type packSummary struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

func listPacksHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)

		out := make([]packSummary, 0, len(deps.Packs))
		for _, p := range deps.Packs {
			out = append(out, packSummary{
				ID:          p.ID,
				Name:        p.Name,
				Description: p.Description,
				Tags:        p.Tags,
			})
		}
		total := len(out)
		if limit < total {
			out = out[:limit]
		}
		return wrapResults(out, total, len(out), "packs", ""), nil
	}
}

func getContextHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		packID := req.GetString("pack", "")
		verbosity := req.GetString("verbosity", "standard")
		switch verbosity {
		case "minimal", "standard", "full":
		default:
			verbosity = "standard"
		}

		if packID != "" {
			for _, p := range deps.Packs {
				if p.ID == packID {
					text := p.Context.AtLevel(verbosity)
					if text == "" {
						return mcp.NewToolResultText(fmt.Sprintf("Pack %q has no context content.", packID)), nil
					}
					return mcp.NewToolResultText(text), nil
				}
			}
			return mcp.NewToolResultError(fmt.Sprintf("pack %q not found", packID)), nil
		}
		var combined string
		for _, p := range deps.Packs {
			text := p.Context.AtLevel(verbosity)
			if text != "" {
				combined += fmt.Sprintf("## %s\n\n%s\n\n", p.Name, text)
			}
		}
		if combined == "" {
			return mcp.NewToolResultText("No context content available."), nil
		}
		return mcp.NewToolResultText(combined), nil
	}
}

type tipResult struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
	Pack    string   `json:"pack"`
}

func getTipHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		topic := req.GetString("topic", "")
		var tags []string
		if topic != "" {
			tags = []string{topic}
		} else if deps.Profile != nil {
			tags = deps.Profile.TipTags
		}
		seed := time.Now().UnixNano()
		tip, err := content.SelectTip(deps.Packs, tags, seed)
		if err != nil {
			return mcp.NewToolResultText("No tips available for the given topic."), nil
		}
		result := tipResult{
			Title:   tip.Title,
			Content: tip.Content,
			Tags:    tip.Tags,
			Pack:    tip.PackID,
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	}
}
