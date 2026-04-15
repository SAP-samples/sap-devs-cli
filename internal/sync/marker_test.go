package sync_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
)

func TestScanMarkers_Basic(t *testing.T) {
	content := `## Section
<!-- sync:fetch url="https://example.com/notes" max_lines="50" label="Release Notes" -->
## Other
`
	markers, warns := sapSync.ScanMarkers("cap", content)
	require.Empty(t, warns)
	require.Len(t, markers, 1)
	assert.Equal(t, "cap", markers[0].PackID)
	assert.Equal(t, 0, markers[0].Index)
	assert.Equal(t, "https://example.com/notes", markers[0].URL)
	assert.Equal(t, 50, markers[0].MaxLines)
	assert.Equal(t, "Release Notes", markers[0].Label)
}

func TestScanMarkers_SkipsInsideCodeFence(t *testing.T) {
	content := "```markdown\n<!-- sync:fetch url=\"https://example.com\" -->\n```\n"
	markers, warns := sapSync.ScanMarkers("cap", content)
	assert.Empty(t, warns)
	assert.Empty(t, markers)
}

func TestScanMarkers_MalformedMissingURL(t *testing.T) {
	content := `<!-- sync:fetch max_lines="10" -->` + "\n"
	markers, warns := sapSync.ScanMarkers("cap", content)
	assert.Empty(t, markers)
	require.Len(t, warns, 1)
	assert.Contains(t, warns[0], "missing required 'url'")
}

func TestScanMarkers_BothBudgetsWarnMaxLinesWins(t *testing.T) {
	content := `<!-- sync:fetch url="https://x.com" max_lines="20" max_tokens="500" -->` + "\n"
	markers, warns := sapSync.ScanMarkers("cap", content)
	require.Len(t, markers, 1)
	assert.Equal(t, 20, markers[0].MaxLines)
	require.Len(t, warns, 1)
	assert.Contains(t, warns[0], "max_lines takes precedence")
}

func TestScanMarkers_MultipleMarkers(t *testing.T) {
	content := "<!-- sync:fetch url=\"https://a.com\" -->\n## Mid\n<!-- sync:fetch url=\"https://b.com\" -->\n"
	markers, warns := sapSync.ScanMarkers("cap", content)
	assert.Empty(t, warns)
	require.Len(t, markers, 2)
	assert.Equal(t, 0, markers[0].Index)
	assert.Equal(t, 1, markers[1].Index)
	assert.Equal(t, "https://a.com", markers[0].URL)
	assert.Equal(t, "https://b.com", markers[1].URL)
}

func TestExpandMarkers_ReplacesAtPosition(t *testing.T) {
	content := "## Before\n<!-- sync:fetch url=\"https://x.com\" -->\n## After\n"
	markers, _ := sapSync.ScanMarkers("cap", content)
	results := map[int]string{0: "Fetched content here"}
	expanded := sapSync.ExpandMarkers(content, markers, results)
	assert.Contains(t, expanded, "Fetched content here")
	assert.Contains(t, expanded, "## Before")
	assert.Contains(t, expanded, "## After")
	assert.NotContains(t, expanded, "sync:fetch")
}

func TestExpandMarkers_SkipsInsideCodeFence(t *testing.T) {
	content := "```\n<!-- sync:fetch url=\"https://x.com\" -->\n```\n"
	markers, _ := sapSync.ScanMarkers("cap", content)
	results := map[int]string{0: "should not appear"}
	expanded := sapSync.ExpandMarkers(content, markers, results)
	// No markers found (inside fence) → no substitution
	assert.NotContains(t, expanded, "should not appear")
}

func TestExpandMarkers_FenceSkipDirect(t *testing.T) {
	content := "```\n<!-- sync:fetch url=\"https://x.com\" -->\n```\n"
	// Construct a marker manually — LineNum 2 points to the marker line inside the fence
	m := sapSync.Marker{PackID: "cap", Index: 0, URL: "https://x.com", LineNum: 2}
	results := map[int]string{0: "should not appear"}
	expanded := sapSync.ExpandMarkers(content, []sapSync.Marker{m}, results)
	assert.NotContains(t, expanded, "should not appear")
}

