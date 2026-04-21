package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tutorialExecDeps(t *testing.T) Deps {
	t.Helper()
	dir := t.TempDir()
	cacheDir := t.TempDir()

	tut := &tutorials.Tutorial{
		TutorialMeta: tutorials.TutorialMeta{
			Slug:  "cap-getting-started",
			Title: "Getting Started with CAP",
			Repo:  "Tutorials-en",
		},
		YouWillLearn: []string{"How to init a CAP project"},
		Steps: []tutorials.TutorialStep{
			{Number: 1, Title: "Set up", Content: "Install CDS:\n\n```bash\nnpm i -g @sap/cds-dk\n```\n"},
			{Number: 2, Title: "Init project", Content: "Run:\n\n```bash\ncds init bookshop\n```\n"},
		},
	}
	require.NoError(t, tutorials.SaveContent(cacheDir, tut))

	return Deps{
		TutorialIndex: []tutorials.TutorialMeta{tut.TutorialMeta},
		CacheDir:      cacheDir,
		DataDir:       dir,
	}
}

func TestGetTutorialStep_Valid(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := getTutorialStepHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"slug": "cap-getting-started", "step": float64(1)}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resp))
	assert.Equal(t, "cap-getting-started", resp["slug"])
	assert.Equal(t, float64(2), resp["total_steps"])

	step := resp["step"].(map[string]any)
	assert.Equal(t, float64(1), step["number"])
	assert.Equal(t, "Set up", step["title"])
	assert.Contains(t, step["content"], "npm i -g @sap/cds-dk")

	anns := step["annotations"].(map[string]any)
	cmds := anns["commands"].([]any)
	assert.Len(t, cmds, 1)
}

func TestGetTutorialStep_InvalidSlug(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := getTutorialStepHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"slug": "nonexistent"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGetTutorialStep_OutOfRange(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := getTutorialStepHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"slug": "cap-getting-started", "step": float64(99)}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGetTutorialStep_TracksProgress(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := getTutorialStepHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"slug": "cap-getting-started", "step": float64(1)}
	_, err := handler(context.Background(), req)
	require.NoError(t, err)

	p, err := tutorials.GetProgress(deps.DataDir, "cap-getting-started")
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, 1, p.CurrentStep)
}

func TestGetTutorialStep_NoTrack(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := getTutorialStepHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"slug": "cap-getting-started", "step": float64(1), "track": false}
	_, err := handler(context.Background(), req)
	require.NoError(t, err)

	p, err := tutorials.GetProgress(deps.DataDir, "cap-getting-started")
	require.NoError(t, err)
	assert.Nil(t, p)
}

func TestUpdateTutorialProgress_Basic(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := updateTutorialProgressHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"slug":            "cap-getting-started",
		"completed_steps": []any{float64(1)},
		"current_step":    float64(2),
	}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resp))
	prog := resp["progress"].(map[string]any)
	assert.Equal(t, float64(2), prog["current_step"])
}

func TestUpdateTutorialProgress_InvalidSlug(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := updateTutorialProgressHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"slug":            "nonexistent",
		"completed_steps": []any{float64(1)},
	}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestUpdateTutorialProgress_OutOfRange(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := updateTutorialProgressHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"slug":            "cap-getting-started",
		"completed_steps": []any{float64(99)},
	}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
