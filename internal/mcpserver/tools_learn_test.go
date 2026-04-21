package mcpserver

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
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

	env := unmarshalEnvelope(t, result)
	tuts := env.resultSlice(t)
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

	env := unmarshalEnvelope(t, result)
	assert.Equal(t, 0, env.Count)
	assert.Equal(t, 0, env.Total)
	assert.Contains(t, env.Hint, "No tutorials loaded")
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

	env := unmarshalEnvelope(t, result)
	journeys := env.resultSlice(t)
	assert.Len(t, journeys, 1)
}

func TestSearchLearningJourneys_EmptyIndex(t *testing.T) {
	deps := Deps{LearningIndex: nil}
	handler := searchLearningJourneysHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "btp"}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	env := unmarshalEnvelope(t, result)
	assert.Equal(t, 0, env.Count)
	assert.Equal(t, 0, env.Total)
	assert.Contains(t, env.Hint, "No learning journeys loaded")
}
