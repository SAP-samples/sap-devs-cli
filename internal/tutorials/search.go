package tutorials

import "strings"

// Search returns tutorials matching query against title, description, slug, and tags.
// Multi-word queries use AND semantics: every word must appear somewhere in the combined text.
// Results are ranked: title matches first, then others.
func Search(index []TutorialMeta, query string) []TutorialMeta {
	words := splitQuery(query)
	if len(words) == 0 {
		return nil
	}
	var titleMatches, otherMatches []TutorialMeta
	for _, m := range index {
		title := strings.ToLower(m.Title)
		if containsAllWords(title, words) {
			titleMatches = append(titleMatches, m)
			continue
		}
		combined := title + " " + strings.ToLower(m.Description) + " " + strings.ToLower(m.Slug)
		for _, tag := range m.Tags {
			combined += " " + strings.ToLower(tag)
		}
		if containsAllWords(combined, words) {
			otherMatches = append(otherMatches, m)
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
