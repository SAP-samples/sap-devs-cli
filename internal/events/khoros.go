package events

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

const khorosBaseURL = "https://groups.community.sap.com/api/2.0/search"

type khorosResponse struct {
	Status string     `json:"status"`
	Data   khorosData `json:"data"`
}

type khorosData struct {
	Items []khorosItem `json:"items"`
}

type khorosItem struct {
	ID           string              `json:"id"`
	Subject      string              `json:"subject"`
	ViewHref     string              `json:"view_href"`
	OccasionData *khorosOccasionData `json:"occasion_data"`
}

type khorosOccasionData struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Timezone  string `json:"timezone"`
	Location  string `json:"location"`
}

func FetchKhoros(boardID, typeID, defaultScope string, timeout time.Duration) ([]content.EventInstance, error) {
	query := fmt.Sprintf(
		"SELECT id,subject,view_href,occasion_data.location,occasion_data.start_time,occasion_data.end_time,occasion_data.timezone FROM messages WHERE board.id='%s'",
		boardID,
	)
	reqURL := khorosBaseURL + "?q=" + url.QueryEscape(query)

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
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
		return nil, fmt.Errorf("events: HTTP %d fetching Khoros API", resp.StatusCode)
	}

	const maxBodyBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}

	return ParseKhoros(data, typeID, defaultScope)
}

func ParseKhoros(data []byte, typeID, defaultScope string) ([]content.EventInstance, error) {
	var resp khorosResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("events: parse Khoros: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("events: Khoros API status: %s", resp.Status)
	}

	now := time.Now()
	var out []content.EventInstance
	for _, item := range resp.Data.Items {
		if item.OccasionData == nil {
			continue
		}
		if strings.Contains(item.ViewHref, "/ec-p/") {
			continue
		}

		startTime, err := time.Parse(time.RFC3339, normalizeTimestamp(item.OccasionData.StartTime))
		if err != nil {
			continue
		}
		if startTime.Before(now) {
			continue
		}

		var endDateStr string
		if item.OccasionData.EndTime != "" {
			if et, err := time.Parse(time.RFC3339, normalizeTimestamp(item.OccasionData.EndTime)); err == nil {
				endDateStr = et.Format("2006-01-02")
			}
		}

		out = append(out, content.EventInstance{
			ID:         fmt.Sprintf("%s/%s", typeID, item.ID),
			Type:       typeID,
			Title:      item.Subject,
			DateStr:    startTime.Format("2006-01-02"),
			EndDateStr: endDateStr,
			Location:   item.OccasionData.Location,
			Scope:      defaultScope,
			URL:        item.ViewHref,
		})
	}
	return out, nil
}

// normalizeTimestamp converts Khoros timestamps like "2026-05-21T09:30:00.000+02:00" to RFC3339.
func normalizeTimestamp(ts string) string {
	if i := strings.Index(ts, "."); i != -1 {
		rest := ts[i+1:]
		if plus := strings.IndexAny(rest, "+-Z"); plus != -1 {
			return ts[:i] + rest[plus:]
		}
	}
	return ts
}
