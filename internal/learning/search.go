package learning

import "strings"

// Search performs case-insensitive multi-word matching across title, description, slug, and product.
// Multi-word queries use AND semantics: every word must appear somewhere in the combined text.
// Results are ranked: title matches first, then others.
func Search(journeys []LearningJourney, query string) []LearningJourney {
	words := splitQuery(query)
	if len(words) == 0 {
		return nil
	}
	var titleMatches, otherMatches []LearningJourney
	for _, j := range journeys {
		title := strings.ToLower(j.Title)
		if containsAllWords(title, words) {
			titleMatches = append(titleMatches, j)
		} else {
			combined := title + " " + strings.ToLower(j.Description) + " " + strings.ToLower(j.Slug) + " " + strings.ToLower(j.Product)
			if containsAllWords(combined, words) {
				otherMatches = append(otherMatches, j)
			}
		}
	}
	return append(titleMatches, otherMatches...)
}

func splitQuery(query string) []string {
	var words []string
	for _, w := range strings.Fields(strings.ToLower(query)) {
		if w != "" {
			words = append(words, w)
		}
	}
	return words
}

func containsAllWords(text string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

// FilterByLevel returns journeys matching the given level (case-insensitive exact match).
func FilterByLevel(journeys []LearningJourney, level string) []LearningJourney {
	l := strings.ToUpper(level)
	var out []LearningJourney
	for _, j := range journeys {
		if strings.EqualFold(j.Level, l) {
			out = append(out, j)
		}
	}
	return out
}

// FilterByRole returns journeys where at least one role matches (case-insensitive).
func FilterByRole(journeys []LearningJourney, role string) []LearningJourney {
	r := strings.ToLower(role)
	var out []LearningJourney
	for _, j := range journeys {
		for _, jr := range j.Roles {
			if strings.EqualFold(jr, r) {
				out = append(out, j)
				break
			}
		}
	}
	return out
}

// FindBySlug returns the first journey with the given slug, or nil.
func FindBySlug(journeys []LearningJourney, slug string) *LearningJourney {
	for i := range journeys {
		if journeys[i].Slug == slug {
			return &journeys[i]
		}
	}
	return nil
}
