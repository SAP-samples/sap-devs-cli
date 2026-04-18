package discovery

import (
	"strings"
	"time"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

const (
	CacheTTL       = 7 * 24 * time.Hour // 7 days
	SearchCacheTTL = 1 * time.Hour
)

var CategoryMapping = map[string]string{
	"appdev":        "Application Development and Automation",
	"intgn":         "Integration",
	"dataanalytics": "Data and Analytics",
	"aicatg":        "Artificial Intelligence",
}

var EffortLabels = map[string]string{
	"0": "<1h",
	"1": "1h",
	"2": "2h",
	"3": "3h+",
}

func ResolveMissions(
	refs []content.DiscoveryMissionRef,
	filters content.DiscoveryProfileFilters,
	cacheDir string,
	force bool,
	client *Client,
) ([]Mission, error) {
	allMissions, err := loadOrFetchMissions(cacheDir, force, client)
	if err != nil {
		return nil, err
	}

	missionByID := make(map[int]Mission, len(allMissions))
	for _, m := range allMissions {
		missionByID[m.ID] = m
	}

	var result []Mission
	seen := make(map[int]bool)

	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if m, ok := missionByID[ref.ID]; ok {
			result = append(result, m)
			seen[ref.ID] = true
		}
	}

	for _, ref := range refs {
		if ref.Featured || seen[ref.ID] {
			continue
		}
		if m, ok := missionByID[ref.ID]; ok {
			result = append(result, m)
			seen[ref.ID] = true
		}
	}

	for _, m := range allMissions {
		if seen[m.ID] {
			continue
		}
		if matchesFilters(m, filters) {
			result = append(result, m)
			seen[m.ID] = true
		}
	}

	return result, nil
}

func ResolveServices(
	refs []content.DiscoveryServiceRef,
	filters content.DiscoveryProfileFilters,
	cacheDir string,
	force bool,
	showDeprecated bool,
	client *Client,
) ([]Service, error) {
	allServices, err := loadOrFetchServices(cacheDir, force, client)
	if err != nil {
		return nil, err
	}

	svcByID := make(map[string]Service, len(allServices))
	for _, s := range allServices {
		svcByID[s.ID] = s
	}

	var result []Service
	seen := make(map[string]bool)

	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if s, ok := svcByID[ref.ID]; ok {
			if !s.IsDeprecatedService || showDeprecated {
				result = append(result, s)
				seen[ref.ID] = true
			}
		}
	}

	for _, ref := range refs {
		if ref.Featured || seen[ref.ID] {
			continue
		}
		if s, ok := svcByID[ref.ID]; ok {
			if !s.IsDeprecatedService || showDeprecated {
				result = append(result, s)
				seen[ref.ID] = true
			}
		}
	}

	categorySet := buildCategorySet(filters.Categories)
	for _, s := range allServices {
		if seen[s.ID] {
			continue
		}
		if s.IsDeprecatedService && !showDeprecated {
			continue
		}
		if len(categorySet) > 0 && !categorySet[s.Category] {
			continue
		}
		result = append(result, s)
		seen[s.ID] = true
	}

	return result, nil
}

func ResolveGuidanceTree(
	cacheDir string,
	force bool,
	domainFilter string,
	client *Client,
) ([]GuidanceNode, error) {
	var tree []GuidanceNode
	var ok bool
	if !force {
		tree, ok = LoadCache[[]GuidanceNode](cacheDir, "guidance-tree", CacheTTL)
	}
	if !ok {
		var err error
		tree, err = client.FetchGuidanceTree()
		if err != nil {
			if stale, staleOK := LoadCacheStale[[]GuidanceNode](cacheDir, "guidance-tree"); staleOK {
				tree = stale
			} else {
				return nil, err
			}
		} else {
			_ = SaveCache(cacheDir, "guidance-tree", tree)
		}
	}

	if domainFilter != "" {
		tree = filterGuidanceByDomain(tree, domainFilter)
	}
	return tree, nil
}

func ResolveGuidanceContent(
	cacheDir string,
	force bool,
	id string,
	client *Client,
) (string, error) {
	cacheName := "guidance/" + id
	if !force {
		if c, ok := LoadCache[string](cacheDir, cacheName, CacheTTL); ok {
			return c, nil
		}
	}
	c, err := client.FetchGuidanceContent(id)
	if err != nil {
		if stale, ok := LoadCacheStale[string](cacheDir, cacheName); ok {
			return stale, nil
		}
		return "", err
	}
	_ = SaveCache(cacheDir, cacheName, c)
	return c, nil
}

func loadOrFetchMissions(cacheDir string, force bool, client *Client) ([]Mission, error) {
	if !force {
		if cached, ok := LoadCache[[]Mission](cacheDir, "missions", CacheTTL); ok {
			return cached, nil
		}
	}
	groups, err := client.FetchMissions()
	if err != nil {
		if stale, ok := LoadCacheStale[[]Mission](cacheDir, "missions"); ok {
			return stale, nil
		}
		return nil, err
	}
	var all []Mission
	for _, g := range groups {
		all = append(all, g.Missions...)
	}
	_ = SaveCache(cacheDir, "missions", all)
	return all, nil
}

func loadOrFetchServices(cacheDir string, force bool, client *Client) ([]Service, error) {
	if !force {
		if cached, ok := LoadCache[[]Service](cacheDir, "services", CacheTTL); ok {
			return cached, nil
		}
	}
	svcs, err := client.FetchServices()
	if err != nil {
		if stale, ok := LoadCacheStale[[]Service](cacheDir, "services"); ok {
			return stale, nil
		}
		return nil, err
	}
	_ = SaveCache(cacheDir, "services", svcs)
	return svcs, nil
}

func matchesFilters(m Mission, f content.DiscoveryProfileFilters) bool {
	if len(f.Products) == 0 && len(f.Categories) == 0 && len(f.FocusTags) == 0 {
		return true
	}
	for _, p := range f.Products {
		if ContainsCSV(m.Product, p) {
			return true
		}
	}
	for _, c := range f.Categories {
		if ContainsCSV(m.Category, c) {
			return true
		}
	}
	for _, t := range f.FocusTags {
		if ContainsCSV(m.FocusTags, t) {
			return true
		}
	}
	return false
}

func ContainsCSV(csv, val string) bool {
	for _, v := range strings.Split(csv, ",") {
		if strings.TrimSpace(v) == val {
			return true
		}
	}
	return false
}

func buildCategorySet(codes []string) map[string]bool {
	if len(codes) == 0 {
		return nil
	}
	set := make(map[string]bool)
	for _, code := range codes {
		if mapped, ok := CategoryMapping[code]; ok {
			set[mapped] = true
		} else {
			set[code] = true
		}
	}
	return set
}

func filterGuidanceByDomain(nodes []GuidanceNode, domain string) []GuidanceNode {
	d := strings.ToLower(domain)
	var result []GuidanceNode
	for _, phase := range nodes {
		var children []GuidanceNode
		for _, child := range phase.Children {
			if child.Domain != nil && strings.Contains(strings.ToLower(*child.Domain), d) {
				children = append(children, child)
			}
		}
		if len(children) > 0 {
			filtered := phase
			filtered.Children = children
			result = append(result, filtered)
		}
	}
	return result
}
