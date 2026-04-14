package content

// FlattenMCPServers returns all MCPServer entries from all packs in order.
func FlattenMCPServers(packs []*Pack) []MCPServer {
	var out []MCPServer
	for _, p := range packs {
		out = append(out, p.MCPServers...)
	}
	return out
}

// FindMCPServer returns the first MCPServer with the given ID across packs, or nil.
func FindMCPServer(packs []*Pack, id string) *MCPServer {
	for _, p := range packs {
		for i := range p.MCPServers {
			if p.MCPServers[i].ID == id {
				return &p.MCPServers[i]
			}
		}
	}
	return nil
}
