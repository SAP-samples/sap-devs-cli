package youtube_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

func TestParsePlaylistFeed_Count(t *testing.T) {
	data, err := os.ReadFile("testdata/playlist.xml")
	require.NoError(t, err)
	episodes, err := youtube.ParseFeed(data)
	require.NoError(t, err)
	assert.Len(t, episodes, 2)
}

func TestParsePlaylistFeed_Fields(t *testing.T) {
	data, err := os.ReadFile("testdata/playlist.xml")
	require.NoError(t, err)
	episodes, err := youtube.ParseFeed(data)
	require.NoError(t, err)
	e := episodes[0]
	assert.Equal(t, "abc123", e.ID)
	assert.Equal(t, "SAP Developer News Apr 11 2026", e.Title)
	assert.Equal(t, "https://www.youtube.com/watch?v=abc123", e.URL)
	assert.Equal(t, "CAP updates and BTP news this week.", e.Description)
	assert.Equal(t, 2026, e.Published.Year())
	assert.Equal(t, time.April, e.Published.Month())
	assert.Equal(t, 11, e.Published.Day())
}

func TestParsePlaylistFeed_OrderPreserved(t *testing.T) {
	data, err := os.ReadFile("testdata/playlist.xml")
	require.NoError(t, err)
	episodes, err := youtube.ParseFeed(data)
	require.NoError(t, err)
	assert.Equal(t, "abc123", episodes[0].ID)
	assert.Equal(t, "def456", episodes[1].ID)
}

func TestParsePlaylistFeed_InvalidXML(t *testing.T) {
	_, err := youtube.ParseFeed([]byte("not xml"))
	assert.Error(t, err)
}
