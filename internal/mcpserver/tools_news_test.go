package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/news"
	"github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

func TestGetRecentNews_WithItems(t *testing.T) {
	items := []news.NewsItem{
		{Episode: youtube.Episode{Title: "Episode 1", URL: "https://yt/1", Published: time.Now()}},
		{Episode: youtube.Episode{Title: "Episode 2", URL: "https://yt/2", Published: time.Now()}},
	}
	fetcher := &newsFetcher{
		cached:    items,
		fetchedAt: time.Now(),
	}
	handler := getRecentNewsHandler(fetcher)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"limit": float64(1)}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	env := unmarshalEnvelope(t, result)
	out := env.resultSlice(t)
	assert.Len(t, out, 1)
	assert.Equal(t, "Episode 1", out[0]["title"])
}
