# `sap-devs news` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs news` to list, open, search, and read SAP Developer News episodes fetched live from YouTube RSS and SAP Community, with a footer showing LinkedIn and YouTube Music podcast links.

**Architecture:** Three internal packages (`internal/youtube`, `internal/community`, `internal/news`) handle fetch/parse and correlation independently. `cmd/news.go` is thin cobra wiring that calls them. No caching, no sync integration — every invocation fetches live. `html-to-markdown/v2` (already in go.mod) converts Community post HTML to readable markdown for `news read`.

**Tech Stack:** Go stdlib `encoding/xml`, `net/http`, `os/exec`; `golang.org/x/net/html` (already transitive); `github.com/JohannesKaufmann/html-to-markdown/v2` (already in go.mod); `github.com/pkg/browser` (already used in `cmd/resources.go`); `github.com/spf13/cobra`.

---

## Key Context for Every Task

**Module path:** `github.com/SAP-samples/sap-devs-cli`

**Build/vet (no `go test` on Windows — Windows Defender blocks binary execution from .config paths):**
```bash
go build ./...
go vet ./...
```

**Run tests (CI authoritative; use on Linux/CI or in worktree under project root):**
```bash
go test ./internal/youtube/...
go test ./internal/community/...
go test ./internal/news/...
```

**Test package convention:** Use `package youtube_test` (external test package), `require` for fatal assertions, `assert` for non-fatal — matches `internal/content/resources_test.go`.

**HTTP pattern (from `internal/sync/marker.go`):**
```go
client := &http.Client{Timeout: 10 * time.Second}
resp, err := client.Get(url)
if err != nil { return nil, err }
defer resp.Body.Close()
if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
}
```

**html-to-markdown usage (from `internal/sync/convert.go`):**
```go
import htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
md, err := htmltomarkdown.ConvertString(htmlString)
```

**Browser open (from `cmd/resources.go`):**
```go
import "github.com/pkg/browser"
browser.OpenURL(url)
```

**Worktree note:** Run all commands from the worktree root. Create worktree under `.worktrees/` in the project root (not under `~/.config` — Windows Defender blocks test binary execution from that path).

---

## File Map

| File | Change |
| --- | --- |
| `internal/youtube/youtube.go` | **Create** — `Episode` type, `FetchPlaylist(url string) ([]Episode, error)` |
| `internal/youtube/youtube_test.go` | **Create** — fixture-based RSS parsing tests |
| `internal/youtube/testdata/playlist.xml` | **Create** — minimal YouTube Atom RSS fixture (2 entries) |
| `internal/community/community.go` | **Create** — `BlogPost` type, `FetchBlogPosts`, `FetchPostContent` |
| `internal/community/community_test.go` | **Create** — RSS parse + HTML→markdown extraction tests |
| `internal/community/testdata/posts.xml` | **Create** — minimal RSS 2.0 fixture (2 items) |
| `internal/community/testdata/post.html` | **Create** — minimal HTML blog post fixture |
| `internal/news/news.go` | **Create** — `NewsItem` type, `Correlate(episodes, posts)` |
| `internal/news/news_test.go` | **Create** — correlation unit tests (4 cases) |
| `cmd/news.go` | **Create** — cobra command tree: list, latest, open, search, read |
| `cmd/root.go` | **Modify** — register `newsCmd` in `init()` |

---

## Task 1: YouTube RSS package

**Files:**
- Create: `internal/youtube/youtube.go`
- Create: `internal/youtube/youtube_test.go`
- Create: `internal/youtube/testdata/playlist.xml`

- [ ] **Step 1: Create the testdata fixture**

Create `internal/youtube/testdata/playlist.xml`:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015"
      xmlns:media="http://search.yahoo.com/mrss/"
      xmlns="http://www.w3.org/2005/Atom">
  <title>SAP Developer News</title>
  <entry>
    <yt:videoId>abc123</yt:videoId>
    <title>SAP Developer News Apr 11 2026</title>
    <link rel="alternate" href="https://www.youtube.com/watch?v=abc123"/>
    <published>2026-04-11T15:00:00+00:00</published>
    <media:group>
      <media:description>CAP updates and BTP news this week.</media:description>
    </media:group>
  </entry>
  <entry>
    <yt:videoId>def456</yt:videoId>
    <title>SAP Developer News Apr 4 2026</title>
    <link rel="alternate" href="https://www.youtube.com/watch?v=def456"/>
    <published>2026-04-04T15:00:00+00:00</published>
    <media:group>
      <media:description>ABAP Cloud and UI5 highlights.</media:description>
    </media:group>
  </entry>
