package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestSearchTutorials(t *testing.T) {
	deps := Deps{
		TutorialIndex: []tutorials.TutorialMeta{
			{Slug: "cap-getting-started", Title: "Getting Started with CAP", Description: "Learn CAP basics", URL: "https://developers.sap.com/tutorials/cap-getting-started.html", Tags: []string{"cap"}},
			{Slug: "abap-adt", Title: "ABAP Development Tools", Description: "ADT setup", URL: "https://developers.sap.com/tutorials/abap-adt.html", Tags: []string{"abap"}},
		},
	}
	handler := searchTutorialsHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "CAP"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var tuts []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &tuts)
	require.NoError(t, err)
	assert.Len(t, tuts, 1)
	assert.Equal(t, "cap-getting-started", tuts[0]["slug"])
}

func TestSearchTutorials_EmptyIndex(t *testing.T) {
	deps := Deps{TutorialIndex: nil}
	handler := searchTutorialsHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "cap"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "[]", result.Content[0].(mcp.TextContent).Text)
}

func TestSearchLearningJourneys(t *testing.T) {
	deps := Deps{
		LearningIndex: []learning.LearningJourney{
			{Slug: "btp-architect", Title: "Becoming a BTP Architect", Level: "INTERMEDIATE", DurationHours: "6.5", URL: "https://learning.sap.com/learning-journeys/btp-architect"},
		},
	}
	handler := searchLearningJourneysHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "architect"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	var journeys []map[string]any
	err = json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &journeys)
	require.NoError(t, err)
	assert.Len(t, journeys, 1)
}

func TestSearchLearningJourneys_EmptyIndex(t *testing.T) {
	deps := Deps{LearningIndex: nil}
	handler := searchLearningJourneysHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "btp"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "[]", result.Content[0].(mcp.TextContent).Text)
}
