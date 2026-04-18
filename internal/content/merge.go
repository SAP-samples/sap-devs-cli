package content

// MergeWith returns a new *Pack that augments base with the content of a.
// If a.Additive is false, MergeWith is a no-op and returns base unchanged.
func (a *Pack) MergeWith(base *Pack) *Pack {
	if !a.Additive {
		return base
	}
	merged := *base // shallow copy of scalar fields; slices replaced below

	// Metadata: override on non-empty / non-zero only.
	if a.Name != "" {
		merged.Name = a.Name
	}
	if a.Description != "" {
		merged.Description = a.Description
	}
	if a.Weight != 0 {
		merged.Weight = a.Weight
	}
	merged.Tags = unionStrings(base.Tags, a.Tags)
	// Profiles and Overlaps always come from base; produce fresh slices to avoid
	// aliasing base's backing arrays if callers ever append to the result.
	merged.Profiles = append([]string(nil), base.Profiles...)
	merged.Overlaps = append([]string(nil), base.Overlaps...)

	// Context: position controls order. Empty additive ContextMD preserves base unchanged.
	if a.ContextMD != "" {
		if a.AdditivePosition == "before" {
			merged.ContextMD = a.ContextMD + "\n\n" + base.ContextMD
		} else {
			merged.ContextMD = base.ContextMD + "\n\n" + a.ContextMD
		}
	}

	// Tips: both sets kept; position controls order. Always fresh slice.
	if a.AdditivePosition == "before" {
		merged.Tips = append(append([]Tip(nil), a.Tips...), base.Tips...)
	} else {
		merged.Tips = append(append([]Tip(nil), base.Tips...), a.Tips...)
	}

	// Structured lists: additive replaces on matching ID, appends new entries.
	// PackID re-stamped to base pack's ID on Resources, MCPServers, and Hooks.
	merged.Resources = mergeResources(base.Resources, a.Resources, base.ID)
	merged.Tools = mergeTools(base.Tools, a.Tools)
	merged.MCPServers = mergeMCPServers(base.MCPServers, a.MCPServers, base.ID)
	merged.Hooks = mergeHooks(base.Hooks, a.Hooks, base.ID)
	merged.Influencers = mergeInfluencers(base.Influencers, a.Influencers, base.ID)
	merged.EventTypes = mergeEventTypes(base.EventTypes, a.EventTypes, base.ID)
	merged.EventInstances = mergeEventInstances(base.EventInstances, a.EventInstances, base.ID)
	merged.Samples = mergeSamples(base.Samples, a.Samples, base.ID)

	// Merged result is not itself additive; a subsequent additive layer will
	// merge into this result rather than treating it as an additive pack.
	merged.Additive = false
	merged.AdditivePosition = "" // no position on a non-additive pack
	return &merged
}

// unionStrings returns a fresh deduplicated slice: all elements of a,
// then elements of b not already present in a. Order is preserved.
func unionStrings(a, b []string) []string {
	seen := make(map[string]bool, len(a))
	result := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// mergeResources builds a fresh []Resource: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeResources(base, additive []Resource, packID string) []Resource {
	result := make([]Resource, len(base))
	copy(result, base)
	for _, a := range additive {
		replaced := false
		for i, b := range result {
			if b.ID == a.ID {
				result[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, a)
		}
	}
	// Re-stamp all entries: base entries are already stamped correctly by LoadPack,
	// but replaced/new additive entries carry the additive layer's ID — normalise all.
	for i := range result {
		result[i].PackID = packID
	}
	return result
}

// mergeTools builds a fresh []ToolDef: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
func mergeTools(base, additive []ToolDef) []ToolDef {
	result := make([]ToolDef, len(base))
	copy(result, base)
	for _, a := range additive {
		replaced := false
		for i, b := range result {
			if b.ID == a.ID {
				result[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, a)
		}
	}
	return result
}

// mergeHooks builds a fresh []HookDef: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeHooks(base, additive []HookDef, packID string) []HookDef {
	result := make([]HookDef, len(base))
	copy(result, base)
	for _, a := range additive {
		replaced := false
		for i, b := range result {
			if b.ID == a.ID {
				result[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, a)
		}
	}
	// Re-stamp all entries: same rationale as mergeResources.
	for i := range result {
		result[i].PackID = packID
	}
	return result
}

// mergeInfluencers builds a fresh []Influencer: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeInfluencers(base, additive []Influencer, packID string) []Influencer {
	result := make([]Influencer, len(base))
	copy(result, base)
	for _, a := range additive {
		replaced := false
		for i, b := range result {
			if b.ID == a.ID {
				result[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, a)
		}
	}
	for i := range result {
		result[i].PackID = packID
	}
	return result
}

// mergeEventTypes builds a fresh []EventType: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeEventTypes(base, additive []EventType, packID string) []EventType {
	result := make([]EventType, len(base))
	copy(result, base)
	for _, a := range additive {
		replaced := false
		for i, b := range result {
			if b.ID == a.ID {
				result[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, a)
		}
	}
	for i := range result {
		result[i].PackID = packID
	}
	return result
}

// mergeEventInstances builds a fresh []EventInstance: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeEventInstances(base, additive []EventInstance, packID string) []EventInstance {
	result := make([]EventInstance, len(base))
	copy(result, base)
	for _, a := range additive {
		replaced := false
		for i, b := range result {
			if b.ID == a.ID {
				result[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, a)
		}
	}
	for i := range result {
		result[i].PackID = packID
	}
	return result
}

// mergeSamples builds a fresh []Sample: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeSamples(base, additive []Sample, packID string) []Sample {
	result := make([]Sample, len(base))
	copy(result, base)
	for _, a := range additive {
		replaced := false
		for i, b := range result {
			if b.ID == a.ID {
				result[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, a)
		}
	}
	for i := range result {
		result[i].PackID = packID
	}
	return result
}

// mergeMCPServers builds a fresh []MCPServer: starts with base entries, replaces
// any entry whose ID matches an additive entry, appends unmatched additive entries.
// PackID is re-stamped to packID on every entry in the result.
func mergeMCPServers(base, additive []MCPServer, packID string) []MCPServer {
	result := make([]MCPServer, len(base))
	copy(result, base)
	for _, a := range additive {
		replaced := false
		for i, b := range result {
			if b.ID == a.ID {
				result[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, a)
		}
	}
	// Re-stamp all entries: same rationale as mergeResources.
	for i := range result {
		result[i].PackID = packID
	}
	return result
}
