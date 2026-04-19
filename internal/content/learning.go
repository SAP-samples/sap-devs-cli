package content

import "strings"

// FlattenLearningRefs collects all curated learning refs from all packs.
func FlattenLearningRefs(packs []*Pack) []LearningRef {
	var out []LearningRef
	for _, p := range packs {
		out = append(out, p.LearningRefs...)
	}
	return out
}

// CollectLearningFilters unions all LearningProfileFilters across active packs.
func CollectLearningFilters(packs []*Pack) LearningProfileFilters {
	products := make(map[string]bool)
	categories := make(map[string]bool)
	roles := make(map[string]bool)

	for _, p := range packs {
		if p.LearningFilters == nil {
			continue
		}
		for _, v := range p.LearningFilters.Products {
			products[v] = true
		}
		for _, v := range p.LearningFilters.ProductCategories {
			categories[v] = true
		}
		for _, v := range p.LearningFilters.Roles {
			roles[v] = true
		}
	}

	return LearningProfileFilters{
		Products:          setToSlice(products),
		ProductCategories: setToSlice(categories),
		Roles:             setToSlice(roles),
	}
}

// MatchesLearningFilters checks if a journey matches the profile filters.
func MatchesLearningFilters(product, productCategory string, roles []string, f LearningProfileFilters) bool {
	if len(f.Products) == 0 && len(f.ProductCategories) == 0 && len(f.Roles) == 0 {
		return true
	}
	for _, fp := range f.Products {
		if strings.Contains(product, fp) {
			return true
		}
	}
	for _, fc := range f.ProductCategories {
		if strings.Contains(productCategory, fc) {
			return true
		}
	}
	for _, fr := range f.Roles {
		for _, r := range roles {
			if strings.EqualFold(r, fr) {
				return true
			}
		}
	}
	return false
}
