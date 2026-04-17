package content

// FlattenHooks returns all HookDef entries from all packs in order.
func FlattenHooks(packs []*Pack) []HookDef {
	var out []HookDef
	for _, p := range packs {
		out = append(out, p.Hooks...)
	}
	return out
}

// FindHookDef returns the first HookDef with the given ID across packs, or nil.
func FindHookDef(packs []*Pack, id string) *HookDef {
	for _, p := range packs {
		for i := range p.Hooks {
			if p.Hooks[i].ID == id {
				return &p.Hooks[i]
			}
		}
	}
	return nil
}
