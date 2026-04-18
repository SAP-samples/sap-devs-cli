// internal/discovery/client.go
package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL            = "https://discovery-center.cloud.sap"
	platformxPath      = "/platformx/"
	servicecatalogPath = "/servicecatalog/"
)

// Client talks to the Discovery Center OData V2 services.
type Client struct {
	baseURL   string
	http      *http.Client
	csrfToken string
}

// NewClient returns a Client ready to call Discovery Center APIs.
func NewClient() *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// FetchMissions calls GetMissionCatalogContentV2 and returns grouped missions.
func (c *Client) FetchMissions() ([]MissionCatalogGroup, error) {
	raw, err := c.batchGET("GetMissionCatalogContentV2?username=''")
	if err != nil {
		return nil, fmt.Errorf("fetch missions: %w", err)
	}
	var groups []MissionCatalogGroup
	if err := json.Unmarshal(raw, &groups); err != nil {
		return nil, fmt.Errorf("parse missions: %w", err)
	}
	return groups, nil
}

// SearchMissions calls GetViewFuzzySearchesCustomV3 with the given query and filters.
func (c *Client) SearchMissions(query string, f SearchFilters) ([]Mission, error) {
	top := f.Top
	if top <= 0 {
		top = 20
	}
	q := fmt.Sprintf(
		"GetViewFuzzySearchesCustomV3?searchString='%s'&filterCategory='%s'&filterType=mission-catalog-search&filterProduct='%s'&filterLob='%s'&filterIndustry='%s'&filterFocusTags='%s'&filterPartners='%s'&filterQuickFilter=''&top='%d'",
		query, f.Category, f.Product, f.LoB, f.Industry, f.FocusTags, f.Partners, top)
	raw, err := c.batchGET(q)
	if err != nil {
		return nil, fmt.Errorf("search missions: %w", err)
	}
	var missions []Mission
	if err := json.Unmarshal(raw, &missions); err != nil {
		return nil, fmt.Errorf("parse search results: %w", err)
	}
	return missions, nil
}

// FetchCategories calls GetProductsCategories.
func (c *Client) FetchCategories() (*Categories, error) {
	raw, err := c.batchGET("GetProductsCategories?version='1'")
	if err != nil {
		return nil, fmt.Errorf("fetch categories: %w", err)
	}
	var cats Categories
	if err := json.Unmarshal(raw, &cats); err != nil {
		return nil, fmt.Errorf("parse categories: %w", err)
	}
	return &cats, nil
}

// FetchFacets calls GetApplicationFocusTagsIndustryLob.
func (c *Client) FetchFacets() (*Facets, error) {
	raw, err := c.batchGET("GetApplicationFocusTagsIndustryLob?version='1'")
	if err != nil {
		return nil, fmt.Errorf("fetch facets: %w", err)
	}
	var facets Facets
	if err := json.Unmarshal(raw, &facets); err != nil {
		return nil, fmt.Errorf("parse facets: %w", err)
	}
	return &facets, nil
}

// FetchGuidanceTree calls GetGuidanceFrameworkTree.
func (c *Client) FetchGuidanceTree() ([]GuidanceNode, error) {
	raw, err := c.batchGET("GetGuidanceFrameworkTree")
	if err != nil {
		return nil, fmt.Errorf("fetch guidance tree: %w", err)
	}
	var nodes []GuidanceNode
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, fmt.Errorf("parse guidance tree: %w", err)
	}
	return nodes, nil
}

// FetchGuidanceContent calls GetGuidanceFrameworkContentById for a single node.
func (c *Client) FetchGuidanceContent(id string) (string, error) {
	raw, err := c.batchGET(fmt.Sprintf("GetGuidanceFrameworkContentById?id='%s'", id))
	if err != nil {
		return "", fmt.Errorf("fetch guidance content: %w", err)
	}
	// Raw is a JSON string (the content is returned as a plain string, not an array/object).
	var content string
	if err := json.Unmarshal(raw, &content); err != nil {
		// If unmarshal as string fails, return raw bytes as string.
		return string(raw), nil
	}
	return content, nil
}