</feed>
```

- [ ] **Step 2: Write the failing tests**

Create `internal/youtube/youtube_test.go`:
```go
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
```

- [ ] **Step 3: Run tests — verify they fail**

```bash
go test ./internal/youtube/...
```
Expected: FAIL — package does not exist yet.

- [ ] **Step 4: Implement `internal/youtube/youtube.go`**

```go
package youtube

import (
    "encoding/xml"
    "fmt"
    "io"
    "net/http"
    "time"
)

// Episode is a single SAP Developer News video from the YouTube playlist.
type Episode struct {
    ID          string
    Title       string
    URL         string
    Published   time.Time
    Description string
}

type atomFeed struct {
    XMLName xml.Name    `xml:"feed"`
    Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
    VideoID   string     `xml:"videoId"`
    Title     string     `xml:"title"`
    Link      atomLink   `xml:"link"`
    Published string     `xml:"published"`
    Group     mediaGroup `xml:"group"`
}

type atomLink struct {
    Href string `xml:"href,attr"`
}

type mediaGroup struct {
    Description string `xml:"description"`
}

// ParseFeed parses a YouTube Atom RSS feed and returns the episodes in feed order.
func ParseFeed(data []byte) ([]Episode, error) {
    var feed atomFeed
    if err := xml.Unmarshal(data, &feed); err != nil {
        return nil, fmt.Errorf("youtube: parse feed: %w", err)
    }
    episodes := make([]Episode, 0, len(feed.Entries))
    for _, e := range feed.Entries {
        pub, _ := time.Parse(time.RFC3339, e.Published)
        episodes = append(episodes, Episode{
            ID:          e.VideoID,
            Title:       e.Title,
            URL:         e.Link.Href,
            Published:   pub,
            Description: e.Group.Description,
        })
    }
    return episodes, nil
}

// FetchPlaylist fetches the YouTube playlist RSS feed at url and returns episodes.
func FetchPlaylist(url string) ([]Episode, error) {
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Get(url) //nolint:gosec // URL is a package-level constant
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("youtube: HTTP %d fetching playlist", resp.StatusCode)
    }
    var buf []byte
    buf, err = io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    return ParseFeed(buf)
}
```

- [ ] **Step 5: Run tests — verify they pass**

```bash
go test ./internal/youtube/...
go vet ./internal/youtube/...
```
Expected: PASS, no vet warnings.

- [ ] **Step 6: Commit**

```bash
git add internal/youtube/
git commit -m "feat: add internal/youtube package for playlist RSS parsing"
```

---

## Task 2: Community RSS + HTML extraction package

**Files:**
- Create: `internal/community/community.go`
- Create: `internal/community/community_test.go`
- Create: `internal/community/testdata/posts.xml`
- Create: `internal/community/testdata/post.html`

- [ ] **Step 1: Create testdata fixtures**

Create `internal/community/testdata/posts.xml`:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>SAP Developer News</title>
    <item>
      <title>SAP Developer News - April 11 2026</title>
      <link>https://community.sap.com/t5/developer-news/apr-11/ba-p/999</link>
      <pubDate>Fri, 11 Apr 2026 10:00:00 +0000</pubDate>
    </item>
    <item>
      <title>SAP Developer News - March 21 2026</title>
      <link>https://community.sap.com/t5/developer-news/mar-21/ba-p/998</link>
      <pubDate>Fri, 21 Mar 2026 10:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>
```

Create `internal/community/testdata/post.html`:
```html
<!DOCTYPE html>
<html>
<head><title>SAP Developer News - April 11 2026</title></head>
<body>
<article>
<h1>SAP Developer News - April 11 2026</h1>
<p>Welcome to this week&#39;s SAP Developer News.</p>
<h2>CAP Updates</h2>
<p>Parallel batch processing is now available in CAP Node.js.</p>
<h2>BTP News</h2>
<p>New services are available in the SAP Discovery Center.</p>
</article>
</body>
</html>
```

- [ ] **Step 2: Write the failing tests**

Create `internal/community/community_test.go`:
```go
package community_test

import (
    "os"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/SAP-samples/sap-devs-cli/internal/community"
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
    assert.Empty(t, md)
}
```

