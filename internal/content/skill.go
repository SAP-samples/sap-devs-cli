package content

// FlattenSkills returns all SkillDef entries from all packs in order.
func FlattenSkills(packs []*Pack) []SkillDef {
	var out []SkillDef
	for _, p := range packs {
		out = append(out, p.Skills...)
	}
	return out
}

// FindSkillDef returns the first SkillDef with the given ID across packs, or nil.
func FindSkillDef(packs []*Pack, id string) *SkillDef {
	for _, p := range packs {
		for i := range p.Skills {
			if p.Skills[i].ID == id {
				return &p.Skills[i]
			}
		}
	}
	return nil
}
