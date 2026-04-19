package learning

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchCatalog downloads the full catalog and returns only learning journeys.
func FetchCatalog() ([]LearningJourney, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(CatalogURL)
	if err != nil {
		return nil, fmt.Errorf("fetch catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch catalog: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}

	var items []catalogItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}

	var journeys []LearningJourney
	for _, item := range items {
		if item.LearningType != "Learning Journey" {
			continue
		}
		j := convertCatalogItem(item)
		if j.Slug == "" {
			continue
		}
		journeys = append(journeys, j)
	}
	return journeys, nil
}

func convertCatalogItem(item catalogItem) LearningJourney {
	slug := extractSlug(item.DirectLink.Hyperlink)
	var roles []string
	for _, r := range strings.Split(item.Role, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			roles = append(roles, r)
		}
	}
	return LearningJourney{
		ObjectID:        item.LearningObjectID,
		Title:           item.Title,
		Slug:            slug,
		Description:     item.Description,
		Level:           item.Level,
		DurationHours:   item.DurationInHours,
		Roles:           roles,
		Product:         item.Product,
		ProductCategory: item.ProductCategory,
		ProductSubcat:   item.ProductSubcat,
		Objectives:      item.Objectives,
		AvailableFrom:   item.AvailableFrom,
		URL:             item.DirectLink.Hyperlink,
	}
}

func extractSlug(url string) string {
	const prefix = "https://learning.sap.com/learning-journeys/"
	if strings.HasPrefix(url, prefix) {
		return strings.TrimSuffix(strings.TrimPrefix(url, prefix), "/")
	}
	return ""
}
