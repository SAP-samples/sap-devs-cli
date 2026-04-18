package events

import (
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

const userAgent = "Mozilla/5.0 (compatible; sap-devs/1.0)"

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

// FetchRSS fetches an RSS feed and returns parsed EventInstances.
func FetchRSS(rssURL, typeID, defaultScope string, timeout time.Duration) ([]content.EventInstance, error) {
	client := &http.Client{Timeout: timeout}
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
		return nil, fmt.Errorf("events: HTTP %d fetching RSS", resp.StatusCode)
	}
	const maxBodyBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	return ParseRSS(data, typeID, defaultScope)
}

// ParseRSS parses RSS XML into EventInstances.
func ParseRSS(data []byte, typeID, defaultScope string) ([]content.EventInstance, error) {
	var feed rssFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("events: parse RSS: %w", err)
	}
	var out []content.EventInstance
	for _, item := range feed.Channel.Items {
		hash := sha256.Sum256([]byte(item.Link))
		id := fmt.Sprintf("%s/%x", typeID, hash[:6])
		dateStr := ""
		if pub, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil {
			dateStr = pub.Format("2006-01-02")
		}
		out = append(out, content.EventInstance{
			ID:      id,
			Type:    typeID,
			Title:   item.Title,
			DateStr: dateStr,
			Scope:   defaultScope,
			URL:     item.Link,
		})
	}
	return out, nil
}
