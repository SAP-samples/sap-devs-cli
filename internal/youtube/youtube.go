package youtube

import (
	"encoding/xml"
	"errors"
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
	Duration    string
	Tags        []string
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
		pub, err := time.Parse(time.RFC3339, e.Published)
		if err != nil {
			return nil, fmt.Errorf("youtube: parse published %q: %w", e.Published, err)
		}
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

// HTTPError records an HTTP status code from a failed YouTube request.
type HTTPError struct {
	StatusCode int
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("youtube: HTTP %d fetching %s", e.StatusCode, e.URL)
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
		return nil, &HTTPError{StatusCode: resp.StatusCode, URL: url}
	}
	const maxBodyBytes = 1 << 20 // 1 MiB
	buf, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	return ParseFeed(buf)
}

func isRetryableHTTP(err error) bool {
	var he *HTTPError
	if errors.As(err, &he) {
		return he.StatusCode == 404 || he.StatusCode >= 500
	}
	return false
}

// FetchPlaylistRetry wraps FetchPlaylist with exponential backoff.
// Only retries on HTTP 404/5xx (YouTube's intermittent outage codes).
func FetchPlaylistRetry(url string, maxAttempts int) ([]Episode, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var lastErr error
	backoff := 2 * time.Second
	for i := 0; i < maxAttempts; i++ {
		eps, err := FetchPlaylist(url)
		if err == nil {
			return eps, nil
		}
		lastErr = err
		if !isRetryableHTTP(err) || i == maxAttempts-1 {
			break
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	return nil, lastErr
}
