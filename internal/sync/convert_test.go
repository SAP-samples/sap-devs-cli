package sync

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertContent_RawPassthrough(t *testing.T) {
	body := "<html><body><p>Hello</p></body></html>"
	result, warns, err := convertContent(body, "raw", "")
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, body, result)
}

func TestConvertContent_RawIgnoresSelector(t *testing.T) {
	body := "<html><body><p>Hello</p></body></html>"
	result, warns, err := convertContent(body, "raw", "p")
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, body, result)
}

func TestConvertContent_TextStripsHTML(t *testing.T) {
	body := "<h1>Title</h1><p>Content here</p>"
	result, warns, err := convertContent(body, "text", "")
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Contains(t, result, "Title")
	assert.Contains(t, result, "Content here")
	assert.NotContains(t, result, "<h1>")
	assert.NotContains(t, result, "<p>")
}

func TestConvertContent_MarkdownConvertsHTML(t *testing.T) {
	body := "<h1>Hello</h1><p>World</p>"
	result, warns, err := convertContent(body, "markdown", "")
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Contains(t, result, "# Hello")
	assert.Contains(t, result, "World")
}

func TestConvertContent_MarkdownWithSelectorHit(t *testing.T) {
	body := `<html><body><nav>nav content</nav><main><h1>Main Title</h1></main></body></html>`
	result, warns, err := convertContent(body, "markdown", "main")
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Contains(t, result, "Main Title")
	assert.NotContains(t, result, "nav content")
}

func TestConvertContent_SelectorMissFallsBackToFullBody(t *testing.T) {
	body := `<html><body><p>Hello</p></body></html>`
	result, warns, err := convertContent(body, "markdown", ".notexist")
	require.NoError(t, err)
	require.Len(t, warns, 1)
	assert.Contains(t, warns[0], "matched no elements")
	assert.Contains(t, result, "Hello")
}

func TestConvertContent_InvalidSelectorFallsBackToFullBody(t *testing.T) {
	body := `<html><body><p>Hello</p></body></html>`
	result, warns, err := convertContent(body, "markdown", "!!invalid!!")
	require.NoError(t, err)
	require.Len(t, warns, 1)
	assert.Contains(t, warns[0], "invalid selector")
	assert.Contains(t, result, "Hello")
}

func TestConvertContent_UnknownFormatWarnsAndConverts(t *testing.T) {
	body := "<p>Hello</p>"
	result, warns, err := convertContent(body, "xml", "")
	require.NoError(t, err)
	assert.Empty(t, warns) // ScanMarkers warns at parse time; convertContent does not re-warn
	assert.Contains(t, result, "Hello")
}

func TestConvertContent_TextWithSelector(t *testing.T) {
	body := `<html><body><nav>skip this</nav><article>keep this</article></body></html>`
	result, warns, err := convertContent(body, "text", "article")
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Contains(t, result, "keep this")
	assert.NotContains(t, result, "skip this")
}

func TestConvertContent_EmptyBody(t *testing.T) {
	result, warns, err := convertContent("", "markdown", "")
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Empty(t, strings.TrimSpace(result))
}

func TestExtractText_StripsAllTags(t *testing.T) {
	body := "<div><b>bold</b> and <i>italic</i></div>"
	result, _, err := convertContent(body, "text", "")
	require.NoError(t, err)
	assert.NotContains(t, result, "<b>")
	assert.NotContains(t, result, "<i>")
	assert.Contains(t, result, "bold")
	assert.Contains(t, result, "italic")
}
