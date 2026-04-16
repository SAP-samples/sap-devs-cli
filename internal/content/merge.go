package content

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
	for i := range result {
		result[i].PackID = packID
	}
	return result
}
