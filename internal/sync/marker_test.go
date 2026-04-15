package sync_test

import (
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
