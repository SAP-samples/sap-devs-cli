package youtube_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

func TestParsePlaylistItemsResponse(t *testing.T) {
	data, err := os.ReadFile("testdata/apiv3_playlistitems.json")
	require.NoError(t, err)
	items, nextPage, err := youtube.ParsePlaylistItemsResponse(data)
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "vid001", items[0].VideoID)
	assert.Equal(t, "Build a CAP App in 10 Minutes", items[0].Title)
	assert.Empty(t, nextPage)
}

func TestParseVideosResponse(t *testing.T) {
	data, err := os.ReadFile("testdata/apiv3_videos.json")
	require.NoError(t, err)
	details, err := youtube.ParseVideosResponse(data)
	require.NoError(t, err)
	assert.Len(t, details, 2)
	assert.Equal(t, "PT10M30S", details["vid001"].Duration)
	assert.Contains(t, details["vid001"].Tags, "cap")
}