- [ ] **Step 3: Run tests — verify they fail**

```bash
go test ./internal/community/...
```
Expected: FAIL — package does not exist yet.

- [ ] **Step 4: Implement `internal/community/community.go`**

```go
package community

import (
    "encoding/xml"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"

    htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// BlogPost is a single SAP Developer News Community blog post.
type BlogPost struct {
    Title     string
    URL       string
    Published time.Time
}

type rssFeed struct {
    XMLName xml.Name   `xml:"rss"`
    Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
    Items []rssItem `xml:"item"`
}

type rssItem struct {
    Title   string `xml:"title"`
    Link    string `xml:"link"`
    PubDate string `xml:"pubDate"`
}

// ParsePosts parses an RSS 2.0 feed and returns the blog posts in feed order.
func ParsePosts(data []byte) ([]BlogPost, error) {
    var feed rssFeed
    if err := xml.Unmarshal(data, &feed); err != nil {
        return nil, fmt.Errorf("community: parse posts: %w", err)
    }
    posts := make([]BlogPost, 0, len(feed.Channel.Items))
    for _, item := range feed.Channel.Items {
        pub, _ := time.Parse(time.RFC1123Z, item.PubDate)
        posts = append(posts, BlogPost{
            Title:     item.Title,
            URL:       item.Link,
            Published: pub,
        })
    }
    return posts, nil
}

// ExtractMarkdown converts an HTML page body to readable markdown text.
func ExtractMarkdown(data []byte) (string, error) {
    md, err := htmltomarkdown.ConvertString(string(data))
    if err != nil {
        return "", fmt.Errorf("community: extract markdown: %w", err)
    }
    return strings.TrimSpace(md), nil
}

// FetchBlogPosts fetches the SAP Community RSS feed and returns blog posts.
func FetchBlogPosts(rssURL string) ([]BlogPost, error) {
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Get(rssURL) //nolint:gosec // URL is a package-level constant
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("community: HTTP %d fetching RSS", resp.StatusCode)
    }
    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    return ParsePosts(data)
}

// FetchPostContent fetches a Community blog post URL and returns the body as markdown.
func FetchPostContent(postURL string) (string, error) {
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Get(postURL) //nolint:gosec // URL comes from RSS feed
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return "", fmt.Errorf("community: HTTP %d fetching post", resp.StatusCode)
    }
    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    return ExtractMarkdown(data)
}
```

- [ ] **Step 5: Run tests — verify they pass**

```bash
go test ./internal/community/...
go vet ./internal/community/...
```
Expected: PASS, no vet warnings.

- [ ] **Step 6: Commit**

```bash
git add internal/community/
git commit -m "feat: add internal/community package for SAP Community RSS and post extraction"
```

---

## Task 3: Correlator package

