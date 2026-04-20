package mcpserver

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/community"
	"github.com/SAP-samples/sap-devs-cli/internal/news"
	"github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

const (
	newsPlaylistRSS  = "https://www.youtube.com/feeds/videos.xml?playlist_id=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg"
	newsCommunityRSS = "https://community.sap.com/t5/developer-news/bg-p/developer-news/rss"
	newsFetchTimeout = 5 * time.Second
)

type newsFetcher struct {
	mu     sync.Mutex
	once   sync.Once
	cached []news.NewsItem
}

func (f *newsFetcher) get() []news.NewsItem {
	f.once.Do(func() {
		done := make(chan struct{})
		go func() {
			episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
			if err != nil {
				close(done)
				return
			}
			posts, _ := community.FetchBlogPosts(newsCommunityRSS)
			f.mu.Lock()
			f.cached = news.Correlate(episodes, posts)
			f.mu.Unlock()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(newsFetchTimeout):
		}
	})
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cached
}

func registerNewsTools(s *server.MCPServer) {
	fetcher := &newsFetcher{}

	s.AddTool(
		mcp.NewTool("get_recent_news",
			mcp.WithDescription("Get the latest SAP Developer News episodes from YouTube and SAP Community"),
			mcp.WithNumber("count",
				mcp.Description("Number of episodes to return (default 5)"),
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
		count := req.GetInt("count", 5)
		if count <= 0 {
			count = 5
		}

		items := fetcher.get()
		if len(items) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		if count > len(items) {
			count = len(items)
		}

		out := make([]newsResult, 0, count)
		for _, item := range items[:count] {
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
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}
