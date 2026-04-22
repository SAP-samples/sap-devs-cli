# Tutorial Inline Images Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Display tutorial images inline in MCP tool responses so AI agents can see and describe screenshots, with a fallback to resolved URLs for lighter-weight usage.

**Architecture:** Add an image fetching layer to `internal/tutorials/` that downloads PNG/JPEG images from GitHub raw CDN, caches them locally, and base64-encodes them. Modify `get_tutorial_step` to return mixed MCP content (TextContent + ImageContent blocks) by default. Add an `include_images` boolean parameter (default `true`) that when set to `false` falls back to resolved-URL-only mode. Add a standalone `get_tutorial_image` tool for on-demand single-image fetching.

**Tech Stack:** Go, mcp-go (`ImageContent`, `CallToolResult` with mixed content), GitHub raw CDN, base64 encoding

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/tutorials/images.go` | Create | Image fetching, caching, base64 encoding; `ExtractImageRefs` parses image references from markdown; `FetchImage` downloads a single image; `FetchStepImages` downloads all images for a step |
| `internal/tutorials/images_test.go` | Create | Tests for image extraction, URL resolution, caching, fetch with httptest |
| `internal/tutorials/parser.go` | Modify | New `ResolveImageURLsKeepMarkdown` variant that resolves URLs but keeps `![alt](url)` markdown syntax (not converting to `[View image]` links) |
| `internal/tutorials/parser_test.go` | Modify | Add tests for `ResolveImageURLs` and `ResolveImageURLsKeepMarkdown` |
| `internal/mcpserver/tools_tutorial_exec.go` | Modify | Update `get_tutorial_step` to accept `include_images` param, resolve image URLs in content, fetch + return `ImageContent` blocks when enabled; add `get_tutorial_image` tool |
| `internal/mcpserver/tools_tutorial_exec_test.go` | Modify | Tests for image-inclusive and image-exclusive modes, `get_tutorial_image` tool |

---

### Task 1: Extract and resolve image references from markdown

**Files:**
- Create: `internal/tutorials/images.go`
- Create: `internal/tutorials/images_test.go`
- Modify: `internal/tutorials/parser.go:204-226`
- Modify: `internal/tutorials/parser_test.go`

This task creates the image reference extraction function and a URL-resolving variant that keeps markdown image syntax intact (for MCP text content where the agent should see `![alt](full-url)` rather than `[View image](url)`).

- [ ] **Step 1: Write test for `ExtractImageRefs`**

In `internal/tutorials/images_test.go`:

```go
package tutorials_test

import (
	"testing"

	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/stretchr/testify/assert"
)

func TestExtractImageRefs_RelativePaths(t *testing.T) {
	content := "Some text\n![cds commands](cds_commands.png)\nMore text\n![folder](folder_structure.png)\n"
	refs := tutorials.ExtractImageRefs(content, "btp-adai", "main", "cp-apm-nodejs-create-service")
	assert.Len(t, refs, 2)
	assert.Equal(t, "cds commands", refs[0].Alt)
	assert.Equal(t, "cds_commands.png", refs[0].OriginalPath)
	assert.Equal(t, "https://raw.githubusercontent.com/sap-tutorials/btp-adai/main/tutorials/cp-apm-nodejs-create-service/cds_commands.png", refs[0].URL)
}

func TestExtractImageRefs_AbsoluteURLs(t *testing.T) {
	content := "![logo](https://example.com/logo.png)\n"
	refs := tutorials.ExtractImageRefs(content, "repo", "main", "slug")
	assert.Len(t, refs, 1)
	assert.Equal(t, "https://example.com/logo.png", refs[0].URL)
	assert.Equal(t, "https://example.com/logo.png", refs[0].OriginalPath)
}

func TestExtractImageRefs_SkipsTraversals(t *testing.T) {
	content := "![bad](../../secret.png)\n"
	refs := tutorials.ExtractImageRefs(content, "repo", "main", "slug")
	assert.Len(t, refs, 1)
	assert.Equal(t, "../../secret.png", refs[0].URL) // not resolved
}