**Files:**
- Create: `internal/news/news.go`
- Create: `internal/news/news_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/news/news_test.go`:
```go
package news_test

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/SAP-samples/sap-devs-cli/internal/community"
    "github.com/SAP-samples/sap-devs-cli/internal/news"
    "github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

func date(year int, month time.Month, day int) time.Time {
    return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
}

func TestCorrelate_ExactDateMatch(t *testing.T) {
    episodes := []youtube.Episode{
        {ID: "ep1", Title: "News Apr 11", Published: date(2026, time.April, 11)},
    }
    posts := []community.BlogPost{
        {Title: "SAP Dev News Apr 11", URL: "https://community.sap.com/1", Published: date(2026, time.April, 11)},
    }
    items := news.Correlate(episodes, posts)
    require.Len(t, items, 1)
    require.NotNil(t, items[0].Community)
    assert.Equal(t, "https://community.sap.com/1", items[0].Community.URL)
}

func TestCorrelate_WithinSevenDayWindow(t *testing.T) {
    episodes := []youtube.Episode{
        {ID: "ep1", Title: "News Apr 11", Published: date(2026, time.April, 11)},
    }
    posts := []community.BlogPost{
        {Title: "SAP Dev News Apr 11", URL: "https://community.sap.com/1", Published: date(2026, time.April, 14)},
    }
    items := news.Correlate(episodes, posts)
    require.Len(t, items, 1)
    require.NotNil(t, items[0].Community)
}

func TestCorrelate_OutsideWindowIsNil(t *testing.T) {
    episodes := []youtube.Episode{
        {ID: "ep1", Title: "News Apr 11", Published: date(2026, time.April, 11)},
    }
    posts := []community.BlogPost{
        {Title: "Old post", URL: "https://community.sap.com/old", Published: date(2026, time.March, 1)},
    }
    items := news.Correlate(episodes, posts)
    require.Len(t, items, 1)
    assert.Nil(t, items[0].Community)
}

func TestCorrelate_NilPostsAllNil(t *testing.T) {
    episodes := []youtube.Episode{
        {ID: "ep1", Title: "News Apr 11", Published: date(2026, time.April, 11)},
        {ID: "ep2", Title: "News Apr 4", Published: date(2026, time.April, 4)},
    }
    items := news.Correlate(episodes, nil)
    require.Len(t, items, 2)
    assert.Nil(t, items[0].Community)
    assert.Nil(t, items[1].Community)
}

func TestCorrelate_TitleTiebreaker(t *testing.T) {
    // Two posts within ±7 days — the one with higher title similarity wins.
    episodes := []youtube.Episode{
        {ID: "ep1", Title: "SAP Developer News Apr 11", Published: date(2026, time.April, 11)},
    }
    posts := []community.BlogPost{
        {Title: "SAP Developer News Apr 11", URL: "https://community.sap.com/match", Published: date(2026, time.April, 13)},
        {Title: "Unrelated Post", URL: "https://community.sap.com/nomatch", Published: date(2026, time.April, 12)},
    }
    items := news.Correlate(episodes, posts)
    require.Len(t, items, 1)
    require.NotNil(t, items[0].Community)
    assert.Equal(t, "https://community.sap.com/match", items[0].Community.URL)
}

func TestCorrelate_EpisodeOrderPreserved(t *testing.T) {
    episodes := []youtube.Episode{
        {ID: "ep1", Published: date(2026, time.April, 11)},
        {ID: "ep2", Published: date(2026, time.April, 4)},
    }
    items := news.Correlate(episodes, nil)
    assert.Equal(t, "ep1", items[0].Episode.ID)
    assert.Equal(t, "ep2", items[1].Episode.ID)
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/news/...
```
Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement `internal/news/news.go`**

```go
package news

import (
    "math"
    "strings"
    "time"

    "github.com/SAP-samples/sap-devs-cli/internal/community"
    "github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

// NewsItem pairs a YouTube episode with its matched Community blog post (if any).
type NewsItem struct {
    Episode   youtube.Episode
    Community *community.BlogPost // nil if no match within ±7 days
}

const correlationWindow = 7 * 24 * time.Hour

// Correlate pairs each episode with the closest Community post within ±7 days.
// When multiple posts are within the window, the one with the highest title
// similarity (longest common substring length) wins.
// Episodes with no match have Community set to nil.
// Input episode order is preserved.
func Correlate(episodes []youtube.Episode, posts []community.BlogPost) []NewsItem {
    items := make([]NewsItem, len(episodes))
    for i, ep := range episodes {
        items[i] = NewsItem{Episode: ep, Community: bestMatch(ep, posts)}
    }
    return items
}

func bestMatch(ep youtube.Episode, posts []community.BlogPost) *community.BlogPost {
    var best *community.BlogPost
    bestScore := -1
    for j := range posts {
        diff := ep.Published.Sub(posts[j].Published)
        if math.Abs(diff.Hours()) > correlationWindow.Hours() {
            continue
        }
        score := lcs(strings.ToLower(ep.Title), strings.ToLower(posts[j].Title))
        if best == nil || score > bestScore {
            best = &posts[j]
            bestScore = score
        }
    }
    return best
}

// lcs returns the length of the longest common substring of a and b.
func lcs(a, b string) int {
    best := 0
    for i := range a {
        for j := range b {
            l := 0
            for i+l < len(a) && j+l < len(b) && a[i+l] == b[j+l] {
                l++
            }
            if l > best {
                best = l
            }
        }
    }
    return best
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/news/...
go vet ./internal/news/...
```
Expected: PASS, no vet warnings.

- [ ] **Step 5: Commit**

```bash
git add internal/news/
git commit -m "feat: add internal/news correlator package"
```

---

## Task 4: `cmd/news.go` command tree

**Files:**
- Create: `cmd/news.go`
- Modify: `cmd/root.go` — add `newsCmd` registration in `init()`

