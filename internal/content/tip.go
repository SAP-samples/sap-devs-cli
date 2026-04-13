package content

import (
	"errors"
	"math/rand"
)

// SelectTip picks a tip from the filtered pool using the given seed.
// profileTags narrows the pool to tips that share at least one tag with the profile.
// seed 0 means "today" (use time.Now().YearDay() * year as seed for daily consistency).
func SelectTip(packs []*Pack, profileTags []string, seed int64) (*Tip, error) {
	tagSet := make(map[string]bool, len(profileTags))
	for _, t := range profileTags {
		tagSet[t] = true
	}

	var pool []Tip
	for _, pack := range packs {
		for _, tip := range pack.Tips {
			if len(profileTags) == 0 {
				pool = append(pool, tip)
				continue
			}
			for _, tag := range tip.Tags {
				if tagSet[tag] {
					pool = append(pool, tip)
					break
				}
			}
		}
	}

	if len(pool) == 0 {
		return nil, errors.New("no tips available for the current profile tags")
	}

	r := rand.New(rand.NewSource(seed)) //nolint:gosec // non-cryptographic selection
	idx := r.Intn(len(pool))
	return &pool[idx], nil
}