func TestExtractImageRefs_NoImages(t *testing.T) {
	content := "Just text, no images here.\n"
	refs := tutorials.ExtractImageRefs(content, "repo", "main", "slug")
	assert.Empty(t, refs)
}
```

- [ ] **Step 2: Write test for `ResolveImageURLsKeepMarkdown`**

Append to `internal/tutorials/parser_test.go`:

```go
func TestResolveImageURLs_Relative(t *testing.T) {
	content := "Text\n![alt text](screenshot.png)\nMore text"
	result := tutorials.ResolveImageURLs(content, "btp-adai", "main", "my-tutorial")
	assert.Contains(t, result, "[View image: alt text](https://raw.githubusercontent.com/sap-tutorials/btp-adai/main/tutorials/my-tutorial/screenshot.png)")
	assert.NotContains(t, result, "![")
}

func TestResolveImageURLs_Absolute(t *testing.T) {
	content := "![logo](https://example.com/logo.png)"
	result := tutorials.ResolveImageURLs(content, "repo", "main", "slug")
	assert.Equal(t, content, result) // unchanged
}

func TestResolveImageURLsKeepMarkdown_Relative(t *testing.T) {
	content := "Text\n![alt text](screenshot.png)\nMore text"
	result := tutorials.ResolveImageURLsKeepMarkdown(content, "btp-adai", "main", "my-tutorial")
	assert.Contains(t, result, "![alt text](https://raw.githubusercontent.com/sap-tutorials/btp-adai/main/tutorials/my-tutorial/screenshot.png)")
}

