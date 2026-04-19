package learn

import (
	"strings"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/discovery"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

const (
	scoreTitle = 2
	scoreDesc  = 1
)

type scoredItem struct {
	item  LearnItem
	score int
}

func Search(
	journeys []learning.LearningJourney,
	tuts []tutorials.TutorialMeta,
	missions []discovery.Mission,
	query string,
	opts RecommendOptions,
) []LearnItem {
	q := strings.ToLower(query)
	var scored []scoredItem

	for _, j := range journeys {
		s := matchScore(j.Title, j.Description, q)
		if s > 0 {
			scored = append(scored, scoredItem{journeyToItem(j, false, ""), s})
		}
	}
	for _, t := range tuts {
		s := matchScore(t.Title, t.Description, q)
		if s > 0 {
			scored = append(scored, scoredItem{tutorialToItem(t, false, ""), s})
		}
	}
	for _, m := range missions {
		s := matchScore(m.Name, m.UCLongDescription, q)
		if s > 0 {
			scored = append(scored, scoredItem{missionToItem(m, false, ""), s})
		}
	}

	// Stable sort: title matches first, then description matches
	sortScored(scored)

	var items []LearnItem
	for _, s := range scored {
		items = append(items, s.item)
	}

	items = filterByLevel(items, opts.Level)
	return capItems(items, opts.Limit)
}

func matchScore(title, description, query string) int {
	score := 0
	if strings.Contains(strings.ToLower(title), query) {
		score += scoreTitle
	}
	if strings.Contains(strings.ToLower(description), query) {
		score += scoreDesc
	}
	return score
}

func sortScored(items []scoredItem) {
	// Simple insertion sort (small N, stable)
	for i := 1; i < len(items); i++ {
		key := items[i]
		j := i - 1
		for j >= 0 && items[j].score < key.score {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = key
	}
}
