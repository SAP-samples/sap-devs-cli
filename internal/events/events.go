package events

import (
	"sort"
	"strings"
	"time"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/geo"
)

const (
	defaultCacheTTL       = 4 * time.Hour
	liveFetchTimeout      = 3 * time.Second
	khorosFetchTimeout    = 10 * time.Second
)

// Resolve returns events for a remotely-sourced event type (RSS or Khoros).
// Checks cache freshness first; fetches live if stale; falls back to stale cache on failure.
// Pass force=true to bypass the TTL check (used by sync --force).
func Resolve(et content.EventType, cacheDir string, force bool) ([]content.EventInstance, error) {
	var fetcher func() ([]content.EventInstance, error)
	switch et.Source {
	case "rss":
		if et.RSSURL == "" {
			return nil, nil
		}
		fetcher = func() ([]content.EventInstance, error) {
			return FetchRSS(et.RSSURL, et.ID, et.DefaultScope, liveFetchTimeout)
		}
	case "khoros":
		if et.KhorosBoardID == "" {
			return nil, nil
		}
		fetcher = func() ([]content.EventInstance, error) {
			return FetchKhoros(et.KhorosBoardID, et.ID, et.DefaultScope, khorosFetchTimeout)
		}
	default:
		return nil, nil
	}

	if !force {
		age := CacheAge(cacheDir, et.ID)
		if age >= 0 && age < defaultCacheTTL {
			return LoadCache(cacheDir, et.ID)
		}
	}

	evts, err := fetcher()
	if err == nil {
		_ = SaveCache(cacheDir, et.ID, evts)
		return evts, nil
	}

	if cached, cacheErr := LoadCache(cacheDir, et.ID); cacheErr == nil {
		return cached, nil
	}

	return nil, nil
}

// FilterByLocation filters events based on user location and scope-based radius thresholds.
func FilterByLocation(events []content.EventInstance, userLat, userLon float64, localRadius, regionalRadius int) []content.EventInstance {
	var out []content.EventInstance
	for _, e := range events {
		scope := strings.ToLower(e.Scope)
		if scope == "virtual" || scope == "global" {
			out = append(out, e)
			continue
		}
		if e.Location == "" || strings.EqualFold(e.Location, "virtual") {
			out = append(out, e)
			continue
		}
		eLat, eLon, ok := geo.Lookup(e.Location)
		if !ok {
			out = append(out, e)
			continue
		}
		switch scope {
		case "regional":
			if geo.IsNearby(userLat, userLon, eLat, eLon, float64(regionalRadius)) {
				out = append(out, e)
			}
		case "local":
			if geo.IsNearby(userLat, userLon, eLat, eLon, float64(localRadius)) {
				out = append(out, e)
			}
		default:
			out = append(out, e)
		}
	}
	return out
}

// MergeAndSort combines two event slices, deduplicates by ID, sorts by date ascending.
func MergeAndSort(a, b []content.EventInstance) []content.EventInstance {
	seen := make(map[string]bool)
	var merged []content.EventInstance
	for _, list := range [][]content.EventInstance{a, b} {
		for _, e := range list {
			if !seen[e.ID] {
				seen[e.ID] = true
				merged = append(merged, e)
			}
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].DateStr < merged[j].DateStr
	})
	return merged
}
