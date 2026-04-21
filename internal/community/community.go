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
		pub, _ := time.Parse(time.RFC1123, item.PubDate)
		if pub.IsZero() {
			pub, _ = time.Parse(time.RFC1123Z, item.PubDate)
		}
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

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

// FetchBlogPosts fetches the SAP Community RSS feed and returns blog posts.
func FetchBlogPosts(rssURL string) ([]BlogPost, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rssURL, nil) //nolint:gosec
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("community: HTTP %d fetching RSS", resp.StatusCode)
	}
	const maxBodyBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	return ParsePosts(data)
}

// FetchPostContent fetches a Community blog post URL and returns the body as markdown.
func FetchPostContent(postURL string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, postURL, nil) //nolint:gosec
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("community: HTTP %d fetching post", resp.StatusCode)
	}
	const maxBodyBytes = 4 << 20 // 4 MiB for full HTML pages
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", err
	}
	return ExtractMarkdown(data)
}