func TestResolveImageURLsKeepMarkdown_Absolute(t *testing.T) {
	content := "![logo](https://example.com/logo.png)"
	result := tutorials.ResolveImageURLsKeepMarkdown(content, "repo", "main", "slug")
	assert.Equal(t, content, result)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/tutorials/... -run "TestExtractImageRefs|TestResolveImageURLs" -v`
Expected: compilation error — `ExtractImageRefs` and `ResolveImageURLsKeepMarkdown` not defined

- [ ] **Step 4: Implement `ImageRef` type and `ExtractImageRefs`**

Create `internal/tutorials/images.go`:

```go
package tutorials

import "fmt"

// ImageRef represents a parsed image reference from tutorial markdown.
type ImageRef struct {
	Alt          string `json:"alt"`
	OriginalPath string `json:"original_path"`
	URL          string `json:"url"`
}

// ExtractImageRefs finds all markdown image references and resolves relative
// paths to full GitHub raw URLs. Absolute URLs and path traversals are left as-is.
func ExtractImageRefs(content, repo, branch, slug string) []ImageRef {
	matches := imageRE.FindAllStringSubmatch(content, -1)
	refs := make([]ImageRef, 0, len(matches))
	for _, m := range matches {
		alt, path := m[1], m[2]
		ref := ImageRef{Alt: alt, OriginalPath: path}
		ref.URL = resolveImagePath(path, repo, branch, slug)
		refs = append(refs, ref)
	}
	return refs
}
```

- [ ] **Step 5: Add `ResolveImageURLsKeepMarkdown` and refactor shared logic**

In `internal/tutorials/parser.go`, refactor the URL resolution into a shared helper and add the new variant. Replace the existing `ResolveImageURLs` function block (lines 206-226) with:

```go
// ResolveImageURLs replaces relative image paths with full GitHub raw content
// URLs rendered as markdown links so glamour doesn't word-wrap the URL.
func ResolveImageURLs(content, repo, branch, slug string) string {
	return imageRE.ReplaceAllStringFunc(content, func(match string) string {
		parts := imageRE.FindStringSubmatch(match)
		alt := parts[1]
		url := resolveImagePath(parts[2], repo, branch, slug)
		if url == parts[2] {
			return match
		}
		if alt != "" {
			return fmt.Sprintf("[View image: %s](%s)", alt, url)
		}
		return fmt.Sprintf("[View image](%s)", url)
	})
}

// ResolveImageURLsKeepMarkdown resolves relative image paths to full GitHub
// raw URLs but preserves the ![alt](url) markdown image syntax. Use this for
// MCP responses where the agent should see image markdown, not link markdown.
func ResolveImageURLsKeepMarkdown(content, repo, branch, slug string) string {
	return imageRE.ReplaceAllStringFunc(content, func(match string) string {
		parts := imageRE.FindStringSubmatch(match)
		url := resolveImagePath(parts[2], repo, branch, slug)
		return fmt.Sprintf("![%s](%s)", parts[1], url)
	})
}

func resolveImagePath(path, repo, branch, slug string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.Contains(path, "..") {
		return path
	}
	path = strings.TrimLeft(path, "/")
	return fmt.Sprintf("%s/sap-tutorials/%s/%s/tutorials/%s/%s", rawBaseURL, repo, branch, slug, path)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/tutorials/... -run "TestExtractImageRefs|TestResolveImageURLs" -v`
Expected: all PASS

- [ ] **Step 7: Run go build and go vet**

Run: `go build ./... && go vet ./...`
Expected: clean

- [ ] **Step 8: Commit**

```bash
git add internal/tutorials/images.go internal/tutorials/images_test.go internal/tutorials/parser.go internal/tutorials/parser_test.go
git commit -m "$(cat <<'EOF'
feat: extract image refs and add markdown-preserving URL resolver

Add ExtractImageRefs for parsing image references from tutorial markdown
and ResolveImageURLsKeepMarkdown for MCP responses that need full URLs
in ![alt](url) syntax rather than [View image](url) link syntax.
EOF
)"
```

---

### Task 2: Image fetching and caching layer

**Files:**
- Modify: `internal/tutorials/images.go`
- Modify: `internal/tutorials/images_test.go`

This task adds the HTTP fetching and local disk caching for tutorial images. Images are cached at `{cacheDir}/tutorials/images/{slug}/{filename}` to avoid repeated downloads.

- [ ] **Step 1: Write test for `FetchImage` with httptest**

Append to `internal/tutorials/images_test.go`:

```go
import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// 1x1 red PNG for testing
var testPNG = func() []byte {
	b, _ := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==")
	return b
}()

func TestFetchImage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	img, err := tutorials.FetchImage(srv.URL+"/test.png", cacheDir, "my-slug")
	require.NoError(t, err)
	assert.Equal(t, "image/png", img.MIMEType)
	assert.NotEmpty(t, img.Data)

	decoded, err := base64.StdEncoding.DecodeString(img.Data)
	require.NoError(t, err)
	assert.Equal(t, testPNG, decoded)
}

func TestFetchImage_CacheHit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()

	// First fetch — populates cache
	_, err := tutorials.FetchImage(srv.URL+"/test.png", cacheDir, "my-slug")
	require.NoError(t, err)
	srv.Close() // kill server

	// Second fetch — must come from cache
	img, err := tutorials.FetchImage(srv.URL+"/test.png", cacheDir, "my-slug")
	require.NoError(t, err)
	assert.Equal(t, "image/png", img.MIMEType)
	assert.NotEmpty(t, img.Data)
}

func TestFetchImage_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	_, err := tutorials.FetchImage(srv.URL+"/missing.png", cacheDir, "my-slug")
	assert.Error(t, err)
}
```

- [ ] **Step 2: Write test for `FetchStepImages`**

Append to `internal/tutorials/images_test.go`:

```go
func TestFetchStepImages_Mixed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG)
	}))
	defer srv.Close()

	refs := []tutorials.ImageRef{
		{Alt: "img1", URL: srv.URL + "/a.png"},
		{Alt: "img2", URL: srv.URL + "/b.png"},
	}
	cacheDir := t.TempDir()
	images := tutorials.FetchStepImages(refs, cacheDir, "my-slug")
	assert.Len(t, images, 2)
	assert.Equal(t, "img1", images[0].Alt)
	assert.NotEmpty(t, images[0].Data)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/tutorials/... -run "TestFetchImage|TestFetchStepImages" -v`
Expected: compilation error — `FetchImage`, `FetchStepImages` not defined

- [ ] **Step 4: Implement `FetchImage` and `FetchStepImages`**

Add to `internal/tutorials/images.go`:

```go
import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FetchedImage holds base64-encoded image data ready for MCP ImageContent.
type FetchedImage struct {
	Alt      string `json:"alt"`
	URL      string `json:"url"`
	Data     string `json:"data"`
	MIMEType string `json:"mime_type"`
}