func TestFetchMarker_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 10; i++ {
			fmt.Fprintf(w, "line %d\n", i+1)
		}
	}))
	defer srv.Close()

	m := sapSync.Marker{URL: srv.URL, MaxLines: 5}
	content, err := sapSync.FetchMarker(m, srv.Client())
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	assert.Len(t, lines, 5)
	assert.Equal(t, "line 1", lines[0])
}

func TestFetchMarker_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := sapSync.Marker{URL: srv.URL}
	_, err := sapSync.FetchMarker(m, srv.Client())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestFetchMarker_NoLimit(t *testing.T) {
	body := "line1\nline2\nline3\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	m := sapSync.Marker{URL: srv.URL} // no max_lines
	content, err := sapSync.FetchMarker(m, srv.Client())
	require.NoError(t, err)
	assert.Equal(t, body, content)
}

func TestTruncateLines_ExactBoundary(t *testing.T) {
	// Content with exactly max lines — should be returned unchanged.
	content := "a\nb\nc"
	markers, _ := sapSync.ScanMarkers("cap", "<!-- sync:fetch url=\"https://x.com\" max_lines=\"3\" -->\n")
	require.Len(t, markers, 1)
	assert.Equal(t, 3, markers[0].MaxLines)
	// Verify via FetchMarker with a test server returning exactly 3 lines.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, content)
	}))
	defer srv.Close()
	m := sapSync.Marker{URL: srv.URL, MaxLines: 3}
	got, err := sapSync.FetchMarker(m, srv.Client())
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestTruncateTokens_NoNewlines(t *testing.T) {
	// Content with no newlines that exceeds the token budget.
	// Should return a character-cut slice (no newline to snap back to).
	longLine := strings.Repeat("x", 200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, longLine)
	}))
	defer srv.Close()
	m := sapSync.Marker{URL: srv.URL, MaxTokens: 10} // budget = 40 chars
	got, err := sapSync.FetchMarker(m, srv.Client())
	require.NoError(t, err)
	assert.LessOrEqual(t, len(got), 40)
	assert.Greater(t, len(got), 0)
}

func TestScanMarkers_DefaultFormatIsMarkdown(t *testing.T) {
	content := "<!-- sync:fetch url=\"https://x.com\" -->\n"
	markers, warns := sapSync.ScanMarkers("cap", content)
	assert.Empty(t, warns)
	require.Len(t, markers, 1)
	assert.Equal(t, "markdown", markers[0].Format)
}

func TestScanMarkers_FormatRaw(t *testing.T) {
	content := "<!-- sync:fetch url=\"https://x.com\" format=\"raw\" -->\n"
	markers, warns := sapSync.ScanMarkers("cap", content)
	assert.Empty(t, warns)
	require.Len(t, markers, 1)
	assert.Equal(t, "raw", markers[0].Format)
}

func TestScanMarkers_Selector(t *testing.T) {
	content := "<!-- sync:fetch url=\"https://x.com\" selector=\"main\" -->\n"
	markers, warns := sapSync.ScanMarkers("cap", content)
	assert.Empty(t, warns)
	require.Len(t, markers, 1)
	assert.Equal(t, "main", markers[0].Selector)
}

func TestScanMarkers_UnknownFormatWarnsAndDefaultsToMarkdown(t *testing.T) {
	content := "<!-- sync:fetch url=\"https://x.com\" format=\"pdf\" -->\n"
	markers, warns := sapSync.ScanMarkers("cap", content)
	require.Len(t, markers, 1)
	assert.Equal(t, "markdown", markers[0].Format)
	require.Len(t, warns, 1)
	assert.Contains(t, warns[0], "unknown format")
}
