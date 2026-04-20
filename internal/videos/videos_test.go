package videos_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/videos"
)

func TestLoadSaveCache_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	vids := []content.Video{
		{ID: "base/news/abc", Title: "Test Video", VideoID: "abc", PackID: "base", SourceID: "news"},
	}
	require.NoError(t, videos.SaveCache(dir, "base", "news", vids))
	loaded, err := videos.LoadCache(dir, "base", "news")
	require.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Equal(t, "Test Video", loaded[0].Title)
}

func TestCacheAge_NoFile(t *testing.T) {
	dir := t.TempDir()
	age := videos.CacheAge(dir, "base", "news")
	assert.Less(t, age, 0*age) // negative when no file
}

func TestResolveAll_FromFixture(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "youtube", "base")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	data, err := os.ReadFile("testdata/base/sap-dev-news.json")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "sap-dev-news.json"), data, 0644))

	sources := []content.YouTubeSource{
		{ID: "sap-dev-news", Type: "playlist", PackID: "base"},
	}
	vids, err := videos.ResolveAll(sources, dir)
	require.NoError(t, err)
	assert.Len(t, vids, 2)
}

func TestFilterVideos(t *testing.T) {
	vids := []content.Video{
		{ID: "a", Title: "CAP Tutorial", Tags: []string{"cap"}},
		{ID: "b", Title: "ABAP Basics", Tags: []string{"abap"}},
	}
	result := videos.FilterVideos(vids, "cap")
	assert.Len(t, result, 1)
	assert.Equal(t, "a", result[0].ID)
}

func TestFindVideo(t *testing.T) {
	vids := []content.Video{
		{ID: "base/news/abc"},
		{ID: "cap/tut/def"},
	}
	assert.NotNil(t, videos.FindVideo(vids, "cap/tut/def"))
	assert.Nil(t, videos.FindVideo(vids, "nonexistent"))
}