var imageHTTPClient = &http.Client{Timeout: 15 * time.Second}

// FetchImage downloads an image from url, caches it locally, and returns
// the base64-encoded data with MIME type. Returns cached data on subsequent calls.
func FetchImage(url, cacheDir, slug string) (*FetchedImage, error) {
	filename := filepath.Base(url)
	if filename == "" || filename == "." || filename == "/" {
		return nil, fmt.Errorf("cannot determine filename from URL: %s", url)
	}

	dir := filepath.Join(cacheDir, "tutorials", "images", slug)
	cached := filepath.Join(dir, filename)

	if data, err := os.ReadFile(cached); err == nil {
		mimeType := mimeFromExt(filename)
		return &FetchedImage{
			URL:      url,
			Data:     base64.StdEncoding.EncodeToString(data),
			MIMEType: mimeType,
		}, nil
	}

	resp, err := imageHTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch image %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch image %s: HTTP %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read image %s: %w", url, err)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" || strings.HasPrefix(mimeType, "application/octet-stream") {
		mimeType = mimeFromExt(filename)
	}
	// Strip any charset or parameters from Content-Type
	if mt, _, err := mime.ParseMediaType(mimeType); err == nil {
		mimeType = mt
	}

	// Cache to disk (best-effort)
	if err := os.MkdirAll(dir, 0755); err == nil {
		_ = os.WriteFile(cached, data, 0644)
	}

	return &FetchedImage{
		URL:      url,
		Data:     base64.StdEncoding.EncodeToString(data),
		MIMEType: mimeType,
	}, nil
}

// FetchStepImages fetches all images from the given refs, skipping any that fail.
func FetchStepImages(refs []ImageRef, cacheDir, slug string) []FetchedImage {
	var images []FetchedImage
	for _, ref := range refs {
		img, err := FetchImage(ref.URL, cacheDir, slug)
		if err != nil {
			continue
		}
		img.Alt = ref.Alt
		images = append(images, *img)
	}
	return images
}

func mimeFromExt(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "image/png"
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tutorials/... -run "TestFetchImage|TestFetchStepImages" -v`
Expected: all PASS

- [ ] **Step 6: Run go build and go vet**

Run: `go build ./... && go vet ./...`
Expected: clean

- [ ] **Step 7: Commit**

```bash
git add internal/tutorials/images.go internal/tutorials/images_test.go
git commit -m "$(cat <<'EOF'
feat: add tutorial image fetching with local disk cache

FetchImage downloads images from GitHub raw CDN and caches them
at {cacheDir}/tutorials/images/{slug}/. Returns base64-encoded data
suitable for MCP ImageContent responses. FetchStepImages fetches
all images for a step, skipping failures gracefully.
EOF
)"
```

---

### Task 3: Update `get_tutorial_step` to return inline images

**Files:**
- Modify: `internal/mcpserver/tools_tutorial_exec.go:16-150`
- Modify: `internal/mcpserver/tools_tutorial_exec_test.go`

This is the core change: `get_tutorial_step` gains an `include_images` parameter (default `true`). When enabled, it fetches all step images and returns them as `ImageContent` blocks in the MCP response alongside the JSON `TextContent`. Image URLs in the text content are always resolved regardless of mode. An `images` field is added to the JSON listing available image URLs and alt text.

- [ ] **Step 1: Write test for `include_images=true` (default)**

Append to `internal/mcpserver/tools_tutorial_exec_test.go`. The test needs a tutorial with image references and a mock HTTP server to serve the images:

```go
import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
)

