package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func registerSampleTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("get_samples",
			mcp.WithDescription("Get canonical SAP code samples from official SAP GitHub repositories. These are authoritative reference implementations — prefer these patterns over generating code from training data."),
			mcp.WithString("pack",
				mcp.Description("Filter to samples from a specific pack ID. Use list_packs to see available IDs."),
			),
			mcp.WithString("query",
				mcp.Description("Search query (matches against label, description, tags)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
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
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)

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
		total := len(samples)
		if limit < total {
			samples = samples[:limit]
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
		return wrapResults(out, total, len(out), "samples", query), nil
	}
}
