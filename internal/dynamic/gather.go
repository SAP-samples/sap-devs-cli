// internal/dynamic/gather.go
package dynamic

import (
	"encoding/json"
	"os"

	"github.com/SAP-samples/sap-devs-cli/internal/adapter"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/project"
	sapSync "github.com/SAP-samples/sap-devs-cli/internal/sync"
)

// GatherOpts holds all inputs needed to collect dynamic context at inject time.
type GatherOpts struct {
	CWD            string
	CLIVersion     string
	Profile        *content.Profile
	Packs          []*content.Pack
	SyncStateDir   string
	Adapters       []adapter.Adapter
	Commands       []content.CommandInfo
	ProjectContext *project.ProjectContext // pre-detected; nil = auto-detect
}

// GatherDynamic collects runtime context from the local environment.
// All sub-steps silently skip on error; the returned pointer is never nil.
func GatherDynamic(opts GatherOpts) *content.DynamicContext {
	d := &content.DynamicContext{}

	// CLI self-awareness
	d.CLIVersion = opts.CLIVersion
	if opts.Profile != nil {
		if opts.Profile.Name != "" {
			d.ActiveProfile = opts.Profile.Name
		} else {
			d.ActiveProfile = opts.Profile.ID
		}
	}
	for _, p := range opts.Packs {
		d.LoadedPackIDs = append(d.LoadedPackIDs, p.ID)
	}
	d.Commands = opts.Commands

	// Pack freshness
	if opts.SyncStateDir != "" {
		d.LastSynced = sapSync.MostRecentSync(opts.SyncStateDir)
	}

	// Project detection
	pc := opts.ProjectContext
	if pc == nil {
		pc, _ = project.Detect(opts.CWD)
	}
	if pc != nil && (pc.Type != "" || pc.HasBTPContext()) {
		// Enrich with latest known versions from packs before building mirror types
		if pc.CAPVersion != "" {
			for _, p := range opts.Packs {
				if v, ok := p.Versions["@sap/cds"]; ok {
					pc.LatestCAP = v
					break
				}
			}
			if pc.LatestCAP != "" {
				pc.RebuildFacts()
			}
		}
		info := &content.ProjectInfo{
			Type:       pc.Type,
			CAPVersion: pc.CAPVersion,
		}
		btpKeys := map[string]bool{"BTP subaccount": true, "Cloud Foundry": true}
		for _, f := range pc.Facts {
			pf := content.ProjectFact{Key: f.Key, Value: f.Value, Warn: f.Warn}
			if btpKeys[f.Key] {
				info.BTPFacts = append(info.BTPFacts, pf)
			} else {
				info.Facts = append(info.Facts, pf)
			}
		}
		d.Project = info
	}

	// Wired SAP MCP servers
	d.WiredMCPServers = detectWiredMCP(opts.Adapters, opts.Packs)

	return d
}

// detectWiredMCP reads each adapter's MCP config file and cross-references
// installed server IDs against known SAP MCP server IDs from loaded packs.
func detectWiredMCP(adapters []adapter.Adapter, packs []*content.Pack) []content.WiredMCPEntry {
	// Build set of known SAP MCP server IDs from packs.
	sapIDs := make(map[string]bool)
	for _, p := range packs {
		for _, srv := range p.MCPServers {
			sapIDs[srv.ID] = true
		}
	}
	if len(sapIDs) == 0 {
		return nil
	}

	var entries []content.WiredMCPEntry
	for _, a := range adapters {
		configs := a.AllMCPConfigs()
		if len(configs) == 0 {
			continue
		}
		var matched []string
		seen := make(map[string]bool)
		for _, cfg := range configs {
			path, err := adapter.ExpandHome(cfg.Path)
			if err != nil {
				continue
			}
			for _, id := range readMCPServerIDs(path, cfg.Key) {
				if sapIDs[id] && !seen[id] {
					matched = append(matched, id)
					seen[id] = true
				}
			}
		}
		if len(matched) > 0 {
			entries = append(entries, content.WiredMCPEntry{
				AdapterName: a.Name,
				ServerIDs:   matched,
			})
		}
	}
	return entries
}

// readMCPServerIDs reads the top-level keys of the object at root[key] from a JSON file.
// Returns nil on any error.
func readMCPServerIDs(path, key string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	raw, ok := root[key]
	if !ok {
		return nil
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(raw, &servers); err != nil {
		return nil
	}
	ids := make([]string, 0, len(servers))
	for id := range servers {
		ids = append(ids, id)
	}
	return ids
}