// 1x1 red PNG for testing
var testPNG = func() []byte {
	b, _ := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==")
	return b
}()

func TestGetTutorialStep_IncludesImages(t *testing.T) {
	// Serve a fake image
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG)
	}))
	defer imgSrv.Close()

	deps := tutorialExecDeps(t)
	// Overwrite step content with an image referencing our test server
	tut, _ := tutorials.LoadContent(deps.CacheDir, "cap-getting-started")
	tut.Steps[0].Content = fmt.Sprintf("Install CDS:\n\n![cds commands](%s/cds_commands.png)\n", imgSrv.URL)
	tut.Repo = "test-repo"
	tutorials.SaveContent(deps.CacheDir, tut)

	handler := getTutorialStepHandler(deps)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"slug": "cap-getting-started", "step": float64(1), "track": false}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	// Should have TextContent + at least one ImageContent
	assert.GreaterOrEqual(t, len(result.Content), 2, "expected text + image content blocks")

	// First block is text (JSON)
	textBlock, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "first content block should be TextContent")

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(textBlock.Text), &resp))
	assert.Equal(t, "cap-getting-started", resp["slug"])

	// Check images field in JSON
	images, ok := resp["images"].([]any)
	assert.True(t, ok, "expected images array in response")
	assert.Len(t, images, 1)

	// Second block is an ImageContent
	imgBlock, ok := result.Content[1].(mcp.ImageContent)
	require.True(t, ok, "second content block should be ImageContent")
	assert.Equal(t, "image/png", imgBlock.MIMEType)
	assert.NotEmpty(t, imgBlock.Data)
}

func TestGetTutorialStep_ExcludesImages(t *testing.T) {
	deps := tutorialExecDeps(t)
	handler := getTutorialStepHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"slug": "cap-getting-started", "step": float64(1),
		"track": false, "include_images": false,
	}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)

	// Should only have one TextContent block
	assert.Len(t, result.Content, 1)
	_, ok := result.Content[0].(mcp.TextContent)
	assert.True(t, ok)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcpserver/... -run "TestGetTutorialStep_IncludesImages|TestGetTutorialStep_ExcludesImages" -v`
Expected: FAIL — no `include_images` parameter, no `images` field, only `TextContent` returned

- [ ] **Step 3: Add `include_images` parameter to tool registration**

In `internal/mcpserver/tools_tutorial_exec.go`, update the `get_tutorial_step` tool registration (line 18-24):

```go
s.AddTool(
    mcp.NewTool("get_tutorial_step",
        mcp.WithDescription("Get a single step from an SAP tutorial with content, annotations (executable commands, file creates, verifications), and progress. Use to guide users through tutorials step-by-step. First call for an uncached tutorial triggers a GitHub fetch. When include_images is true (default), tutorial images are fetched and returned inline as MCP ImageContent blocks that you can see and describe to the user."),
        mcp.WithString("slug", mcp.Required(), mcp.Description("Tutorial slug (e.g., 'cap-getting-started')")),
        mcp.WithNumber("step", mcp.Description("Step number, 1-indexed (default 1)")),
        mcp.WithBoolean("track", mcp.Description("If true (default), creates/updates progress. Set false to preview without starting.")),
        mcp.WithBoolean("include_images", mcp.Description("If true (default), fetch tutorial images and return them inline as ImageContent blocks. Set false for text-only mode with resolved image URLs.")),
    ),
    getTutorialStepHandler(deps),
)
```

- [ ] **Step 4: Add `images` field to `stepResult` and resolve branch**

Update `stepResult` struct to include an images field:

```go
type stepResult struct {
    Slug          string            `json:"slug"`
    Title         string            `json:"title"`
    Step          stepContent       `json:"step"`
    TotalSteps    int               `json:"total_steps"`
    YouWillLearn  []string          `json:"you_will_learn,omitempty"`
    Progress      *progressSnapshot `json:"progress,omitempty"`
    PrevStepTitle *string           `json:"prev_step_title"`
    NextStepTitle *string           `json:"next_step_title"`
    Level         string            `json:"level,omitempty"`
    Time          int               `json:"time,omitempty"`
    Images        []imageRef        `json:"images,omitempty"`
}

