# HTML Content Conversion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `format` and `selector` attributes to `sync:fetch` markers so fetched HTML is converted to clean Markdown (or plain text) before being stored in `context.expanded.md`.

**Architecture:** A new `convertContent(body, format, selector string) (string, []string, error)` helper in `internal/sync/convert.go` handles the select → convert pipeline; `FetchMarker` calls it after reading the body and before truncation. `ScanMarkers` parses the two new attributes and sets defaults.

**Tech Stack:** `github.com/JohannesKaufmann/html-to-markdown/v2` (HTML→Markdown), `github.com/andybalholm/cascadia` (CSS selector engine), `golang.org/x/net/html` (HTML parser, already an indirect dep).

**Spec:** `docs/superpowers/specs/2026-04-15-html-content-conversion-design.md`

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `go.mod` / `go.sum` | Add two new direct dependencies |
| Modify | `internal/sync/marker.go` | Add `Format`+`Selector` to `Marker`; update `ScanMarkers`; wire `convertContent` into `FetchMarker` |
| **Create** | `internal/sync/convert.go` | `convertContent` + `extractText` helpers |
| **Create** | `internal/sync/convert_test.go` | Unit tests for `convertContent` (package `sync`, not `sync_test`) |
| Modify | `internal/sync/marker_test.go` | Add `Format:"raw"` to existing raw-text tests; add new HTML conversion tests |
| Modify | `content/packs/cap/context.md` | Update `sync:fetch` marker with `format`, `selector`, `max_lines=1000` |
| Modify | `docs/content-authoring.md` | Add `format`/`selector` to attribute table; update examples; add pack author guidance |

---

## Task 1: Add dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add html-to-markdown and cascadia**

```bash
cd d:/projects/sap-devs-cli
go get github.com/JohannesKaufmann/html-to-markdown/v2
go get github.com/andybalholm/cascadia
go mod tidy
```

- [ ] **Step 2: Verify build is clean**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add html-to-markdown/v2 and cascadia dependencies"
```

---

## Task 2: Extend `Marker` struct and `ScanMarkers` parsing

**Files:**
- Modify: `internal/sync/marker.go`
- Modify: `internal/sync/marker_test.go`

- [ ] **Step 1: Write failing tests for new `ScanMarkers` behaviour**

Add to `internal/sync/marker_test.go` (inside `package sync_test`):

```go
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
```

- [ ] **Step 2: Verify tests fail**

```bash
go build ./...
```

Expected: compile error — `markers[0].Format` and `markers[0].Selector` undefined.

- [ ] **Step 3: Add `Format` and `Selector` fields to `Marker` struct**

In `internal/sync/marker.go`, extend the struct:

```go
type Marker struct {
	PackID    string
	Index     int    // zero-based position in the file
	URL       string
	MaxLines  int    // 0 = no limit
	MaxTokens int    // 0 = no limit; MaxLines takes precedence when both set
	Label     string
	TTLHours  int    // 0 = use pack/engine default
	LineNum   int
	Format    string // "raw" | "text" | "markdown"; default "markdown"
	Selector  string // CSS selector for DOM scoping; empty = whole body; ignored for "raw"
}
```

- [ ] **Step 4: Parse `format` and `selector` in `ScanMarkers`**

In `ScanMarkers`, after the existing `ttl_hours` parsing block, add:

```go
m.Format = "markdown" // default
if v := attrs["format"]; v != "" {
    switch v {
    case "raw", "text", "markdown":
        m.Format = v
    default:
        warnings = append(warnings, fmt.Sprintf(
            "%s: line %d: unknown format %q — defaulting to markdown", packID, lineNum+1, v,
        ))
    }
}
if v := attrs["selector"]; v != "" {
    m.Selector = v
}
```

- [ ] **Step 5: Build and verify clean**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/sync/marker.go internal/sync/marker_test.go
git commit -m "feat(sync): add Format and Selector fields to Marker struct"
```

---

## Task 3: Implement `convertContent`

**Files:**
- Create: `internal/sync/convert.go`
- Create: `internal/sync/convert_test.go`

- [ ] **Step 1: Write failing tests in a new `convert_test.go`**

Create `internal/sync/convert_test.go` with `package sync` (not `sync_test`) so the unexported function is accessible:

