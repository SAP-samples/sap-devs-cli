# `sap-devs news` Command — Design Spec

**Date:** 2026-04-17
**Status:** Approved
**Project:** sap-devs-cli

---

## Overview

Add a `sap-devs news` command that lists and reads SAP Developer News episodes from the terminal. Each episode is published across multiple formats: YouTube video, SAP Community blog post, LinkedIn newsletter (static index link), and YouTube Music podcast (show-level). The command fetches live from YouTube RSS and cross-references the SAP Community by publish date. LinkedIn is shown as a static newsletter URL only — per-episode scraping requires authentication and is deferred. No static YAML; no sync integration — every invocation fetches fresh.

---

## Data Sources

| Source | Mechanism | Per-episode? |
| --- | --- | --- |
| YouTube Developer News playlist | Public Atom RSS feed | Yes |
| SAP Community blog | RSS feed + HTML scrape for body | Yes |
| LinkedIn newsletter | Static index URL constant | No — shown as footer link |
| YouTube Music podcast | Static show URL constant | No — shown as footer link |

**Note:** LinkedIn's newsletter index page requires authentication for scraping. Per-episode LinkedIn correlation is deferred to a future pass. The static index URL is always shown so users can navigate there directly.

**YouTube playlist RSS:** `https://www.youtube.com/feeds/videos.xml?playlist_id=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg`

**SAP Community RSS:** `https://community.sap.com/t5/developer-news/bg-p/developer-news/rss`

**LinkedIn newsletter page:** `https://www.linkedin.com/newsletters/sap-developer-news-7155319074263044096/`

**YouTube Music podcast URL:** configured as a constant in `cmd/news.go` (user provides exact URL).

---

## Data Layer

### Packages

```text
internal/youtube/     — fetch playlist RSS → []Episode
internal/community/   — fetch Community RSS + scrape post HTML → []BlogPost
internal/news/        — correlate sources by publish date → []NewsItem
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

// internal/news
type NewsItem struct {
    Episode   youtube.Episode
    Community *community.BlogPost // nil if no match found
}
```

### Correlation

`internal/news.Correlate(episodes, posts)` joins YouTube episodes and Community posts by closest publish date within a **±7 day window**. When multiple candidates exist within the window, title similarity (longest common substring) is used as a tiebreaker. Unmatched episodes still appear in the list — `Community` remains nil and renders as `--` in the table.

**Note:** IDs are positional (1 = most recent) and are recomputed on every live fetch. They are not persistent — always run `news list` before `news open` or `news read` in the same session.

### HTML Extraction

Community post body is extracted inside `internal/community` using `golang.org/x/net/html` tokenizer (existing transitive dependency). `FetchPostContent(url string) (string, error)` returns the extracted plain text directly — tags stripped, whitespace collapsed, paragraph breaks preserved as blank lines. The extraction helper is private to `internal/community`.

---

## Commands

```text
sap-devs news                    → alias for news list
sap-devs news list [-n <count>]  → table of recent episodes (default: 10)
sap-devs news latest             → open newest episode in browser
sap-devs news open <id>          → open episode <id> YouTube URL in browser
sap-devs news search <query>     → filter by title or description (case-insensitive)
sap-devs news read <id>          → fetch and render Community post in terminal
  [--plain]                      → print raw text to stdout (pipe to AI/file)
```

**ID convention:** 1-based integer from the list; `1` = most recent episode. Consistent with how other commands use positional references.

### `news list` output format

```text
#   DATE        TITLE                                VIDEO   COMMUNITY
1   2026-04-11  SAP Developer News Apr 11            [yt]    [com]
2   2026-04-04  SAP Developer News Apr 4             [yt]    [com]
3   2026-03-28  SAP Developer News Mar 28            [yt]    --

LinkedIn newsletter: https://www.linkedin.com/newsletters/sap-developer-news-7155319074263044096/
Listen on YouTube Music: https://music.youtube.com/...
```

Format markers are full URLs in `--plain` mode; short bracketed labels in interactive mode.

### `news read` output

Fetches HTML from the Community post, extracts clean text via `internal/community.FetchPostContent`, and prints to stdout. With `--plain`, output is raw text suitable for piping to an AI tool:

```bash
sap-devs news read 1 --plain | cat  # pipe to AI context
```

Without `--plain`, output is paginated via the system pager. Pager resolution: `$PAGER` env var → `less` (probed silently via `exec.LookPath` — no error if absent) → plain print. On Windows, `less` is not present by default; plain print is the expected fallback and produces no error.

---

## Error Handling

| Failure | Behaviour |
| --- | --- |
| YouTube RSS unreachable | Exit with error — no episodes to show |
| Community RSS fails at list time | Episodes show `--` in COMMUNITY column; no error printed |
| `news read` fetch fails | Explicit error message |
| No correlation match (±7 days) | Silent; `Community` nil, shown as `--` |

All HTTP calls use `net/http` with a **10-second timeout**. No retries. No rate limiting (user-invoked only).

---

## Testing

Tests live in `internal/` packages only. No HTTP calls in tests — all network logic is exercised via fixtures.

| Package | What is tested |
| --- | --- |
| `internal/youtube` | RSS Atom parsing with fixture XML |
| `internal/community` | RSS parsing + HTML text extraction with fixture files |
| `internal/news` | Correlation: exact match, ±7 day window, no match, multi-candidate tiebreaker |

`cmd/news.go` is thin cobra wiring — not unit tested directly, consistent with other commands in this project.

---

## Files to Create / Modify

| File | Action |
| --- | --- |
| `internal/youtube/youtube.go` | New — `FetchPlaylist(url string) ([]Episode, error)` |
| `internal/youtube/youtube_test.go` | New — fixture-based parsing tests |
| `internal/youtube/testdata/playlist.xml` | New — YouTube Atom RSS fixture |
| `internal/community/community.go` | New — `FetchBlogPosts(rssURL string) ([]BlogPost, error)`, `FetchPostContent(url string) (string, error)` |
| `internal/community/community_test.go` | New |
| `internal/community/testdata/posts.xml` | New — Community RSS fixture |
| `internal/community/testdata/post.html` | New — Community post HTML fixture |
| `internal/news/news.go` | New — `Correlate(episodes []youtube.Episode, posts []community.BlogPost) []NewsItem` |
| `internal/news/news_test.go` | New — correlation tests |
| `cmd/news.go` | New — cobra command tree |
| `cmd/root.go` | Modify — register `newsCmd` |

---

## Out of Scope

- YouTube Data API v3 (future tier-2 enhancement)
- Static `news.yaml` fallback
- Sync/cache integration
- Per-episode podcast audio links (YouTube does not expose them)
- Per-episode LinkedIn correlation (requires authentication; future enhancement)
- `--linkedin` flag on `news read` (deferred with LinkedIn per-episode support)
