package learning

import "strings"

// Search performs case-insensitive substring matching across title, description, slug, and product.
func Search(journeys []LearningJourney, query string) []LearningJourney {
	q := strings.ToLower(query)
	var out []LearningJourney
	for _, j := range journeys {
		if strings.Contains(strings.ToLower(j.Title), q) ||
			strings.Contains(strings.ToLower(j.Description), q) ||
			strings.Contains(strings.ToLower(j.Slug), q) ||
			strings.Contains(strings.ToLower(j.Product), q) {
			out = append(out, j)
		}
	}
	return out
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