```go
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
	assert.NotEmpty(t, warns)
	assert.Contains(t, warns[0], "unknown format")
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
	_ = result // empty or whitespace is fine
}

func TestExtractText_StripsAllTags(t *testing.T) {
	// Test the helper directly via TextConvertContent
	body := "<div><b>bold</b> and <i>italic</i></div>"
	result, _, err := convertContent(body, "text", "")
	require.NoError(t, err)
	assert.NotContains(t, result, "<b>")
	assert.NotContains(t, result, "<i>")
	assert.Contains(t, result, "bold")
	assert.Contains(t, result, "italic")
}
```

- [ ] **Step 2: Verify tests fail**

```bash
go build ./...
```

Expected: compile error — `convertContent` undefined.

- [ ] **Step 3: Create `internal/sync/convert.go`**

```go
package sync

import (
	"fmt"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

// convertContent applies format post-processing to a fetched response body.
// selector scopes HTML extraction to a matching DOM element before conversion;
// it is silently ignored when format is "raw".
// Returns: processed content, non-fatal warnings (selector miss, invalid selector),
// and any fatal conversion error.
func convertContent(body, format, selector string) (string, []string, error) {
	if format == "raw" {
		return body, nil, nil
	}

	var warns []string

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		// golang.org/x/net/html is very lenient; this path is unlikely in practice.
		warns = append(warns, fmt.Sprintf("html parse error: %v — using raw body", err))
		return body, warns, nil
	}

	root := doc

	if selector != "" {
		sel, compileErr := cascadia.Compile(selector)
		if compileErr != nil {
			warns = append(warns, fmt.Sprintf("invalid selector %q: %v — using full body", selector, compileErr))
		} else {
			match := cascadia.Query(doc, sel)
			if match == nil {
				warns = append(warns, fmt.Sprintf("selector %q matched no elements — using full body", selector))
			} else {
				root = match
			}
		}
	}

	switch format {
	case "text":
		return extractText(root), warns, nil

	case "markdown":
		var buf strings.Builder
		if err := html.Render(&buf, root); err != nil {
			return "", warns, fmt.Errorf("render selected node: %w", err)
		}
		md, err := htmltomarkdown.ConvertString(buf.String())
		if err != nil {
			return "", warns, fmt.Errorf("html-to-markdown conversion: %w", err)
		}
		return md, warns, nil

	default:
		// Unknown format — ScanMarkers already warned at parse time.
		// Fall back to markdown so the content is still useful.
		warns = append(warns, fmt.Sprintf("unknown format %q — defaulting to markdown", format))
		md, moreWarns, err := convertContent(body, "markdown", selector)
		return md, append(warns, moreWarns...), err
	}
}

// extractText recursively walks an HTML node tree and returns all text node content.
func extractText(n *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return buf.String()
}
```

- [ ] **Step 4: Build and verify clean**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/sync/convert.go internal/sync/convert_test.go
git commit -m "feat(sync): implement convertContent helper for HTML format conversion"
```

---

## Task 4: Wire `convertContent` into `FetchMarker` and fix existing tests

**Files:**
- Modify: `internal/sync/marker.go`
- Modify: `internal/sync/marker_test.go`

- [ ] **Step 1: Add `Format: "raw"` to existing raw-text tests**

In `internal/sync/marker_test.go`, update the four tests that serve raw text (not HTML) so they do not trigger HTML conversion:

```go
// TestFetchMarker_Success
m := sapSync.Marker{URL: srv.URL, MaxLines: 5, Format: "raw"}

// TestFetchMarker_NoLimit
m := sapSync.Marker{URL: srv.URL, Format: "raw"}

// TestTruncateLines_ExactBoundary
m := sapSync.Marker{URL: srv.URL, MaxLines: 3, Format: "raw"}

// TestTruncateTokens_NoNewlines
m := sapSync.Marker{URL: srv.URL, MaxTokens: 10, Format: "raw"}
```

`TestFetchMarker_Non200` does not need a change (it errors before reading the body).

- [ ] **Step 2: Add new FetchMarker tests for HTML conversion**

Append to `internal/sync/marker_test.go`:

```go
func TestFetchMarker_DefaultFormatIsMarkdown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<h1>Hello</h1>")
	}))
	defer srv.Close()

	m := sapSync.Marker{URL: srv.URL} // Format not set — defaults to markdown
	content, err := sapSync.FetchMarker(m, srv.Client())
	require.NoError(t, err)
	assert.Contains(t, content, "# Hello")
}