- [ ] **Step 1: Create `cmd/news.go` with the full command tree**

```go
package cmd

import (
    "fmt"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "text/tabwriter"

    "github.com/pkg/browser"
    "github.com/spf13/cobra"
    "github.com/SAP-samples/sap-devs-cli/internal/community"
    "github.com/SAP-samples/sap-devs-cli/internal/news"
    "github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

const (
    newsPlaylistRSS  = "https://www.youtube.com/feeds/videos.xml?playlist_id=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg"
    newsCommunityRSS = "https://community.sap.com/t5/developer-news/bg-p/developer-news/rss"
    newsLinkedIn     = "https://www.linkedin.com/newsletters/sap-developer-news-7155319074263044096/"
    newsYTMusic      = "" // fill in with YouTube Music podcast URL before shipping
)

var newsCmd = &cobra.Command{
    Use:   "news",
    Short: "Browse SAP Developer News episodes",
    RunE: func(cmd *cobra.Command, args []string) error {
        return newsListCmd.RunE(cmd, args)
    },
}

var newsListN int

var newsListCmd = &cobra.Command{
    Use:   "list",
    Short: "List recent SAP Developer News episodes",
    RunE: func(cmd *cobra.Command, args []string) error {
        episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
        if err != nil {
            return fmt.Errorf("could not fetch SAP Developer News: %w", err)
        }
        posts, _ := community.FetchBlogPosts(newsCommunityRSS) // failure is silent
        items := news.Correlate(episodes, posts)

        n := newsListN
        if n <= 0 || n > len(items) {
            n = len(items)
        }
        items = items[:n]

        w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
        fmt.Fprintln(w, "#\tDATE\tTITLE\tVIDEO\tCOMMUNITY")
        for i, item := range items {
            com := "--"
            if item.Community != nil {
                com = "[com]"
            }
            date := item.Episode.Published.Format("2006-01-02")
            fmt.Fprintf(w, "%d\t%s\t%s\t[yt]\t%s\n", i+1, date, item.Episode.Title, com)
        }
        w.Flush()
        fmt.Fprintln(cmd.OutOrStdout())
        fmt.Fprintf(cmd.OutOrStdout(), "LinkedIn newsletter: %s\n", newsLinkedIn)
        if newsYTMusic != "" {
            fmt.Fprintf(cmd.OutOrStdout(), "Listen on YouTube Music: %s\n", newsYTMusic)
        }
        return nil
    },
}

var newsLatestCmd = &cobra.Command{
    Use:   "latest",
    Short: "Open the most recent SAP Developer News episode in the browser",
    RunE: func(cmd *cobra.Command, args []string) error {
        episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
        if err != nil {
            return fmt.Errorf("could not fetch SAP Developer News: %w", err)
        }
        if len(episodes) == 0 {
            return fmt.Errorf("no episodes found")
        }
        if err := browser.OpenURL(episodes[0].URL); err != nil {
            fmt.Fprintln(cmd.OutOrStdout(), episodes[0].URL)
        }
        return nil
    },
}

var newsOpenCmd = &cobra.Command{
    Use:   "open <id>",
    Short: "Open a specific episode in the browser",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        id, err := strconv.Atoi(args[0])
        if err != nil || id < 1 {
            return fmt.Errorf("id must be a positive integer (run 'news list' to see IDs)")
        }
        episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
        if err != nil {
            return fmt.Errorf("could not fetch SAP Developer News: %w", err)
        }
        if id > len(episodes) {
            return fmt.Errorf("id %d out of range (only %d episodes available)", id, len(episodes))
        }
        ep := episodes[id-1]
        if err := browser.OpenURL(ep.URL); err != nil {
            fmt.Fprintln(cmd.OutOrStdout(), ep.URL)
        }
        return nil
    },
}

var newsSearchCmd = &cobra.Command{
    Use:   "search <query>",
    Short: "Search SAP Developer News episodes by title or description",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        q := strings.ToLower(args[0])
        episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
        if err != nil {
            return fmt.Errorf("could not fetch SAP Developer News: %w", err)
        }
        var matched []youtube.Episode
        for _, ep := range episodes {
            if strings.Contains(strings.ToLower(ep.Title), q) ||
                strings.Contains(strings.ToLower(ep.Description), q) {
                matched = append(matched, ep)
            }
        }
        if len(matched) == 0 {
            fmt.Fprintf(cmd.OutOrStdout(), "No episodes found matching %q\n", args[0])
            return nil
        }
        w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
        fmt.Fprintln(w, "DATE\tTITLE\tURL")
        for _, ep := range matched {
            fmt.Fprintf(w, "%s\t%s\t%s\n", ep.Published.Format("2006-01-02"), ep.Title, ep.URL)
        }
        w.Flush()
        return nil
    },
}

var newsReadPlain bool

var newsReadCmd = &cobra.Command{
    Use:   "read <id>",
    Short: "Read the SAP Community blog post for an episode",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        id, err := strconv.Atoi(args[0])
        if err != nil || id < 1 {
            return fmt.Errorf("id must be a positive integer (run 'news list' to see IDs)")
        }
        episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
        if err != nil {
            return fmt.Errorf("could not fetch SAP Developer News: %w", err)
        }
        if id > len(episodes) {
            return fmt.Errorf("id %d out of range (only %d episodes available)", id, len(episodes))
        }
        posts, err := community.FetchBlogPosts(newsCommunityRSS)
        if err != nil {
            return fmt.Errorf("could not fetch Community posts: %w", err)
        }
        items := news.Correlate(episodes, posts)
        item := items[id-1]
        if item.Community == nil {
            return fmt.Errorf("no SAP Community post found for episode %d", id)
        }
        content, err := community.FetchPostContent(item.Community.URL)
        if err != nil {
            return fmt.Errorf("could not fetch post content: %w", err)
        }
        if newsReadPlain {
            fmt.Fprintln(cmd.OutOrStdout(), content)
            return nil
        }
        return openPager(content)
    },
}

// openPager displays content via $PAGER, less (if available), or plain print.
func openPager(content string) error {
    pager := os.Getenv("PAGER")
    if pager == "" {
        if _, err := exec.LookPath("less"); err == nil {
            pager = "less"
        }
    }
    if pager == "" {
        fmt.Print(content)
        return nil
    }
    c := exec.Command(pager) //nolint:gosec // pager comes from env or LookPath
    c.Stdin = strings.NewReader(content)
    c.Stdout = os.Stdout
    c.Stderr = os.Stderr
    return c.Run()
}

func init() {
    newsListCmd.Flags().IntVarP(&newsListN, "count", "n", 10, "number of episodes to show")
    newsReadCmd.Flags().BoolVar(&newsReadPlain, "plain", false, "print plain text to stdout")
    newsCmd.AddCommand(newsListCmd, newsLatestCmd, newsOpenCmd, newsSearchCmd, newsReadCmd)
    rootCmd.AddCommand(newsCmd)
}
```

