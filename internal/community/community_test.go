package community_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/community"
)

func TestParsePosts_Count(t *testing.T) {
	data, err := os.ReadFile("testdata/posts.xml")
	require.NoError(t, err)
	posts, err := community.ParsePosts(data)
	require.NoError(t, err)
	assert.Len(t, posts, 2)
}

func TestParsePosts_Fields(t *testing.T) {
	data, err := os.ReadFile("testdata/posts.xml")
	require.NoError(t, err)
	posts, err := community.ParsePosts(data)
	require.NoError(t, err)
	p := posts[0]
	assert.Equal(t, "SAP Developer News - April 11 2026", p.Title)
	assert.Equal(t, "https://community.sap.com/t5/developer-news/apr-11/ba-p/999", p.URL)
	assert.Equal(t, 2026, p.Published.Year())
	assert.Equal(t, time.April, p.Published.Month())
	assert.Equal(t, 11, p.Published.Day())
}

func TestParsePosts_InvalidXML(t *testing.T) {
	_, err := community.ParsePosts([]byte("not xml"))
	assert.Error(t, err)
}

func TestExtractMarkdown_ContainsHeadings(t *testing.T) {
	data, err := os.ReadFile("testdata/post.html")
	require.NoError(t, err)
	md, err := community.ExtractMarkdown(data)
	require.NoError(t, err)
	assert.Contains(t, md, "CAP Updates")
	assert.Contains(t, md, "Parallel batch processing")
}

func TestExtractMarkdown_Empty(t *testing.T) {
	md, err := community.ExtractMarkdown([]byte("<html><body></body></html>"))
	require.NoError(t, err)
	assert.Equal(t, "", md)
}
