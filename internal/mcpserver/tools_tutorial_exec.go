package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/SAP-samples/sap-devs-cli/internal/credentials"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTutorialExecTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("get_tutorial_step",
			mcp.WithDescription("Get a single step from an SAP tutorial with content, annotations (executable commands, file creates, verifications), and progress. Use to guide users through tutorials step-by-step. First call for an uncached tutorial triggers a GitHub fetch. When include_images is true (default), tutorial images are fetched and returned inline as MCP ImageContent blocks that you can see and describe to the user."),
			mcp.WithString("slug", mcp.Required(), mcp.Description("Tutorial slug (e.g., 'cap-getting-started')")),
			mcp.WithNumber("step", mcp.Description("Step number, 1-indexed (default 1)")),
			mcp.WithBoolean("track", mcp.Description("If true (default), creates/updates progress. Set false to preview without starting.")),
			mcp.WithBoolean("include_images", mcp.Description("If true (default), fetch tutorial images and return them inline as ImageContent blocks. Set false for text-only mode with resolved image URLs.")),
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
	Slug          string            `json:"slug"`
	Title         string            `json:"title"`
	Step          stepContent       `json:"step"`
	TotalSteps    int               `json:"total_steps"`
	YouWillLearn  []string          `json:"you_will_learn,omitempty"`
	Progress      *progressSnapshot `json:"progress,omitempty"`
	PrevStepTitle *string           `json:"prev_step_title"`
	NextStepTitle *string           `json:"next_step_title"`
	Level         string            `json:"level,omitempty"`
	Time          int               `json:"time,omitempty"`
	Images        []imageRef        `json:"images,omitempty"`
}

type imageRef struct {
	Alt string `json:"alt"`
	URL string `json:"url"`
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
		includeImages := req.GetBool("include_images", true)

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

		// Resolve branch for image URL construction
		branch := resolveBranch(deps, meta.Repo)

		// Always resolve image URLs in content (keep markdown image syntax)
		resolvedContent := tutorials.ResolveImageURLsKeepMarkdown(step.Content, meta.Repo, branch, slug)
		annotations := tutorials.AnnotateStep(step.Content)

		// Extract image refs for the images field
		imgRefs := tutorials.ExtractImageRefs(step.Content, meta.Repo, branch, slug)
		var imgRefList []imageRef
		for _, ref := range imgRefs {
			imgRefList = append(imgRefList, imageRef{Alt: ref.Alt, URL: ref.URL})
		}

		var prevTitle, nextTitle *string
		if stepNum > 1 {
			t := tut.Steps[stepNum-2].Title
			prevTitle = &t
		}
		if stepNum < len(tut.Steps) {
			t := tut.Steps[stepNum].Title
			nextTitle = &t
		}

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
			Slug:          slug,
			Title:         tut.Title,
			Step:          stepContent{Number: step.Number, Title: step.Title, Content: resolvedContent, Annotations: annotations},
			TotalSteps:    len(tut.Steps),
			YouWillLearn:  tut.YouWillLearn,
			Progress:      ps,
			PrevStepTitle: prevTitle,
			NextStepTitle: nextTitle,
			Level:         meta.Level,
			Time:          meta.Time,
			Images:        imgRefList,
		}

		b, _ := json.Marshal(result)
		textContent := mcp.TextContent{Type: "text", Text: string(b)}

		if !includeImages || len(imgRefs) == 0 {
			return &mcp.CallToolResult{Content: []mcp.Content{textContent}}, nil
		}

		// Fetch images and build mixed content response
		fetched := tutorials.FetchStepImages(imgRefs, deps.CacheDir, slug)
		content := []mcp.Content{textContent}
		for _, img := range fetched {
			content = append(content, mcp.ImageContent{
				Type:     "image",
				Data:     img.Data,
				MIMEType: img.MIMEType,
			})
		}
		return &mcp.CallToolResult{Content: content}, nil
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

	branch := resolveBranch(deps, meta.Repo)

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

func resolveBranch(deps Deps, repo string) string {
	repos, _ := tutorials.LoadRepoInfo(deps.CacheDir)
	for _, r := range repos {
		if r.Name == repo {
			return r.DefaultBranch
		}
	}
	return "main"
}

type progressResult struct {
	Slug     string           `json:"slug"`
	Progress progressSnapshot `json:"progress"`
}

func updateTutorialProgressHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug, err := req.RequireString("slug")
		if err != nil {
			return mcp.NewToolResultError("slug parameter is required"), nil
		}

		meta := tutorials.FindBySlug(deps.TutorialIndex, slug)
		if meta == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Tutorial %q not found.", slug)), nil
		}

		args := req.GetArguments()
		stepsRaw, ok := args["completed_steps"]
		if !ok {
			return mcp.NewToolResultError("completed_steps parameter is required"), nil
		}
		stepsArr, ok := stepsRaw.([]any)
		if !ok {
			return mcp.NewToolResultError("completed_steps must be an array of integers"), nil
		}
		var completedSteps []int
		for _, v := range stepsArr {
			n, ok := v.(float64)
			if !ok {
				return mcp.NewToolResultError("completed_steps must be an array of integers"), nil
			}
			completedSteps = append(completedSteps, int(n))
		}

		currentStep := req.GetInt("current_step", 0)

		tut, err := loadOrFetchTutorial(deps, meta)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to load tutorial: %v", err)), nil
		}

		p, err := tutorials.MergeCompletedSteps(deps.DataDir, slug, completedSteps, currentStep, len(tut.Steps))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update progress: %v", err)), nil
		}

		result := progressResult{
			Slug: slug,
			Progress: progressSnapshot{
				CompletedSteps: p.CompletedSteps,
				CurrentStep:    p.CurrentStep,
				TotalSteps:     len(tut.Steps),
				StartedAt:      p.StartedAt.Format("2006-01-02T15:04:05Z"),
				LastAccessed:   p.LastAccessed.Format("2006-01-02T15:04:05Z"),
			},
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	}
}

type tutorialProgressResult struct {
	Slug     string           `json:"slug"`
	Title    string           `json:"title"`
	Progress progressSnapshot `json:"progress"`
}

func getTutorialProgressHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug := req.GetString("slug", "")

		if slug != "" {
			p, err := tutorials.GetProgress(deps.DataDir, slug)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to load progress: %v", err)), nil
			}
			if p == nil {
				return mcp.NewToolResultError(fmt.Sprintf("No progress found for tutorial %q.", slug)), nil
			}
			title := slug
			if m := tutorials.FindBySlug(deps.TutorialIndex, slug); m != nil {
				title = m.Title
			}
			result := tutorialProgressResult{
				Slug:  slug,
				Title: title,
				Progress: progressSnapshot{
					CompletedSteps: p.CompletedSteps,
					CurrentStep:    p.CurrentStep,
					TotalSteps:     p.TotalSteps,
					StartedAt:      p.StartedAt.Format("2006-01-02T15:04:05Z"),
					LastAccessed:   p.LastAccessed.Format("2006-01-02T15:04:05Z"),
				},
			}
			b, _ := json.Marshal(result)
			return mcp.NewToolResultText(string(b)), nil
		}

		all, err := tutorials.LoadProgress(deps.DataDir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to load progress: %v", err)), nil
		}

		var items []tutorialProgressResult
		for slug, p := range all {
			title := slug
			if m := tutorials.FindBySlug(deps.TutorialIndex, slug); m != nil {
				title = m.Title
			}
			items = append(items, tutorialProgressResult{
				Slug:  slug,
				Title: title,
				Progress: progressSnapshot{
					CompletedSteps: p.CompletedSteps,
					CurrentStep:    p.CurrentStep,
					TotalSteps:     p.TotalSteps,
					StartedAt:      p.StartedAt.Format("2006-01-02T15:04:05Z"),
					LastAccessed:   p.LastAccessed.Format("2006-01-02T15:04:05Z"),
				},
			})
		}

		sort.Slice(items, func(i, j int) bool {
			return items[i].Progress.LastAccessed > items[j].Progress.LastAccessed
		})

		return wrapResults(items, len(items), len(items), "tutorial progress entries", ""), nil
	}
}

type activeTutorialResult struct {
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	CompletedSteps []int  `json:"completed_steps"`
	TotalSteps     int    `json:"total_steps"`
	LastAccessed   string `json:"last_accessed"`
}

func listActiveTutorialsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := clampLimit(req.GetInt("limit", 10), 10, 50)

		all, err := tutorials.LoadProgress(deps.DataDir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to load progress: %v", err)), nil
		}

		var items []activeTutorialResult
		for slug, p := range all {
			if p.CompletedAt != nil {
				continue
			}
			title := slug
			if m := tutorials.FindBySlug(deps.TutorialIndex, slug); m != nil {
				title = m.Title
			}
			items = append(items, activeTutorialResult{
				Slug:           slug,
				Title:          title,
				CompletedSteps: p.CompletedSteps,
				TotalSteps:     p.TotalSteps,
				LastAccessed:   p.LastAccessed.Format("2006-01-02T15:04:05Z"),
			})
		}

		sort.Slice(items, func(i, j int) bool {
			return items[i].LastAccessed > items[j].LastAccessed
		})

		total := len(items)
		if limit < total {
			items = items[:limit]
		}
		return wrapResults(items, total, len(items), "active tutorials", ""), nil
	}
}
