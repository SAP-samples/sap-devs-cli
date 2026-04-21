package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/community"
)

const newsDetailTTL = 1 * time.Hour

type newsDetailResult struct {
	Title        string              `json:"title"`
	CommunityURL string             `json:"community_url"`
	Items        []newsDetailItem    `json:"items"`
	Chapters     []newsDetailChapter `json:"chapters"`
	RawContent   string              `json:"raw_content,omitempty"`
}

type newsDetailItem struct {
	Title string   `json:"title"`
	Links []string `json:"links"`
}

type newsDetailChapter struct {
	Time  string `json:"time"`
	Title string `json:"title"`
}

func newsDetailCachePath(cacheDir, key string) string {
	return filepath.Join(cacheDir, "news-detail", key+".json")
}

func loadNewsDetailCache(cacheDir, key string, ttl time.Duration) (newsDetailResult, bool) {
	var zero newsDetailResult
	path := newsDetailCachePath(cacheDir, key)
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > ttl {
		return zero, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, false
	}
	var result newsDetailResult
	if err := json.Unmarshal(data, &result); err != nil {
		return zero, false
	}
	return result, true
}

func saveNewsDetailCache(cacheDir, key string, result newsDetailResult) {
	path := newsDetailCachePath(cacheDir, key)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}

func registerNewsDetailTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("get_news_detail",
			mcp.WithDescription("Get the full content of a specific SAP Developer News episode, including topics covered, chapter timestamps, and links. Use after get_recent_news to dive deeper into a specific episode."),
			mcp.WithString("community_url",
				mcp.Required(),
				mcp.Description("The community_url from a get_recent_news result"),
			),
		),
		getNewsDetailHandler(deps),
	)
}

func getNewsDetailHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url, err := req.RequireString("community_url")
		if err != nil {
			return mcp.NewToolResultError("community_url parameter is required"), nil
		}

		cacheKey := fmt.Sprintf("%x", sha256.Sum256([]byte(url)))
		if deps.CacheDir != "" {
			if cached, ok := loadNewsDetailCache(deps.CacheDir, cacheKey, newsDetailTTL); ok {
				b, _ := json.Marshal(cached)
				return mcp.NewToolResultText(string(b)), nil
			}
		}

		markdown, err := community.FetchPostContent(url)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch episode content: %v", err)), nil
		}

		result := parseNewsDetail(url, markdown)

		if deps.CacheDir != "" {
			saveNewsDetailCache(deps.CacheDir, cacheKey, result)
		}

		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	}
}

var (
	boldHeadingRe = regexp.MustCompile(`(?m)^\*\*(.+?)\*\*\s*$`)
	linkRe        = regexp.MustCompile(`\[([^\]]*)\]\((https?://[^\s)]+)\)`)
	chapterRe     = regexp.MustCompile(`(?m)^(\d{2}:\d{2})\s+(.+)$`)
)

func parseNewsDetail(communityURL, markdown string) newsDetailResult {
	result := newsDetailResult{
		CommunityURL: communityURL,
	}

	lines := strings.Split(markdown, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
			candidate := strings.TrimLeft(trimmed, "# ")
			if strings.Contains(strings.ToLower(candidate), "developer news") {
				result.Title = candidate
				break
			}
		}
	}

	itemsSection := extractSection(markdown, "ITEMS")
	if itemsSection != "" {
		parts := boldHeadingRe.Split(itemsSection, -1)
		headings := boldHeadingRe.FindAllStringSubmatch(itemsSection, -1)
		for i, heading := range headings {
			item := newsDetailItem{Title: strings.TrimSpace(heading[1])}
			if i+1 < len(parts) {
				for _, m := range linkRe.FindAllStringSubmatch(parts[i+1], -1) {
					item.Links = append(item.Links, m[2])
				}
			}
			result.Items = append(result.Items, item)
		}
	}

	chaptersSection := extractSection(markdown, "CHAPTER TITLES")
	if chaptersSection != "" {
		for _, m := range chapterRe.FindAllStringSubmatch(chaptersSection, -1) {
			result.Chapters = append(result.Chapters, newsDetailChapter{
				Time:  m[1],
				Title: strings.TrimSpace(m[2]),
			})
		}
	}

	if len(result.Items) == 0 {
		result.RawContent = markdown
	}

	return result
}

func extractSection(markdown, sectionName string) string {
	marker := strings.ToUpper(sectionName)
	idx := strings.Index(strings.ToUpper(markdown), marker)
	if idx == -1 {
		return ""
	}
	rest := markdown[idx+len(marker):]
	nextSection := -1
	for _, sep := range []string{"### ", "## ", "CHAPTER TITLES", "TRANSCRIPT", "ITEMS"} {
		if sep == marker {
			continue
		}
		pos := strings.Index(strings.ToUpper(rest), strings.ToUpper(sep))
		if pos != -1 && (nextSection == -1 || pos < nextSection) {
			nextSection = pos
		}
	}
	if nextSection != -1 {
		rest = rest[:nextSection]
	}
	return strings.TrimSpace(rest)
}
