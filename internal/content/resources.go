package content

import "strings"

// FlattenResources collects all resources from all packs into a single slice.
func FlattenResources(packs []*Pack) []Resource {
	var out []Resource
	for _, p := range packs {
		out = append(out, p.Resources...)
	}
	return out
}

// FilterResources returns resources whose id, title, type, or any tag contains query
// (case-insensitive substring match).
func FilterResources(resources []Resource, query string) []Resource {
	q := strings.ToLower(query)
	var out []Resource
	for _, r := range resources {
		if strings.Contains(strings.ToLower(r.ID), q) ||
			strings.Contains(strings.ToLower(r.Title), q) ||
			strings.Contains(strings.ToLower(r.Type), q) {
			out = append(out, r)
			continue
		}
		for _, tag := range r.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// FindResource returns a pointer to the first resource with an exact ID match, or nil.
func FindResource(resources []Resource, id string) *Resource {
	for i := range resources {
		if resources[i].ID == id {
			return &resources[i]
		}
	}
	return nil
}
