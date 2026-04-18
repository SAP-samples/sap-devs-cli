package tutorials

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const solrSearchURL = "https://developers.sap.com/bin/sapdx/v3/solr/search"

// Enrich attempts to augment the index with data from developers.sap.com.
// Returns the original index unchanged if the API is unavailable (403, timeout, etc.).
func Enrich(index []TutorialMeta, userAgent string) []TutorialMeta {
	return EnrichWithURL(index, userAgent, solrSearchURL)
}

// EnrichWithURL is the testable variant of Enrich with a custom base URL.
func EnrichWithURL(index []TutorialMeta, userAgent, baseURL string) []TutorialMeta {
	payload := fmt.Sprintf(`{"rows":"2000","start":0,"searchField":"","pagePath":"/content/developers/website/languages/en/tutorial-navigator","language":"en_us","addDefaultLanguage":true,"filters":[]}`)
	url := fmt.Sprintf("%s?json=%s", baseURL, payload)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return index
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return index
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return index
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return index
	}

	var result struct {
		Result []struct {
			PublicURL string `json:"publicUrl"`
			Featured  bool   `json:"featured"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return index
	}

	featuredSlugs := make(map[string]bool)
	for _, r := range result.Result {
		slug := extractSlugFromURL(r.PublicURL)
		if slug != "" && r.Featured {
			featuredSlugs[slug] = true
		}
	}

	// Currently we only extract featured flags.
	// Mission/group membership can be added later.
	_ = featuredSlugs

	return index
}

func extractSlugFromURL(publicURL string) string {
	if len(publicURL) < 12 {
		return ""
	}
	s := publicURL
	if s[0] == '/' {
		s = s[1:]
	}
	if len(s) > 10 && s[:10] == "tutorials/" {
		slug := s[10:]
		if len(slug) > 5 && slug[len(slug)-5:] == ".html" {
			return slug[:len(slug)-5]
		}
	}
	return ""
}
