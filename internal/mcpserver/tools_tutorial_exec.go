package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SAP-samples/sap-devs-cli/internal/credentials"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTutorialExecTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("get_tutorial_step",
			mcp.WithDescription("Get a single step from an SAP tutorial with content, annotations (executable commands, file creates, verifications), and progress. Use to guide users through tutorials step-by-step. First call for an uncached tutorial triggers a GitHub fetch."),
			mcp.WithString("slug", mcp.Required(), mcp.Description("Tutorial slug (e.g., 'cap-getting-started')")),
			mcp.WithNumber("step", mcp.Description("Step number, 1-indexed (default 1)")),
			mcp.WithBoolean("track", mcp.Description("If true (default), creates/updates progress. Set false to preview without starting.")),
		),
		getTutorialStepHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("update_tutorial_progress",
			mcp.WithDescription("Record step completion for a tutorial. Called after guiding a user through a step."),
			mcp.WithString("slug", mcp.Required(), mcp.Description("Tutorial slug")),
			mcp.WithArray("completed_steps", mcp.Required(), mcp.Description("Step numbers to mark as completed (1-indexed)"), mcp.WithNumberItems()),
			mcp.WithNumber("current_step", mcp.Description("Where the user is now. If omitted, set to max(completed) + 1.")),
		),
		updateTutorialProgressHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("get_tutorial_progress",
			mcp.WithDescription("Check progress on a specific tutorial or all tutorials with saved progress (including completed). For only incomplete tutorials, use list_active_tutorials."),
			mcp.WithString("slug", mcp.Description("Tutorial slug. If omitted, returns all tutorials with progress.")),
		),
		getTutorialProgressHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("list_active_tutorials",
			mcp.WithDescription("List tutorials with in-progress state (not yet completed). Enables 'resume where you left off' flows."),
			mcp.WithNumber("limit", mcp.Description("Maximum number of results (default 10, max 50)")),
		),
		listActiveTutorialsHandler(deps),
	)
}

type stepResult struct {
	Slug         string            `json:"slug"`
	Title        string            `json:"title"`
	Step         stepContent       `json:"step"`
	TotalSteps   int               `json:"total_steps"`
	YouWillLearn []string          `json:"you_will_learn,omitempty"`
	Progress     *progressSnapshot `json:"progress,omitempty"`
}

type stepContent struct {
	Number      int                       `json:"number"`
	Title       string                    `json:"title"`
	Content     string                    `json:"content"`
	Annotations tutorials.StepAnnotations `json:"annotations"`
}

type progressSnapshot struct {
	CompletedSteps []int  `json:"completed_steps"`
	CurrentStep    int    `json:"current_step"`
	TotalSteps     int    `json:"total_steps"`
	StartedAt      string `json:"started_at"`
	LastAccessed   string `json:"last_accessed"`
}

func getTutorialStepHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug, err := req.RequireString("slug")
		if err != nil {
			return mcp.NewToolResultError("slug parameter is required"), nil
		}
		stepNum := req.GetInt("step", 1)
		track := req.GetBool("track", true)

		meta := tutorials.FindBySlug(deps.TutorialIndex, slug)
		if meta == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Tutorial %q not found. Use search_tutorials to find valid slugs.", slug)), nil
		}

		tut, err := loadOrFetchTutorial(deps, meta)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to load tutorial %q: %v", slug, err)), nil
		}

		if stepNum < 1 || stepNum > len(tut.Steps) {
			return mcp.NewToolResultError(fmt.Sprintf("Step %d out of range. Valid range: 1..%d", stepNum, len(tut.Steps))), nil
		}

		step := tut.Steps[stepNum-1]
		annotations := tutorials.AnnotateStep(step.Content)

		var ps *progressSnapshot
		if track {
			if err := tutorials.UpdateProgress(deps.DataDir, slug, stepNum, len(tut.Steps), false); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to update progress: %v", err)), nil
			}
		}
		if p, _ := tutorials.GetProgress(deps.DataDir, slug); p != nil {
			ps = &progressSnapshot{
				CompletedSteps: p.CompletedSteps,
				CurrentStep:    p.CurrentStep,
				TotalSteps:     len(tut.Steps),
				StartedAt:      p.StartedAt.Format("2006-01-02T15:04:05Z"),
				LastAccessed:   p.LastAccessed.Format("2006-01-02T15:04:05Z"),
			}
		}

		result := stepResult{
			Slug:         slug,
			Title:        tut.Title,
			Step:         stepContent{Number: step.Number, Title: step.Title, Content: step.Content, Annotations: annotations},
			TotalSteps:   len(tut.Steps),
			YouWillLearn: tut.YouWillLearn,
			Progress:     ps,
		}

		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	}
}

func loadOrFetchTutorial(deps Deps, meta *tutorials.TutorialMeta) (*tutorials.Tutorial, error) {
	tut, err := tutorials.LoadContent(deps.CacheDir, meta.Slug)
	if err == nil {
		return tut, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	branch := "main"
	repos, _ := tutorials.LoadRepoInfo(deps.CacheDir)
	for _, r := range repos {
		if r.Name == meta.Repo {
			branch = r.DefaultBranch
			break
		}
	}

	token := credentials.Resolve(deps.ConfigDir)
	client := tutorials.NewClient(tutorials.ClientConfig{Token: token})
	raw, err := client.FetchRawMarkdown(meta.Repo, branch, meta.Slug)
	if err != nil {
		return nil, fmt.Errorf("fetch tutorial: %w", err)
	}

	tut, err = tutorials.Parse(raw, meta.Slug, meta.Repo)
	if err != nil {
		return nil, fmt.Errorf("parse tutorial: %w", err)
	}

	_ = tutorials.SaveContent(deps.CacheDir, tut)
	return tut, nil
}

// Stub handlers — implemented in Tasks 6-7

func updateTutorialProgressHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError("not implemented yet"), nil
	}
}

func getTutorialProgressHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError("not implemented yet"), nil
	}
}

func listActiveTutorialsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError("not implemented yet"), nil
	}
}
