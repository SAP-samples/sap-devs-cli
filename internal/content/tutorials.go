package content

// FlattenTutorialRefs collects all tutorial references from all packs.
func FlattenTutorialRefs(packs []*Pack) []TutorialRef {
	var out []TutorialRef
	for _, p := range packs {
		out = append(out, p.TutorialRefs...)
	}
	return out
}

// FilterTutorialRefsByPack returns tutorial refs from the matching pack.
func FilterTutorialRefsByPack(packs []*Pack, packID string) []TutorialRef {
	for _, p := range packs {
		if p.ID == packID {
			return p.TutorialRefs
		}
	}
	return nil
}

// FindTutorialRef returns the first tutorial ref matching slug, or nil.
func FindTutorialRef(packs []*Pack, slug string) *TutorialRef {
	for _, p := range packs {
		for i := range p.TutorialRefs {
			if p.TutorialRefs[i].Slug == slug {
				return &p.TutorialRefs[i]
			}
		}
	}
	return nil
}