type imageRef struct {
    Alt string `json:"alt"`
    URL string `json:"url"`
}
```

- [ ] **Step 5: Implement the image-aware handler logic**

Rewrite the `getTutorialStepHandler` function body. Key changes:
1. Read `include_images` param (default `true`)
2. Resolve branch for the tutorial's repo (same logic as `loadOrFetchTutorial`)
3. Always resolve image URLs in step content using `ResolveImageURLsKeepMarkdown`
4. Extract image refs for the `images` field
5. When `include_images` is `true`, fetch images and build mixed `CallToolResult`
6. When `false`, return `TextContent`-only with resolved URLs

```go
func getTutorialStepHandler(deps Deps) server.ToolHandlerFunc {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        slug, err := req.RequireString("slug")
        if err != nil {
            return mcp.NewToolResultError("slug parameter is required"), nil
        }
        stepNum := req.GetInt("step", 1)
        track := req.GetBool("track", true)
        includeImages := req.GetBool("include_images", true)

        meta := tutorials.FindBySlug(deps.TutorialIndex, slug)
        if meta == nil {
            return mcp.NewToolResultError(fmt.Sprintf("Tutorial %q not found. Use search_tutorials to find valid slugs.", slug)), nil
        }

        tut, err := loadOrFetchTutorial(deps, meta)
        if err != nil {
            return mcp.NewToolResultError(fmt.Sprintf("Failed to load tutorial %q: %v", slug, err)), nil
        }

        if stepNum < 1 || stepNum > len(tut.Steps) {
            return mcp.NewToolResultError(fmt.Sprintf("Step %d out of range. Valid range: 1..%d", stepNum, len(tut.Steps))), nil
        }

        step := tut.Steps[stepNum-1]

        // Resolve branch for image URL construction
        branch := resolveBranch(deps, meta.Repo)

        // Always resolve image URLs in content (keep markdown image syntax)
        resolvedContent := tutorials.ResolveImageURLsKeepMarkdown(step.Content, meta.Repo, branch, slug)
        annotations := tutorials.AnnotateStep(step.Content)

        // Extract image refs for the images field
        imgRefs := tutorials.ExtractImageRefs(step.Content, meta.Repo, branch, slug)
        var imgRefList []imageRef
        for _, ref := range imgRefs {
            imgRefList = append(imgRefList, imageRef{Alt: ref.Alt, URL: ref.URL})
        }

        var prevTitle, nextTitle *string
        if stepNum > 1 {
            t := tut.Steps[stepNum-2].Title
            prevTitle = &t
        }
        if stepNum < len(tut.Steps) {
            t := tut.Steps[stepNum].Title
            nextTitle = &t
        }

        var ps *progressSnapshot
        if track {
            if err := tutorials.UpdateProgress(deps.DataDir, slug, stepNum, len(tut.Steps), false); err != nil {
                return mcp.NewToolResultError(fmt.Sprintf("Failed to update progress: %v", err)), nil
            }
        }
        if p, _ := tutorials.GetProgress(deps.DataDir, slug); p != nil {
            ps = &progressSnapshot{
                CompletedSteps: p.CompletedSteps,
                CurrentStep:    p.CurrentStep,
                TotalSteps:     len(tut.Steps),
                StartedAt:      p.StartedAt.Format("2006-01-02T15:04:05Z"),
                LastAccessed:   p.LastAccessed.Format("2006-01-02T15:04:05Z"),
            }
        }

        result := stepResult{
            Slug:          slug,
            Title:         tut.Title,
            Step:          stepContent{Number: step.Number, Title: step.Title, Content: resolvedContent, Annotations: annotations},
            TotalSteps:    len(tut.Steps),
            YouWillLearn:  tut.YouWillLearn,
            Progress:      ps,
            PrevStepTitle: prevTitle,
            NextStepTitle: nextTitle,
            Level:         meta.Level,
            Time:          meta.Time,
            Images:        imgRefList,
        }

        b, _ := json.Marshal(result)
        textContent := mcp.TextContent{Type: "text", Text: string(b)}

        if !includeImages || len(imgRefs) == 0 {
            return &mcp.CallToolResult{Content: []mcp.Content{textContent}}, nil
        }

        // Fetch images and build mixed content response
        fetched := tutorials.FetchStepImages(imgRefs, deps.CacheDir, slug)
        content := []mcp.Content{textContent}
        for _, img := range fetched {
            content = append(content, mcp.ImageContent{
                Type:     "image",
                Data:     img.Data,
                MIMEType: img.MIMEType,
            })
        }
        return &mcp.CallToolResult{Content: content}, nil
    }
}