**Pattern note:** `cmd/resources.go:131` shows the convention — each command file registers itself with `rootCmd` in its own `init()`. Do not modify `cmd/root.go`.

- [ ] **Step 2: Build and vet**

```bash
go build ./...
go vet ./...
```
Expected: no errors.

- [ ] **Step 4: Smoke test locally**

```bash
go run . news list
go run . news search "CAP"
go run . news latest    # opens browser
```

Verify the table renders correctly with `#`, `DATE`, `TITLE`, `VIDEO`, `COMMUNITY` columns, and the footer shows LinkedIn and YouTube Music URLs.

- [ ] **Step 5: Set the YouTube Music podcast URL**

Replace the empty `newsYTMusic` constant in `cmd/news.go` with the real YouTube Music podcast show URL. The footer line is suppressed when the constant is `""`, so the build is clean either way. Do not ship with an empty string if the URL is known.

- [ ] **Step 6: Commit**

```bash
git add cmd/news.go
git commit -m "feat: add sap-devs news command with list, open, search, and read subcommands"
```

---

## Task 5: Final build verification

- [ ] **Step 1: Full build and vet**

```bash
go build ./...
go vet ./...
```
Expected: clean.

- [ ] **Step 2: Run all internal package tests**

```bash
go test ./internal/youtube/...
go test ./internal/community/...
go test ./internal/news/...
```
Expected: all PASS.

- [ ] **Step 3: Verify `sap-devs help news`**

```bash
go run . help news
```
Expected: shows `news`, `list`, `latest`, `open`, `search`, `read` with correct short descriptions.

- [ ] **Step 4: Final commit**

```bash
git add .
git commit -m "chore: final build verification for sap-devs news command"
```