func TestFetchMarker_MarkdownFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<h1>Hello</h1><p>World</p>")
	}))
	defer srv.Close()

	m := sapSync.Marker{URL: srv.URL, Format: "markdown"}
	content, err := sapSync.FetchMarker(m, srv.Client())
	require.NoError(t, err)
	assert.Contains(t, content, "# Hello")
	assert.Contains(t, content, "World")
}

func TestFetchMarker_TruncationAppliedAfterConversion(t *testing.T) {
	// Serve 10 HTML paragraphs; truncate to 3 lines of converted markdown.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for i := 1; i <= 10; i++ {
			fmt.Fprintf(w, "<p>paragraph %d</p>", i)
		}
	}))
	defer srv.Close()

	m := sapSync.Marker{URL: srv.URL, Format: "markdown", MaxLines: 3}
	content, err := sapSync.FetchMarker(m, srv.Client())
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	assert.LessOrEqual(t, len(lines), 3)
}

func TestFetchMarker_SelectorScopesContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><body><nav>skip</nav><main><h1>Keep</h1></main></body></html>`)
	}))
	defer srv.Close()

	m := sapSync.Marker{URL: srv.URL, Format: "markdown", Selector: "main"}
	content, err := sapSync.FetchMarker(m, srv.Client())
	require.NoError(t, err)
	assert.Contains(t, content, "Keep")
	assert.NotContains(t, content, "skip")
}
```

- [ ] **Step 3: Verify tests fail (convertContent not yet wired)**

```bash
go build ./...
```

Expected: compiles but the new tests would fail at runtime — `FetchMarker` still returns raw HTML.

- [ ] **Step 4: Wire `convertContent` into `FetchMarker`**

In `internal/sync/marker.go`, add `"fmt"` and `"os"` to imports if not already present, then replace the body of `FetchMarker` after the `io.ReadAll` call:

Current code to replace:
```go
content := string(body)
if m.MaxLines > 0 {
    content = truncateLines(content, m.MaxLines)
} else if m.MaxTokens > 0 {
    content = truncateTokens(content, m.MaxTokens)
}
return content, nil
```

Replacement:
```go
format := m.Format
if format == "" {
    format = "markdown"
}

content, warns, err := convertContent(string(body), format, m.Selector)
for _, w := range warns {
    fmt.Fprintf(os.Stderr, "WARN  sync:fetch %s\n", w)
}
if err != nil {
    return "", err
}

if m.MaxLines > 0 {
    content = truncateLines(content, m.MaxLines)
} else if m.MaxTokens > 0 {
    content = truncateTokens(content, m.MaxTokens)
}
return content, nil
```

- [ ] **Step 5: Build and verify clean**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/sync/marker.go internal/sync/marker_test.go
git commit -m "feat(sync): wire convertContent into FetchMarker; truncation now applies after conversion"
```

---

## Task 5: Update CAP context.md

**Files:**
- Modify: `content/packs/cap/context.md`

- [ ] **Step 1: Update the `sync:fetch` marker**

In `content/packs/cap/context.md`, replace:

```
<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" max_lines="80" label="CAP Release Notes (feb26)" -->
```

With:

```
<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" format="markdown" selector="main" max_lines="1000" label="CAP Release Notes (feb26)" -->
```

- [ ] **Step 2: Build check**

```bash
go build ./...
```

Expected: no errors (content files are not compiled).

- [ ] **Step 3: Commit**

```bash
git add content/packs/cap/context.md
git commit -m "content(cap): update sync:fetch marker with format, selector, and max_lines=1000"
```

---

## Task 6: Update `docs/content-authoring.md`

**Files:**
- Modify: `docs/content-authoring.md`

- [ ] **Step 1: Update the attribute reference table**

Replace the existing attributes table:

```markdown
| Attribute | Required | Default | Description |
|---|---|---|---|
| `url` | yes | — | URL to fetch. Must be `https://`. |
| `max_lines` | no | — | Truncate fetched content to at most N lines. |
| `max_tokens` | no | — | Truncate fetched content to approx N tokens (1 token ≈ 4 chars). |
| `label` | no | URL | Display label shown in the progress UI during sync. |
| `ttl_hours` | no | `168` (7 days) | Cache TTL in hours. Content is re-fetched after the TTL expires. |
```

With:

```markdown
| Attribute | Required | Default | Description |
|---|---|---|---|
| `url` | yes | — | URL to fetch. Must be `https://`. |
| `format` | no | `markdown` | How to process the response body: `markdown` (HTML→Markdown), `text` (strip all tags), `raw` (no processing). |
| `selector` | no | — | CSS selector to scope the DOM before conversion (e.g. `main`, `article`, `#content`). Ignored for `format="raw"`. |
| `max_lines` | no | — | Truncate fetched content to at most N lines. Applied after conversion. |
| `max_tokens` | no | — | Truncate fetched content to approx N tokens (1 token ≈ 4 chars). Applied after conversion. |
| `label` | no | URL | Display label shown in the progress UI during sync. |
| `ttl_hours` | no | `168` (7 days) | Cache TTL in hours. Content is re-fetched after the TTL expires. |
```

- [ ] **Step 2: Update the Example section**

Replace the existing example:

```markdown
### Example

