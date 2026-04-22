package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recommendDeps(t *testing.T) Deps {
	t.Helper()
	dataDir := t.TempDir()
	cacheDir := t.TempDir()

	tut := &tutorials.Tutorial{
		TutorialMeta: tutorials.TutorialMeta{
			Slug:  "cap-getting-started",
			Title: "Getting Started with CAP",
			Level: "beginner",
			Time:  20,
			Repo:  "Tutorials-en",
		},
		Steps: []tutorials.TutorialStep{
			{Number: 1, Title: "Setup", Content: "step 1"},
			{Number: 2, Title: "Code", Content: "step 2"},
		},
	}
	require.NoError(t, tutorials.SaveContent(cacheDir, tut))

	packs := []*content.Pack{{
		ID:   "cap",
		Name: "CAP",
		TutorialRefs: []content.TutorialRef{
			{Slug: "cap-getting-started", Featured: true, PackID: "cap"},
			{Slug: "cap-deploy-cf", Featured: false, PackID: "cap"},
		},
	}}

	return Deps{
		TutorialIndex: []tutorials.TutorialMeta{tut.TutorialMeta},
		Packs:         packs,
		CacheDir:      cacheDir,
		DataDir:       dataDir,
	}
}

func TestRecommendTutorials_FeaturedOnly(t *testing.T) {
	deps := recommendDeps(t)
	handler := recommendTutorialsHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var resp recommendResult
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resp))

	assert.Empty(t, resp.ActiveTutorials)
	require.Len(t, resp.Recommended, 1)
	assert.Equal(t, "cap-getting-started", resp.Recommended[0].Slug)
	assert.Equal(t, "beginner", resp.Recommended[0].Level)
}

func TestRecommendTutorials_ActiveFirst(t *testing.T) {
	deps := recommendDeps(t)
	handler := recommendTutorialsHandler(deps)

	tutorials.UpdateProgress(deps.DataDir, "cap-getting-started", 1, 2, false)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var resp recommendResult
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resp)

	require.Len(t, resp.ActiveTutorials, 1)
	assert.Equal(t, "cap-getting-started", resp.ActiveTutorials[0].Slug)
	assert.Equal(t, 1, resp.ActiveTutorials[0].CurrentStep)
}

func TestRecommendTutorials_NoPacks(t *testing.T) {
	deps := recommendDeps(t)
	deps.Packs = nil
	handler := recommendTutorialsHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var resp recommendResult
	json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &resp)

	assert.Empty(t, resp.Recommended)
}
