package content

import (
	"math/rand"
	"strings"
)

// FlattenInfluencers collects all influencers from all packs into a single slice.
func FlattenInfluencers(packs []*Pack) []Influencer {
	var out []Influencer
	for _, p := range packs {
		out = append(out, p.Influencers...)
	}
	return out
}

// FilterInfluencersByTags returns influencers with at least one focus tag
// matching any of the provided tags (OR semantics, case-insensitive).
func FilterInfluencersByTags(influencers []Influencer, tags []string) []Influencer {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(strings.TrimSpace(t))] = true
	}
	var out []Influencer
	for _, inf := range influencers {
		for _, f := range inf.Focus {
			if tagSet[strings.ToLower(f)] {
				out = append(out, inf)
				break
			}
		}
	}
	return out
}

// FilterInfluencersByPack returns influencers from packs matching the given pack ID.
func FilterInfluencersByPack(packs []*Pack, packID string) []Influencer {
	for _, p := range packs {
		if p.ID == packID {
			return p.Influencers
		}
	}
	return nil
}

// FindInfluencer returns a pointer to the first influencer with an exact ID match, or nil.
func FindInfluencer(influencers []Influencer, id string) *Influencer {
	for i := range influencers {
		if influencers[i].ID == id {
			return &influencers[i]
		}
	}
	return nil
}

// RandomInfluencer returns one random influencer from the slice using the given seed.
func RandomInfluencer(influencers []Influencer, seed int64) *Influencer {
	if len(influencers) == 0 {
		return nil
	}
	r := rand.New(rand.NewSource(seed))
	return &influencers[r.Intn(len(influencers))]
}
