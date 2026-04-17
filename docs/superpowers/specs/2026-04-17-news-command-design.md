# `sap-devs news` Command — Design Spec

**Date:** 2026-04-17
**Status:** Approved
**Project:** sap-devs-cli

---

## Overview

Add a `sap-devs news` command that lists and reads SAP Developer News episodes from the terminal. Each episode is published across four formats: YouTube video, SAP Community blog post, LinkedIn newsletter issue, and YouTube Music podcast (show-level). The command fetches live from YouTube RSS and cross-references Community and LinkedIn by publish date. No static YAML; no sync integration — every invocation fetches fresh.

---

## Data Sources

| Source | Mechanism | Per-episode? |
|---|---|---|
| YouTube Developer News playlist | Public Atom RSS feed | Yes |
| SAP Community blog | RSS feed + HTML scrape for body | Yes |
| LinkedIn newsletter | HTML scrape of newsletter index page | Yes |
| YouTube Music podcast | Static constant (show-level URL) | No — shown as footer |

**YouTube playlist RSS:** `https://www.youtube.com/feeds/videos.xml?playlist_id=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg`

**SAP Community RSS:** `https://community.sap.com/t5/developer-news/bg-p/developer-news/rss`

**LinkedIn newsletter page:** `https://www.linkedin.com/newsletters/sap-developer-news-7155319074263044096/`

**YouTube Music podcast URL:** configured as a constant in `cmd/news.go` (user provides exact URL).

---

## Data Layer

### Packages

```
internal/youtube/     — fetch playlist RSS → []Episode
internal/community/   — fetch Community RSS + scrape post HTML → []BlogPost
internal/linkedin/    — scrape LinkedIn newsletter page → []NewsletterIssue
internal/news/        — correlate all three by publish date → []NewsItem
```

### Types

```go
// internal/youtube
type Episode struct {
    ID          string    // YouTube video ID
    Title       string
    URL         string    // https://youtube.com/watch?v=...
    Published   time.Time
    Description string
}

// internal/community
type BlogPost struct {
    Title     string
    URL       string
    Published time.Time
}

// internal/linkedin
type NewsletterIssue struct {
    Title     string
    URL       string
    Published time.Time
}

// internal/news
type NewsItem struct {
    Episode   youtube.Episode
    Community *community.BlogPost       // nil if no match found
    LinkedIn  *linkedin.NewsletterIssue // nil if no match found
}
```

### Correlation

`internal/news.Correlate(episodes, posts, issues)` joins sources by closest publish date within a **±7 day window**. When multiple candidates exist within the window, title similarity (longest common substring) is used as a tiebreaker. Unmatched episodes still appear in the list — `Community` and `LinkedIn` fields remain nil and render as `--` in the table.

### HTML Extraction

Both Community post body and LinkedIn issue body are extracted using `golang.org/x/net/html` tokenizer (existing transitive dependency). Tags are stripped, whitespace collapsed, paragraph breaks preserved as blank lines. This same logic applies to both sources via a shared `extractText(r io.Reader) string` helper in `internal/news`.

---

## Commands

```
sap-devs news                    → alias for news list
sap-devs news list [--n <count>] → table of recent episodes (default: 10)
sap-devs news latest             → open newest episode in browser
sap-devs news open <id>          → open episode <id> YouTube URL in browser
sap-devs news search <query>     → filter by title or description (case-insensitive)
sap-devs news read <id>          → fetch and render Community post in terminal
  [--linkedin]                   → use LinkedIn issue text instead of Community
  [--plain]                      → print raw text to stdout (pipe to AI/file)
```

**ID convention:** 1-based integer from the list; `1` = most recent episode. Consistent with how other commands use positional references.

### `news list` output format

```
#   DATE        TITLE                                VIDEO   COMMUNITY   LINKEDIN
1   2026-04-11  SAP Developer News Apr 11            [yt]    [com]       [li]
2   2026-04-04  SAP Developer News Apr 4             [yt]    [com]       --
3   2026-03-28  SAP Developer News Mar 28            [yt]    --          [li]

Listen on YouTube Music: https://music.youtube.com/...
```

Format markers are full URLs in `--plain` mode; short bracketed labels in interactive mode.

### `news read` output

Fetches HTML from Community post (default) or LinkedIn issue (`--linkedin`), extracts clean text, and prints to stdout. With `--plain`, output is raw text suitable for piping to an AI tool:

```bash
sap-devs news read 1 --plain | cat  # pipe to AI context
```

Without `--plain`, output is paginated via the system pager (`$PAGER`, falling back to `less`, then plain print if neither is available).

---

## Error Handling

| Failure | Behaviour |
|---|---|
| YouTube RSS unreachable | Exit with error — no episodes to show |
| Community RSS fails at list time | Episodes show `--` in COMMUNITY column; no error printed |
| LinkedIn scrape fails at list time | Episodes show `--` in LINKEDIN column; no error printed |
| `news read` fetch fails | Explicit error; suggest `--linkedin` as fallback |
| No correlation match (±7 days) | Silent; `Community`/`LinkedIn` fields nil, shown as `--` |

All HTTP calls use `net/http` with a **10-second timeout**. No retries. No rate limiting (user-invoked only).

---

## Testing

Tests live in `internal/` packages only. No HTTP calls in tests — all network logic is exercised via fixtures.

| Package | What is tested |
|---|---|
| `internal/youtube` | RSS Atom parsing with fixture XML |
| `internal/community` | RSS parsing + HTML text extraction with fixture files |
| `internal/linkedin` | HTML scrape with fixture newsletter page |
| `internal/news` | Correlation: exact match, ±7 day window, no match, multi-candidate tiebreaker |

`cmd/news.go` is thin cobra wiring — not unit tested directly, consistent with other commands in this project.

---

## Files to Create / Modify

| File | Action |
|---|---|
| `internal/youtube/youtube.go` | New — `FetchPlaylist(url string) ([]Episode, error)` |
| `internal/youtube/youtube_test.go` | New — fixture-based parsing tests |
| `internal/youtube/testdata/playlist.xml` | New — YouTube Atom RSS fixture |
| `internal/community/community.go` | New — `FetchBlogPosts(rssURL string)`, `FetchPostContent(url string)` |
| `internal/community/community_test.go` | New |
| `internal/community/testdata/posts.xml` | New — Community RSS fixture |
| `internal/community/testdata/post.html` | New — Community post HTML fixture |
| `internal/linkedin/linkedin.go` | New — `FetchNewsletterIssues(pageURL string)`, `FetchIssueContent(url string)` |
| `internal/linkedin/linkedin_test.go` | New |
| `internal/linkedin/testdata/newsletter.html` | New — LinkedIn page HTML fixture |
| `internal/news/news.go` | New — `Correlate(...)`, `extractText(...)` |
| `internal/news/news_test.go` | New — correlation tests |
| `cmd/news.go` | New — cobra command tree |
| `cmd/root.go` | Modify — register `newsCmd` |

---

## Out of Scope

- YouTube Data API v3 (future tier-2 enhancement)
- Static `news.yaml` fallback
- Sync/cache integration
- Per-episode podcast audio links (YouTube does not expose them)
- LinkedIn login / authenticated scraping
