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
			mcp.WithDescription("List all available SAP content packs with their ID, name, description, and tags"),
		),
		listPacksHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("get_context",
			mcp.WithDescription("Get the full SAP developer context markdown for all packs or a specific pack"),
			mcp.WithString("pack",
				mcp.Description("Pack ID to get context for. If omitted, returns context for all packs."),
			),
		),
		getContextHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("get_tip",
			mcp.WithDescription("Get a random SAP developer tip, optionally filtered by topic tag"),
			mcp.WithString("topic",
				mcp.Description("Topic tag to filter tips by (e.g. 'cap', 'abap', 'btp')"),
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
		out := make([]packSummary, 0, len(deps.Packs))
		for _, p := range deps.Packs {
			out = append(out, packSummary{
				ID:          p.ID,
				Name:        p.Name,
				Description: p.Description,
				Tags:        p.Tags,
			})
		}
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}

func getContextHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		packID := req.GetString("pack", "")
		if packID != "" {
			for _, p := range deps.Packs {
				if p.ID == packID {
					text := p.Context.AtLevel("full")
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
			text := p.Context.AtLevel("full")
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
		return mcp.NewToolResultText(fmt.Sprintf("## %s\n\n%s", tip.Title, tip.Content)), nil
	}
}
