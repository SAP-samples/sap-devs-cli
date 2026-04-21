package mcpserver

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/community"
	"github.com/SAP-samples/sap-devs-cli/internal/credentials"
	"github.com/SAP-samples/sap-devs-cli/internal/news"
	"github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

const (
	newsPlaylistRSS  = "https://www.youtube.com/feeds/videos.xml?playlist_id=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg"
	newsPlaylistID   = "PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg"
	newsCommunityRSS = "https://community.sap.com/khhcw49343/rss/board?board.id=developer-news"
	newsTTL          = 10 * time.Minute
)

type newsFetcher struct {
	mu        sync.Mutex
	cached    []news.NewsItem
	fetchedAt time.Time
	cacheDir  string
	configDir string
}

func (f *newsFetcher) get() []news.NewsItem {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.cached) > 0 && time.Since(f.fetchedAt) < newsTTL {
		return f.cached
	}

	if f.cacheDir != "" {
		if items, ok := news.LoadCache(f.cacheDir, newsTTL); ok {
			f.cached = items
			f.fetchedAt = time.Now()
			return f.cached
		}
	}

	episodes, _ := youtube.FetchPlaylistRetry(newsPlaylistRSS, 3)
	if episodes == nil && f.configDir != "" {
		apiKey := credentials.ResolveService(f.configDir, "youtube", []string{"YOUTUBE_API_KEY"})
		if apiKey != "" {
			episodes, _ = youtube.FetchPlaylistAPI(newsPlaylistID, apiKey)
		}
	}

	if episodes != nil {
		posts, _ := community.FetchBlogPosts(newsCommunityRSS)
		f.cached = news.Correlate(episodes, posts)
		f.fetchedAt = time.Now()
		if f.cacheDir != "" {
			_ = news.SaveCache(f.cacheDir, f.cached)
		}
		return f.cached
	}

	if len(f.cached) > 0 {
		return f.cached
	}

	if f.cacheDir != "" {
		if stale, ok := news.LoadCacheStale(f.cacheDir); ok {
			f.cached = stale
			f.fetchedAt = time.Now()
			return f.cached
		}
		officialCache := filepath.Join(f.cacheDir, "official")
		if baseline, ok := news.LoadBaseline(officialCache); ok {
			f.cached = baseline
			f.fetchedAt = time.Now()
			return f.cached
		}
	}

	return nil
}

func registerNewsTools(s *server.MCPServer, deps Deps) {
	fetcher := &newsFetcher{
		cacheDir:  deps.CacheDir,
		configDir: deps.ConfigDir,
	}

	s.AddTool(
		mcp.NewTool("get_recent_news",
			mcp.WithDescription("Get the latest SAP Developer News episodes (weekly show on SAP Developers YouTube). Returns episode titles, YouTube URLs, and companion SAP Community blog post URLs. Use when asked about what's new in SAP."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of episodes to return (default 5, max 50)"),
			),
		),
		getRecentNewsHandler(fetcher),
	)
}

type newsResult struct {
	Title        string `json:"title"`
	URL          string `json:"url"`
	Published    string `json:"published"`
	CommunityURL string `json:"community_url"`
}

func getRecentNewsHandler(fetcher *newsFetcher) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := clampLimit(req.GetInt("limit", 5), 5, 50)

		items := fetcher.get()
		total := len(items)
		if total == 0 {
			return wrapResults([]newsResult{}, 0, 0, "news episodes", ""), nil
		}
		if limit > total {
			limit = total
		}

		out := make([]newsResult, 0, limit)
		for _, item := range items[:limit] {
			nr := newsResult{
				Title:     item.Episode.Title,
				URL:       item.Episode.URL,
				Published: item.Episode.Published.Format(time.RFC3339),
			}
			if item.Community != nil {
				nr.CommunityURL = item.Community.URL
			}
			out = append(out, nr)
		}
		return wrapResults(out, total, len(out), "news episodes", ""), nil
	}
}
