package content

import "strings"

// FlattenSamples collects all samples from all packs into a single slice.
func FlattenSamples(packs []*Pack) []Sample {
	var out []Sample
	for _, p := range packs {
		out = append(out, p.Samples...)
	}
	return out
}

// FilterSamplesByTags returns samples with at least one tag matching any of the
// provided tags (OR semantics, case-insensitive).
func FilterSamplesByTags(samples []Sample, tags []string) []Sample {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(strings.TrimSpace(t))] = true
	}
	var out []Sample
	for _, s := range samples {
		for _, t := range s.Tags {
			if tagSet[strings.ToLower(t)] {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// FilterSamplesByPack returns samples from the pack matching the given pack ID.
func FilterSamplesByPack(packs []*Pack, packID string) []Sample {
	for _, p := range packs {
		if p.ID == packID {
			return p.Samples
		}
	}
	return nil
}

// FilterSamples returns samples whose ID, Label, Description, or any Tag
// contains query (case-insensitive substring match).
func FilterSamples(samples []Sample, query string) []Sample {
	q := strings.ToLower(query)
	var out []Sample
	for _, s := range samples {
		if strings.Contains(strings.ToLower(s.ID), q) ||
			strings.Contains(strings.ToLower(s.Label), q) ||
			strings.Contains(strings.ToLower(s.Description), q) {
			out = append(out, s)
			continue
		}
		for _, tag := range s.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// FindSample returns a pointer to the first sample with an exact ID match, or nil.
func FindSample(samples []Sample, id string) *Sample {
	for i := range samples {
		if samples[i].ID == id {
			return &samples[i]
		}
	}
	return nil
}
