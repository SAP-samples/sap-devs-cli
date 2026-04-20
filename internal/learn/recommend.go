package learn

import (
	"fmt"
	"strings"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/discovery"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
)

func Recommend(
	journeys []learning.LearningJourney,
	tuts []tutorials.TutorialMeta,
	missions []discovery.Mission,
	packs []*content.Pack,
	opts RecommendOptions,
) *Recommendations {
	recs := &Recommendations{
		Journeys:  recommendJourneys(journeys, packs, opts),
		Tutorials: recommendTutorials(tuts, packs, opts),
		Missions:  recommendMissions(missions, packs, opts),
	}
	return recs
}

func recommendJourneys(journeys []learning.LearningJourney, packs []*content.Pack, opts RecommendOptions) []LearnItem {
	refs := content.FlattenLearningRefs(packs)
	filters := content.LearningProfileFilters{}
	if !opts.All {
		filters = content.CollectLearningFilters(packs)
	}

	bySlug := make(map[string]learning.LearningJourney, len(journeys))
	for _, j := range journeys {
		bySlug[j.Slug] = j
	}

	var items []LearnItem
	seen := make(map[string]bool)

	// Tier 1: featured refs
	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if j, ok := bySlug[ref.Slug]; ok && !seen[ref.Slug] {
			items = append(items, journeyToItem(j, true, ref.PackID))
			seen[ref.Slug] = true
		}
	}

	// Tier 2: non-featured pack refs
	for _, ref := range refs {
		if ref.Featured || seen[ref.Slug] {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if j, ok := bySlug[ref.Slug]; ok {
			items = append(items, journeyToItem(j, false, ref.PackID))
			seen[ref.Slug] = true
		}
	}

	// Tier 3: profile-filtered or all
	for _, j := range journeys {
		if seen[j.Slug] {
			continue
		}
		if opts.All || content.MatchesLearningFilters(j.Product, j.ProductCategory, j.Roles, filters) {
			items = append(items, journeyToItem(j, false, ""))
			seen[j.Slug] = true
		}
	}

	items = filterByLevel(items, opts.Level)
	return capItems(items, opts.Limit)
}

func recommendTutorials(tuts []tutorials.TutorialMeta, packs []*content.Pack, opts RecommendOptions) []LearnItem {
	refs := content.FlattenTutorialRefs(packs)

	bySlug := make(map[string]tutorials.TutorialMeta, len(tuts))
	for _, t := range tuts {
		bySlug[t.Slug] = t
	}

	var items []LearnItem
	seen := make(map[string]bool)

	// Tier 1: featured refs
	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if t, ok := bySlug[ref.Slug]; ok && !seen[ref.Slug] {
			items = append(items, tutorialToItem(t, true, ref.PackID))
			seen[ref.Slug] = true
		}
	}

	// Tier 2: non-featured pack refs
	for _, ref := range refs {
		if ref.Featured || seen[ref.Slug] {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if t, ok := bySlug[ref.Slug]; ok {
			items = append(items, tutorialToItem(t, false, ref.PackID))
			seen[ref.Slug] = true
		}
	}

	// Tier 3: all remaining (tutorials don't have profile filters, so show all if --all or pack-scoped)
	if opts.All {
		for _, t := range tuts {
			if !seen[t.Slug] {
				items = append(items, tutorialToItem(t, false, ""))
				seen[t.Slug] = true
			}
		}
	}

	items = filterByLevel(items, opts.Level)
	return capItems(items, opts.Limit)
}

func recommendMissions(missions []discovery.Mission, packs []*content.Pack, opts RecommendOptions) []LearnItem {
	refs := content.FlattenDiscoveryMissionRefs(packs)

	byID := make(map[int]discovery.Mission, len(missions))
	for _, m := range missions {
		byID[m.ID] = m
	}

	var items []LearnItem
	seen := make(map[int]bool)

	// Tier 1: featured refs
	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if m, ok := byID[ref.ID]; ok && !seen[ref.ID] {
			items = append(items, missionToItem(m, true, ref.PackID))
			seen[ref.ID] = true
		}
	}

	// Tier 2: non-featured pack refs
	for _, ref := range refs {
		if ref.Featured || seen[ref.ID] {
			continue
		}
		if opts.PackID != "" && ref.PackID != opts.PackID {
			continue
		}
		if m, ok := byID[ref.ID]; ok {
			items = append(items, missionToItem(m, false, ref.PackID))
			seen[ref.ID] = true
		}
	}

	// Tier 3: all remaining if --all
	if opts.All {
		for _, m := range missions {
			if !seen[m.ID] {
				items = append(items, missionToItem(m, false, ""))
				seen[m.ID] = true
			}
		}
	}

	items = filterByLevel(items, opts.Level)
	return capItems(items, opts.Limit)
}

func journeyToItem(j learning.LearningJourney, featured bool, packID string) LearnItem {
	return LearnItem{
		Type:     ItemJourney,
		Title:    j.Title,
		Slug:     j.Slug,
		Level:    NormalizeLevel(j.Level),
		Duration: formatJourneyDuration(j.DurationHours),
		URL:      j.URL,
		Featured: featured,
		PackID:   packID,
		Product:  j.Product,
	}
}

func tutorialToItem(t tutorials.TutorialMeta, featured bool, packID string) LearnItem {
	return LearnItem{
		Type:     ItemTutorial,
		Title:    t.Title,
		Slug:     t.Slug,
		Level:    NormalizeLevel(t.Level),
		Duration: formatTutorialDuration(t.Time),
		URL:      t.URL,
		Featured: featured,
		PackID:   packID,
	}
}

func missionToItem(m discovery.Mission, featured bool, packID string) LearnItem {
	return LearnItem{
		Type:     ItemMission,
		Title:    m.Name,
		Slug:     fmt.Sprintf("%d", m.ID),
		Level:    effortToLevel(m.Effort),
		Duration: "",
		URL:      fmt.Sprintf("https://discovery-center.cloud.sap/missiondetail/%d/", m.ID),
		Featured: featured,
		PackID:   packID,
		Product:  m.Product,
	}
}

// NormalizeLevel converts any level string to lowercase canonical form.
func NormalizeLevel(level string) string {
	return strings.ToLower(strings.TrimSpace(level))
}

func effortToLevel(effort string) string {
	switch effort {
	case "0", "1":
		return "beginner"
	case "2":
		return "intermediate"
	case "3":
		return "advanced"
	default:
		return ""
	}
}

func formatJourneyDuration(hours string) string {
	if hours == "" {
		return ""
	}
	return hours + " hr"
}

func formatTutorialDuration(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	return fmt.Sprintf("%d min", minutes)
}

func filterByLevel(items []LearnItem, level string) []LearnItem {
	if level == "" {
		return items
	}
	norm := NormalizeLevel(level)
	var out []LearnItem
	for _, item := range items {
		if item.Level == norm {
			out = append(out, item)
		}
	}
	return out
}

func capItems(items []LearnItem, limit int) []LearnItem {
	if limit <= 0 || limit >= len(items) {
		return items
	}
	return items[:limit]
}
