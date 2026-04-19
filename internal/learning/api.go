package learning

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// SearchAPI calls the learning.sap.com search endpoint and returns results.
func SearchAPI(query string, limit int) ([]LearningJourney, error) {
	if limit <= 0 {
		limit = 15
	}
	filters := fmt.Sprintf(`{"locale":"en-US","query":"%s"}`, query)
	types := `["learning-journey"]`

	u := fmt.Sprintf("%s(types='%s',filters='%s',sort='',limit=%d,page=1)",
		SearchURL,
		url.PathEscape(types),
		url.PathEscape(filters),
		limit,
	)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("search API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search API: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read search response: %w", err)
	}

	var sr searchResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	var results []LearningJourney
	for _, r := range sr.Value.Results {
		results = append(results, LearningJourney{
			Title:         r.Title,
			Slug:          r.Slug,
			Description:   r.Description,
			Level:         r.ExperienceLevel,
			DurationHours: strconv.FormatFloat(r.Duration, 'f', 2, 64),
			URL:           BaseURL + r.Slug,
		})
	}
	return results, nil
}
