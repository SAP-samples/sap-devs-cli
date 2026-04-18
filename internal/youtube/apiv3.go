package youtube

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ---- private JSON response types ----

type playlistItemsResponse struct {
	Items         []playlistItem `json:"items"`
	NextPageToken string         `json:"nextPageToken"`
}

type playlistItem struct {
	Snippet playlistItemSnippet `json:"snippet"`
}

type playlistItemSnippet struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	PublishedAt string     `json:"publishedAt"`
	ResourceID  resourceID `json:"resourceId"`
}

type resourceID struct {
	VideoID string `json:"videoId"`
}

type videosResponse struct {
	Items []videoItem `json:"items"`
}

type videoItem struct {
	ID             string         `json:"id"`
	ContentDetails contentDetails `json:"contentDetails"`
	Snippet        videoSnippet   `json:"snippet"`
}

type contentDetails struct {
	Duration string `json:"duration"`
}

type videoSnippet struct {
	Tags []string `json:"tags"`
}

// ---- Public types ----

// PlaylistItemParsed holds the fields extracted from a playlistItems.list response item.
type PlaylistItemParsed struct {
	VideoID     string
	Title       string
	Description string
	Published   time.Time
}

// VideoDetails holds the enrichment fields returned by a videos.list response item.
type VideoDetails struct {
	Duration string
	Tags     []string
}

// ---- Public parse functions ----

// ParsePlaylistItemsResponse parses a YouTube playlistItems.list JSON response.
// It returns the parsed items and the nextPageToken (empty string when no more pages).
func ParsePlaylistItemsResponse(data []byte) ([]PlaylistItemParsed, string, error) {
	var resp playlistItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, "", fmt.Errorf("youtube: parse playlistItems response: %w", err)
	}
	items := make([]PlaylistItemParsed, 0, len(resp.Items))
	for _, item := range resp.Items {
		pub, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
		if err != nil {
			return nil, "", fmt.Errorf("youtube: parse publishedAt %q: %w", item.Snippet.PublishedAt, err)
		}
		items = append(items, PlaylistItemParsed{
			VideoID:     item.Snippet.ResourceID.VideoID,
			Title:       item.Snippet.Title,
			Description: item.Snippet.Description,
			Published:   pub,
		})
	}
	return items, resp.NextPageToken, nil
}

// ParseVideosResponse parses a YouTube videos.list JSON response.
// It returns a map keyed by video ID containing duration and tag details.
func ParseVideosResponse(data []byte) (map[string]VideoDetails, error) {
	var resp videosResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("youtube: parse videos response: %w", err)
	}
	details := make(map[string]VideoDetails, len(resp.Items))
	for _, item := range resp.Items {
		tags := item.Snippet.Tags
		if tags == nil {
			tags = []string{}
		}
		details[item.ID] = VideoDetails{
			Duration: item.ContentDetails.Duration,
			Tags:     tags,
		}
	}
	return details, nil
}

// ---- Private helpers ----

const (
	youtubePlaylistItemsBase = "https://www.googleapis.com/youtube/v3/playlistItems"
	youtubeVideosBase        = "https://www.googleapis.com/youtube/v3/videos"
	maxPageSize              = 50
	maxBodyBytes             = 5 << 20 // 5 MiB
)

func buildPlaylistItemsURL(playlistID, apiKey, pageToken string) string {
	q := url.Values{}
	q.Set("part", "snippet")
	q.Set("maxResults", fmt.Sprintf("%d", maxPageSize))
	q.Set("playlistId", playlistID)
	q.Set("key", apiKey)
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	return youtubePlaylistItemsBase + "?" + q.Encode()
}

func buildVideosURL(videoIDs []string, apiKey string) string {
	q := url.Values{}
	q.Set("part", "contentDetails,snippet")
	q.Set("id", strings.Join(videoIDs, ","))
	q.Set("key", apiKey)
	return youtubeVideosBase + "?" + q.Encode()
}

func fetchJSON(client *http.Client, rawURL string) ([]byte, error) {
	resp, err := client.Get(rawURL) //nolint:gosec // URL is constructed internally
	if err != nil {
		return nil, fmt.Errorf("youtube: GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("youtube: HTTP %d from %s", resp.StatusCode, rawURL)
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("youtube: read body from %s: %w", rawURL, err)
	}
	return buf, nil
}

// FetchPlaylistAPI fetches all episodes in a YouTube playlist using the Data API v3.
// It paginates through playlistItems.list and batch-fetches video details via videos.list.
// Video detail fetch failures are non-fatal — episodes are returned without enrichment.
func FetchPlaylistAPI(playlistID, apiKey string) ([]Episode, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	// Collect all playlist items across pages.
	var allItems []PlaylistItemParsed
	pageToken := ""
	for {
		reqURL := buildPlaylistItemsURL(playlistID, apiKey, pageToken)
		data, err := fetchJSON(client, reqURL)
		if err != nil {
			return nil, err
		}
		items, next, err := ParsePlaylistItemsResponse(data)
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, items...)
		if next == "" {
			break
		}
		pageToken = next
	}

	// Gather all video IDs and batch-fetch details in groups of 50.
	videoIDs := make([]string, 0, len(allItems))
	for _, item := range allItems {
		videoIDs = append(videoIDs, item.VideoID)
	}
	detailsMap := make(map[string]VideoDetails)
	for i := 0; i < len(videoIDs); i += maxPageSize {
		end := i + maxPageSize
		if end > len(videoIDs) {
			end = len(videoIDs)
		}
		batch := videoIDs[i:end]
		reqURL := buildVideosURL(batch, apiKey)
		data, err := fetchJSON(client, reqURL)
		if err != nil {
			// Non-fatal: proceed without enrichment for this batch.
			continue
		}
		batchDetails, err := ParseVideosResponse(data)
		if err != nil {
			// Non-fatal: proceed without enrichment for this batch.
			continue
		}
		for id, d := range batchDetails {
			detailsMap[id] = d
		}
	}

	// Combine items with details into episodes.
	episodes := make([]Episode, 0, len(allItems))
	for _, item := range allItems {
		ep := Episode{
			ID:          item.VideoID,
			Title:       item.Title,
			URL:         "https://www.youtube.com/watch?v=" + item.VideoID,
			Published:   item.Published,
			Description: item.Description,
		}
		if d, ok := detailsMap[item.VideoID]; ok {
			ep.Duration = d.Duration
			ep.Tags = d.Tags
		}
		episodes = append(episodes, ep)
	}
	return episodes, nil
}