// FetchServices calls GET /servicecatalog/ServiceDetailss (no batch needed).
func (c *Client) FetchServices() ([]Service, error) {
	url := c.baseURL + servicecatalogPath +
		"ServiceDetailss?$format=json&$select=Id,Name,ShortName,Category,ShortDescription,LicenseModelType,IsDeprecatedService"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("DataServiceVersion", "2.0")
	req.Header.Set("MaxDataServiceVersion", "2.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch services: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch services: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		D struct {
			Results []Service `json:"results"`
		} `json:"d"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("parse services: %w", err)
	}
	return wrapper.D.Results, nil
}

// fetchCSRF obtains a CSRF token from the /platformx/ endpoint.
func (c *Client) fetchCSRF() error {
	req, err := http.NewRequest("HEAD", c.baseURL+platformxPath, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-csrf-token", "Fetch")
	req.Header.Set("x-requested-with", "XMLHttpRequest")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("CSRF fetch: %w", err)
	}
	resp.Body.Close()
	token := resp.Header.Get("x-csrf-token")
	if token == "" {
		return fmt.Errorf("CSRF fetch: no token in response")
	}
	c.csrfToken = token
	return nil
}

// batchGET sends a single GET inside an OData $batch request to /platformx/$batch
// and returns the unwrapped inner JSON value.
//
// The /platformx/ endpoint returns function import results as JSON strings inside
// the OData wrapper: {"d":{"FunctionName":"[{...}]"}}. This method handles the
// double-unmarshal: outer OData envelope → extract string → return raw JSON bytes.
func (c *Client) batchGET(query string) ([]byte, error) {
	if c.csrfToken == "" {
		if err := c.fetchCSRF(); err != nil {
			return nil, err
		}
	}

	boundary := fmt.Sprintf("batch_%d", time.Now().UnixNano())
	lines := []string{
		"--" + boundary,
		"Content-Type: application/http",
		"Content-Transfer-Encoding: binary",
		"",
		"GET " + query + " HTTP/1.1",
		"sap-cancel-on-close: false",
		"sap-contextid-accept: header",
		"Accept: application/json",
		"Accept-Language: en",
		"DataServiceVersion: 2.0",
		"MaxDataServiceVersion: 2.0",
		"X-Requested-With: XMLHttpRequest",
		"x-csrf-token: " + c.csrfToken,
		"",
		"",
		"--" + boundary + "--",
		"",
	}
	body := strings.Join(lines, "\r\n")

	req, err := http.NewRequest("POST", c.baseURL+platformxPath+"$batch", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "multipart/mixed;boundary="+boundary)
	req.Header.Set("x-csrf-token", c.csrfToken)
	req.Header.Set("x-requested-with", "XMLHttpRequest")
	req.Header.Set("DataServiceVersion", "2.0")
	req.Header.Set("MaxDataServiceVersion", "2.0")
	req.Header.Set("Accept", "multipart/mixed")
	req.Header.Set("Accept-Language", "en")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("batch request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("batch request: HTTP %d", resp.StatusCode)
	}

	return extractBatchJSON(respBody)
}

// extractBatchJSON pulls the JSON value out of a multipart OData batch response.
// It finds the JSON object in the response, then unwraps the "d" envelope.
// If the value is a string (double-encoded JSON), it returns the raw inner JSON.
func extractBatchJSON(body []byte) ([]byte, error) {
	// Find the start of the JSON object in the multipart response.
	text := string(body)
	idx := strings.Index(text, "{")
	if idx < 0 {
		return nil, fmt.Errorf("no JSON found in batch response")
	}
	jsonStr := text[idx:]

	// Unmarshal the outer OData envelope: {"d": {"FunctionName": <value>}}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &envelope); err != nil {
		return nil, fmt.Errorf("parse batch envelope: %w", err)
	}
	dRaw, ok := envelope["d"]
	if !ok {
		return nil, fmt.Errorf("no 'd' key in batch response")
	}

	// The "d" value is {"FunctionName": <value>} — extract the single value.
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(dRaw, &inner); err != nil {
		return nil, fmt.Errorf("parse batch inner: %w", err)
	}

	// Get the first (only) value regardless of key name.
	for _, v := range inner {
		// Check if the value is a string (double-encoded JSON).
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			// It's a string — return the inner JSON bytes.
			return []byte(s), nil
		}
		// Not a string — return the raw JSON directly.
		return v, nil
	}

	return nil, fmt.Errorf("empty batch inner response")
}
