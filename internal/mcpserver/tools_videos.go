package mcpserver

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/videos"
)

func registerVideoTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("search_videos",
			mcp.WithDescription("Search SAP developer videos from the SAP Developers YouTube channel. Covers tutorials, Tech Bytes, live streams, and conference talks. Use when users want video learning content."),
			mcp.WithString("query",
				mcp.Description("Search query — matches against title, description, and tags. Examples: 'CAP tutorial', 'Fiori elements', 'ABAP RAP'."),
			),
			mcp.WithString("source",
				mcp.Description("Source ID to filter by (e.g. 'sap-tech-bytes', 'developer-news')"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 10, max 50)"),
			),
		),
		searchVideosHandler(deps),
	)
}

type videoResult struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Published   string   `json:"published"`
	Duration    string   `json:"duration,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func searchVideosHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		source := req.GetString("source", "")
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		var allVids []content.Video
		for _, p := range deps.Packs {
			sources := p.YouTubeSources
			if source != "" {
				var filtered []content.YouTubeSource
				for _, s := range sources {
					if s.ID == source {
						filtered = append(filtered, s)
					}
				}
				sources = filtered
			}
			vids, _ := videos.ResolveAll(sources, deps.CacheDir)
			allVids = append(allVids, vids...)
		}

		if query != "" {
			allVids = videos.FilterVideos(allVids, query)
		}

		total := len(allVids)
		if limit < total {
			allVids = allVids[:limit]
		}

		out := make([]videoResult, 0, len(allVids))
		for _, v := range allVids {
			desc := v.Description
			if len(desc) > 200 {
				desc = desc[:200] + "..."
			}
			out = append(out, videoResult{
				ID:          v.ID,
				Title:       v.Title,
				URL:         v.URL,
				Published:   v.Published.Format("2006-01-02"),
				Duration:    v.Duration,
				Description: desc,
				Tags:        v.Tags,
			})
		}
		return wrapResults(out, total, len(out), "videos", query), nil
	}
}
