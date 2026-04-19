package content

import "strings"

// FlattenKnownErrors collects all known errors from all packs into a single slice.
func FlattenKnownErrors(packs []*Pack) []KnownError {
	var out []KnownError
	for _, p := range packs {
		out = append(out, p.KnownErrors...)
	}
	return out
}

// FilterKnownErrorsByTags returns errors with at least one tag matching any of the
// provided tags (OR semantics, case-insensitive).
func FilterKnownErrorsByTags(errors []KnownError, tags []string) []KnownError {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(strings.TrimSpace(t))] = true
	}
	var out []KnownError
	for _, e := range errors {
		for _, t := range e.Tags {
			if tagSet[strings.ToLower(t)] {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// FilterKnownErrorsByPack returns errors from the pack matching the given pack ID.
func FilterKnownErrorsByPack(packs []*Pack, packID string) []KnownError {
	for _, p := range packs {
		if p.ID == packID {
			return p.KnownErrors
		}
	}
	return nil
}

// FilterKnownErrors returns errors whose ID, Pattern, Cause, Fix, or any Tag
// contains query (case-insensitive substring match).
func FilterKnownErrors(errors []KnownError, query string) []KnownError {
	q := strings.ToLower(query)
	var out []KnownError
	for _, e := range errors {
		if strings.Contains(strings.ToLower(e.ID), q) ||
			strings.Contains(strings.ToLower(e.Pattern), q) ||
			strings.Contains(strings.ToLower(e.Cause), q) ||
			strings.Contains(strings.ToLower(e.Fix), q) {
			out = append(out, e)
			continue
		}
		for _, tag := range e.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// FindKnownError returns a pointer to the first error with an exact ID match, or nil.
func FindKnownError(errors []KnownError, id string) *KnownError {
	for i := range errors {
		if errors[i].ID == id {
			return &errors[i]
		}
	}
	return nil
}
