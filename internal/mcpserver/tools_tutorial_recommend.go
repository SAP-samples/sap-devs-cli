package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTutorialRecommendTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("recommend_tutorials",
			mcp.WithDescription("Get profile-matched tutorial recommendations plus any in-progress tutorials. Use when the user asks what to learn or starts a tutorial session without a specific query."),
			mcp.WithNumber("limit", mcp.Description("Maximum number of recommendations (default 5, max 20)")),
		),
		recommendTutorialsHandler(deps),
	)
}

type recommendResult struct {
	ActiveTutorials []activeTutorialItem      `json:"active_tutorials"`
	Recommended     []recommendedTutorialItem `json:"recommended"`
}

type activeTutorialItem struct {
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	CurrentStep  int    `json:"current_step"`
	TotalSteps   int    `json:"total_steps"`
	LastAccessed string `json:"last_accessed"`
}

type recommendedTutorialItem struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Level  string `json:"level,omitempty"`
	Time   int    `json:"time,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func recommendTutorialsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := clampLimit(req.GetInt("limit", 5), 5, 20)

		var active []activeTutorialItem
		all, err := tutorials.LoadProgress(deps.DataDir)
		if err == nil {
			for slug, p := range all {
				if p.CompletedAt != nil {
					continue
				}
				title := slug
				if m := tutorials.FindBySlug(deps.TutorialIndex, slug); m != nil {
					title = m.Title
				}
				active = append(active, activeTutorialItem{
					Slug:         slug,
					Title:        title,
					CurrentStep:  p.CurrentStep,
					TotalSteps:   p.TotalSteps,
					LastAccessed: p.LastAccessed.Format("2006-01-02T15:04:05Z"),
				})
			}
			sort.Slice(active, func(i, j int) bool {
				return active[i].LastAccessed > active[j].LastAccessed
			})
		}

		refs := content.FlattenTutorialRefs(deps.Packs)
		seen := make(map[string]bool)
		for _, a := range active {
			seen[a.Slug] = true
		}

		var recommended []recommendedTutorialItem
		for _, ref := range refs {
			if !ref.Featured || seen[ref.Slug] {
				continue
			}
			m := tutorials.FindBySlug(deps.TutorialIndex, ref.Slug)
			if m == nil {
				continue
			}
			packName := ref.PackID
			for _, p := range deps.Packs {
				if p.ID == ref.PackID {
					packName = p.Name
					break
				}
			}
			recommended = append(recommended, recommendedTutorialItem{
				Slug:   m.Slug,
				Title:  m.Title,
				Level:  m.Level,
				Time:   m.Time,
				Reason: fmt.Sprintf("Featured for %s", packName),
			})
			if len(recommended) >= limit {
				break
			}
		}

		result := recommendResult{
			ActiveTutorials: active,
			Recommended:     recommended,
		}
		if result.ActiveTutorials == nil {
			result.ActiveTutorials = []activeTutorialItem{}
		}
		if result.Recommended == nil {
			result.Recommended = []recommendedTutorialItem{}
		}

		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	}
}