​```markdown
### Recent CAP Releases

<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" max_lines="80" label="CAP Release Notes (feb26)" -->
​```
```

With:

```markdown
### Example

​```markdown
### Recent CAP Releases

<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" format="markdown" selector="main" max_lines="1000" label="CAP Release Notes (feb26)" -->
​```

For a plain-text or non-HTML source, use `format="raw"`:

​```markdown
<!-- sync:fetch url="https://example.com/status.txt" format="raw" max_lines="20" label="Status" -->
​```
```

- [ ] **Step 3: Update recommended limits table**

Replace the example row in the recommended limits table:

```markdown
| Release notes / changelog | `max_lines="60"` to `max_lines="100"` |
```

With:

```markdown
| Release notes / changelog | `max_lines="1000"` (HTML pages may produce many lines after conversion) |
```

- [ ] **Step 4: Update the `max_lines` guidance paragraph**

In the "Use `max_lines` for release notes and changelogs" paragraph, replace the old example marker:

```markdown
<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" max_lines="80" label="CAP Release Notes (feb26)" -->
```

With:

```markdown
<!-- sync:fetch url="https://cap.cloud.sap/docs/releases/2026/feb26" format="markdown" selector="main" max_lines="1000" label="CAP Release Notes (feb26)" -->
```

Also update the surrounding copy: replace "60–100 lines is a good starting point" with "1000 lines is a safe starting point for HTML documentation pages after conversion."

- [ ] **Step 5: Add a Pack Author Guidance section before the "Testing a New Marker" section**

Insert:

```markdown
## Pack Author Guidance

`format` defaults to `"markdown"`. Both `format="markdown"` and `format="text"` pass the response through an HTML parser. **Always set `format="raw"` for any non-HTML source** — plain text files, JSON endpoints, RSS feeds. Passing non-HTML through the parser is safe (the parser is lenient) but may produce garbled or sparse output.

Use `selector` to scope conversion to the main content area of a page and exclude nav bars, sidebars, and footers. Common values:

| Site type | Recommended selector |
|---|---|
| Generic (try first) | `main` |
| Article / blog | `article` |
| VitePress docs | `main` |
| Role-based | `[role="main"]` |

If `selector` matches nothing, the full page body is used and a warning is printed to stderr. Test your selector by running `sap-devs sync --force` and inspecting the expanded file.
```

- [ ] **Step 5: Commit**

```bash
git add docs/content-authoring.md
git commit -m "docs: update content-authoring with format/selector attributes and pack author guidance"
```

---

## Verification

After all tasks:

- [ ] **Full build and vet**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Local end-to-end smoke test** (optional but recommended)

```bash
SAP_DEVS_DEV=1 go run . sync --force
```

Then inspect the expanded file:
```
# Windows:
%LOCALAPPDATA%\sap-devs\cache\official\content\packs\cap\context.expanded.md
```

Expected: CAP release notes appear as clean Markdown, not raw HTML.