func resolveBranch(deps Deps, repo string) string {
    repos, _ := tutorials.LoadRepoInfo(deps.CacheDir)
    for _, r := range repos {
        if r.Name == repo {
            return r.DefaultBranch
        }
    }
    return "main"
}
```

- [ ] **Step 6: Add `encoding/base64` to imports if needed, ensure compilation**

The file needs these imports:

```go
import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "sort"

    "github.com/SAP-samples/sap-devs-cli/internal/credentials"
    "github.com/SAP-samples/sap-devs-cli/internal/tutorials"
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)
```

- [ ] **Step 7: Run tests**

Run: `go test ./internal/mcpserver/... -run "TestGetTutorialStep" -v`
Expected: all PASS

- [ ] **Step 8: Run go build and go vet**

Run: `go build ./... && go vet ./...`
Expected: clean

- [ ] **Step 9: Commit**

```bash
git add internal/mcpserver/tools_tutorial_exec.go internal/mcpserver/tools_tutorial_exec_test.go
git commit -m "$(cat <<'EOF'
feat: return inline images in get_tutorial_step MCP responses

Default behavior fetches tutorial images from GitHub, caches locally,
and returns them as MCP ImageContent blocks alongside the JSON text.
Agents can see and describe screenshots. Set include_images=false for
text-only mode with resolved URLs. Images field always lists available
image URLs regardless of mode.
EOF
)"
```

---

### Task 4: Add `get_tutorial_image` standalone tool

**Files:**
- Modify: `internal/mcpserver/tools_tutorial_exec.go`
- Modify: `internal/mcpserver/tools_tutorial_exec_test.go`

This adds a standalone tool for agents to fetch a single image on demand — useful when `include_images=false` was used but the agent wants to inspect a specific image.

- [ ] **Step 1: Write test for `get_tutorial_image`**

Append to `internal/mcpserver/tools_tutorial_exec_test.go`:

```go
func TestGetTutorialImage_Valid(t *testing.T) {
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG)
	}))
	defer imgSrv.Close()

	deps := tutorialExecDeps(t)
	handler := getTutorialImageHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"url":  imgSrv.URL + "/screenshot.png",
		"slug": "cap-getting-started",
	}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Should have TextContent + ImageContent
	require.GreaterOrEqual(t, len(result.Content), 2)
	_, ok := result.Content[0].(mcp.TextContent)
	assert.True(t, ok)
	imgBlock, ok := result.Content[1].(mcp.ImageContent)
	assert.True(t, ok)
	assert.Equal(t, "image/png", imgBlock.MIMEType)
}

