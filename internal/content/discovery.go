package content

// FlattenDiscoveryMissionRefs collects all curated mission refs from all packs.
func FlattenDiscoveryMissionRefs(packs []*Pack) []DiscoveryMissionRef {
	var out []DiscoveryMissionRef
	for _, p := range packs {
		out = append(out, p.DiscoveryMissions...)
	}
	return out
}

// FlattenDiscoveryServiceRefs collects all curated service refs from all packs.
func FlattenDiscoveryServiceRefs(packs []*Pack) []DiscoveryServiceRef {
	var out []DiscoveryServiceRef
	for _, p := range packs {
		out = append(out, p.DiscoveryServices...)
	}
	return out
}

// FlattenDiscoveryGuidanceRefs collects all curated guidance refs from all packs.
func FlattenDiscoveryGuidanceRefs(packs []*Pack) []DiscoveryGuidanceRef {
	var out []DiscoveryGuidanceRef
	for _, p := range packs {
		out = append(out, p.DiscoveryGuidance...)
	}
	return out
}

// CollectProfileFilters unions all DiscoveryProfileFilters across active packs.
func CollectProfileFilters(packs []*Pack) DiscoveryProfileFilters {
	products := make(map[string]bool)
	categories := make(map[string]bool)
	focusTags := make(map[string]bool)

	for _, p := range packs {
		if p.DiscoveryFilters == nil {
			continue
		}
		for _, v := range p.DiscoveryFilters.Products {
			products[v] = true
		}
		for _, v := range p.DiscoveryFilters.Categories {
			categories[v] = true
		}
		for _, v := range p.DiscoveryFilters.FocusTags {
			focusTags[v] = true
		}
	}

	return DiscoveryProfileFilters{
		Products:   setToSlice(products),
		Categories: setToSlice(categories),
		FocusTags:  setToSlice(focusTags),
	}
}

func setToSlice(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
