package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestGetTutorialProgress_Single(t *testing.T) {
	deps := tutorialExecDeps(t)

	require.NoError(t, tutorials.UpdateProgress(deps.DataDir, "cap-getting-started", 2, 2, true))

	handler := getTutorialProgressHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"slug": "cap-getting-started"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resp))
	assert.Equal(t, "cap-getting-started", resp["slug"])
}

func TestGetTutorialProgress_All(t *testing.T) {
	deps := tutorialExecDeps(t)

	require.NoError(t, tutorials.UpdateProgress(deps.DataDir, "cap-getting-started", 1, 2, false))
	require.NoError(t, tutorials.UpdateProgress(deps.DataDir, "other-tut", 1, 3, false))

	handler := getTutorialProgressHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	env := unmarshalEnvelope(t, result)
	assert.Equal(t, 2, env.Count)
}

func TestGetTutorialProgress_None(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := getTutorialProgressHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"slug": "nonexistent"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestListActiveTutorials_FiltersCompleted(t *testing.T) {
	deps := tutorialExecDeps(t)

	require.NoError(t, tutorials.UpdateProgress(deps.DataDir, "cap-getting-started", 1, 2, false))
	require.NoError(t, tutorials.UpdateProgress(deps.DataDir, "done-tut", 1, 1, true))

	handler := listActiveTutorialsHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	env := unmarshalEnvelope(t, result)
	assert.Equal(t, 1, env.Count)
	items := env.resultSlice(t)
	assert.Equal(t, "cap-getting-started", items[0]["slug"])
}

func TestListActiveTutorials_Empty(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := listActiveTutorialsHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	env := unmarshalEnvelope(t, result)
	assert.Equal(t, 0, env.Count)
}

func TestGetTutorialStep_NavigationMetadata(t *testing.T) {
	deps := tutorialExecDeps(t)
	deps.TutorialIndex[0].Level = "beginner"
	deps.TutorialIndex[0].Time = 20

	handler := getTutorialStepHandler(deps)

	// Step 1: no prev, has next
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"slug": "cap-getting-started", "step": float64(1), "track": false}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var sr map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &sr))

	assert.Nil(t, sr["prev_step_title"])
	assert.Equal(t, "Init project", sr["next_step_title"])
	assert.Equal(t, "beginner", sr["level"])
	assert.Equal(t, float64(20), sr["time"])

	// Step 2: has prev, no next
	req2 := mcp.CallToolRequest{}
	req2.Params.Arguments = map[string]any{"slug": "cap-getting-started", "step": float64(2), "track": false}
	result2, _ := handler(context.Background(), req2)

	var sr2 map[string]any
	json.Unmarshal([]byte(result2.Content[0].(mcp.TextContent).Text), &sr2)

	assert.Equal(t, "Set up", sr2["prev_step_title"])
	assert.Nil(t, sr2["next_step_title"])
}

var testPNG = func() []byte {
	b, _ := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==")
	return b
}()

func TestGetTutorialStep_IncludesImages(t *testing.T) {
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG)
	}))
	defer imgSrv.Close()

	deps := tutorialExecDeps(t)
	tut, _ := tutorials.LoadContent(deps.CacheDir, "cap-getting-started")
	tut.Steps[0].Content = fmt.Sprintf("Install CDS:\n\n![cds commands](%s/cds_commands.png)\n", imgSrv.URL)
	tut.Repo = "test-repo"
	tutorials.SaveContent(deps.CacheDir, tut)

	handler := getTutorialStepHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"slug": "cap-getting-started", "step": float64(1), "track": false}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(result.Content), 2, "expected text + image content blocks")

	textBlock, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "first content block should be TextContent")

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(textBlock.Text), &resp))
	assert.Equal(t, "cap-getting-started", resp["slug"])

	images, ok := resp["images"].([]any)
	assert.True(t, ok, "expected images array in response")
	assert.Len(t, images, 1)

	imgBlock, ok := result.Content[1].(mcp.ImageContent)
	require.True(t, ok, "second content block should be ImageContent")
	assert.Equal(t, "image/png", imgBlock.MIMEType)
	assert.NotEmpty(t, imgBlock.Data)
}

func TestGetTutorialStep_ExcludesImages(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := getTutorialStepHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"slug": "cap-getting-started", "step": float64(1),
		"track": false, "include_images": false,
	}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	assert.Len(t, result.Content, 1)
	_, ok := result.Content[0].(mcp.TextContent)
	assert.True(t, ok)
}

func TestGetTutorialImage_Valid(t *testing.T) {
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG)
	}))
	defer imgSrv.Close()

	deps := tutorialExecDeps(t)
	handler := getTutorialImageHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"url":  imgSrv.URL + "/screenshot.png",
		"slug": "cap-getting-started",
	}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	require.GreaterOrEqual(t, len(result.Content), 2)
	_, ok := result.Content[0].(mcp.TextContent)
	assert.True(t, ok)
	imgBlock, ok := result.Content[1].(mcp.ImageContent)
	assert.True(t, ok)
	assert.Equal(t, "image/png", imgBlock.MIMEType)
}

func TestGetTutorialImage_BadURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	deps := tutorialExecDeps(t)
	handler := getTutorialImageHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"url":  srv.URL + "/missing.png",
		"slug": "cap-getting-started",
	}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