func TestGetTutorialImage_BadURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	deps := tutorialExecDeps(t)
	handler := getTutorialImageHandler(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"url":  srv.URL + "/missing.png",
		"slug": "cap-getting-started",
	}
	result, err := handler(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcpserver/... -run "TestGetTutorialImage" -v`
Expected: compilation error — `getTutorialImageHandler` not defined

- [ ] **Step 3: Implement `get_tutorial_image` tool**

In `internal/mcpserver/tools_tutorial_exec.go`, add to `registerTutorialExecTools`:

```go
s.AddTool(
    mcp.NewTool("get_tutorial_image",
        mcp.WithDescription("Fetch a single tutorial image by URL and return it inline as an ImageContent block. Use this when include_images was false in get_tutorial_step but you need to inspect a specific image. The image URLs are available in the 'images' field of get_tutorial_step responses."),
        mcp.WithString("url", mcp.Required(), mcp.Description("Full URL to the tutorial image (from the images field of get_tutorial_step)")),
        mcp.WithString("slug", mcp.Required(), mcp.Description("Tutorial slug (for caching)")),
    ),
    getTutorialImageHandler(deps),
)
```

Add the handler function:

```go
func getTutorialImageHandler(deps Deps) server.ToolHandlerFunc {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        url, err := req.RequireString("url")
        if err != nil {
            return mcp.NewToolResultError("url parameter is required"), nil
        }
        slug, err := req.RequireString("slug")
        if err != nil {
            return mcp.NewToolResultError("slug parameter is required"), nil
        }

        img, err := tutorials.FetchImage(url, deps.CacheDir, slug)
        if err != nil {
            return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch image: %v", err)), nil
        }

        return &mcp.CallToolResult{
            Content: []mcp.Content{
                mcp.TextContent{Type: "text", Text: fmt.Sprintf("Tutorial image from %s", url)},
                mcp.ImageContent{Type: "image", Data: img.Data, MIMEType: img.MIMEType},
            },
        }, nil
    }
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/mcpserver/... -run "TestGetTutorialImage" -v`
Expected: all PASS

- [ ] **Step 5: Run go build and go vet**

Run: `go build ./... && go vet ./...`
Expected: clean

- [ ] **Step 6: Commit**

```bash
git add internal/mcpserver/tools_tutorial_exec.go internal/mcpserver/tools_tutorial_exec_test.go
git commit -m "$(cat <<'EOF'
feat: add get_tutorial_image MCP tool for on-demand image fetching

Standalone tool to fetch a single tutorial image by URL and return
it as an MCP ImageContent block. Useful when include_images=false
was used but the agent wants to inspect a specific screenshot.
EOF
)"
```

---

### Task 5: Update documentation and tool count

**Files:**
- Modify: `CLAUDE.md` (tool count in MCP serve description)
- Modify: `internal/mcpserver/server.go` (server instructions)

- [ ] **Step 1: Update MCP tool count and instructions**

In `internal/mcpserver/server.go`, the `WithInstructions` string needs to mention `get_tutorial_image`. Add after the existing tutorial tool mentions:

```
Use get_tutorial_image to fetch and view a specific tutorial image when include_images was set to false.
```

- [ ] **Step 2: Update CLAUDE.md tool count**

In `CLAUDE.md`, search for `mcp list/install/status/serve` in the CLI Commands table. The tool count (currently 31) should be updated to 32 with `get_tutorial_image` added to the list.

- [ ] **Step 3: Run go build and go vet**

Run: `go build ./... && go vet ./...`
Expected: clean

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md internal/mcpserver/server.go
git commit -m "$(cat <<'EOF'
docs: update MCP tool count and instructions for tutorial images
EOF
)"
```

---

### Task 6: Full integration verification

- [ ] **Step 1: Run the full test suite**

Run: `go test ./internal/tutorials/... ./internal/mcpserver/... -v`
Expected: all PASS

- [ ] **Step 2: Build the binary**

Run: `go build -o sap-devs.exe .`
Expected: clean build

- [ ] **Step 3: Manual smoke test with MCP**

Start the MCP server and test `get_tutorial_step` with a real tutorial to verify images are returned:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./sap-devs.exe mcp serve
```

Then test `get_tutorial_step` with a tutorial that has images and verify the response includes `ImageContent` blocks.

- [ ] **Step 4: Final commit if any fixups needed**
