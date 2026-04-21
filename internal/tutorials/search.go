package tutorials

import "strings"

// Search returns tutorials matching query against title, description, slug, and tags.
// Results are ranked: title matches first, then others.
func Search(index []TutorialMeta, query string) []TutorialMeta {
	q := strings.ToLower(query)
	var titleMatches, otherMatches []TutorialMeta
	for _, m := range index {
		if strings.Contains(strings.ToLower(m.Title), q) {
			titleMatches = append(titleMatches, m)
			continue
		}
		if strings.Contains(strings.ToLower(m.Description), q) ||
			strings.Contains(strings.ToLower(m.Slug), q) {
			otherMatches = append(otherMatches, m)
			continue
		}
		for _, tag := range m.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				otherMatches = append(otherMatches, m)
				break
			}
		}
	}
	return append(titleMatches, otherMatches...)
}

// FilterByLevel returns tutorials matching the given level.
func FilterByLevel(index []TutorialMeta, level string) []TutorialMeta {
	l := strings.ToLower(level)
	var out []TutorialMeta
	for _, m := range index {
		if strings.ToLower(m.Level) == l {
			out = append(out, m)
		}
	}
	return out
}

// FilterByTags returns tutorials with at least one tag matching (OR, case-insensitive, substring).
func FilterByTags(index []TutorialMeta, tags []string) []TutorialMeta {
	needles := make([]string, len(tags))
	for i, t := range tags {
		needles[i] = strings.ToLower(strings.TrimSpace(t))
	}
	var out []TutorialMeta
	for _, m := range index {
		if matchesAnyTag(m.Tags, needles) {
			out = append(out, m)
		}
	}
	return out
}

func matchesAnyTag(tags, needles []string) bool {
	for _, t := range tags {
		lower := strings.ToLower(t)
		for _, n := range needles {
			if strings.Contains(lower, n) {
				return true
			}
		}
	}
	return false
}

// FindBySlug returns the first tutorial matching slug, or nil.
func FindBySlug(index []TutorialMeta, slug string) *TutorialMeta {
	for i := range index {
		if index[i].Slug == slug {
			return &index[i]
		}
	}
	return nil
}
