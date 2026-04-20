package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerSampleTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("get_samples",
			mcp.WithDescription("Get canonical SAP code samples, optionally filtered by pack or keyword"),
			mcp.WithString("pack",
				mcp.Description("Filter to samples from a specific pack ID"),
			),
			mcp.WithString("query",
				mcp.Description("Search query (matches against label, description, tags)"),
			),
		),
		getSamplesHandler(deps),
	)
}

type sampleResult struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Tags        []string `json:"tags"`
}

func getSamplesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		packID := req.GetString("pack", "")
		query := req.GetString("query", "")

		var samples []content.Sample
		if packID != "" {
			found := false
			for _, p := range deps.Packs {
				if p.ID == packID {
					found = true
					break
				}
			}
			if !found {
				return mcp.NewToolResultError(fmt.Sprintf("pack %q not found", packID)), nil
			}
			samples = content.FilterSamplesByPack(deps.Packs, packID)
		} else {
			samples = content.FlattenSamples(deps.Packs)
		}
		if query != "" {
			samples = content.FilterSamples(samples, query)
		}

		out := make([]sampleResult, 0, len(samples))
		for _, s := range samples {
			out = append(out, sampleResult{
				ID:          s.ID,
				Label:       s.Label,
				Description: s.Description,
				URL:         s.URL,
				Tags:        s.Tags,
			})
		}
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}
