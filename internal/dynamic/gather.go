// internal/dynamic/gather.go
package dynamic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
)

// GatherOpts holds all inputs needed to collect dynamic context at inject time.
type GatherOpts struct {
	CWD          string
	CLIVersion   string
	Profile      *content.Profile
	Packs        []*content.Pack
	SyncStateDir string
	Adapters     []adapter.Adapter
	Commands     []content.CommandInfo
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

	// Project type
	d.ProjectType = detectProjectType(opts.CWD)

	// Wired SAP MCP servers
	d.WiredMCPServers = detectWiredMCP(opts.Adapters, opts.Packs)

	return d
}

// detectProjectType checks CWD for well-known SAP project indicators.
// Returns the first match; returns "" if nothing is detected.
func detectProjectType(cwd string) string {
	if cwd == "" {
		return ""
	}

	// .cdsrc.json — definitive CAP Node.js marker
	if fileExists(filepath.Join(cwd, ".cdsrc.json")) {
		return "CAP (Node.js)"
	}

	// package.json — check for @sap/cds before falling through to plain Node.js
	pkgPath := filepath.Join(cwd, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		if hasSAPCDS(data) {
			return "CAP (Node.js)"
		}
	}

	// pom.xml — CAP Java
	if data, err := os.ReadFile(filepath.Join(cwd, "pom.xml")); err == nil {
		if strings.Contains(string(data), "com.sap.cds") {
			return "CAP (Java)"
		}
	}

	// mta.yaml — Multi-target Application
	if fileExists(filepath.Join(cwd, "mta.yaml")) {
		return "Multi-target Application (MTA)"
	}

	// xs-app.json — Fiori / BAS
	if fileExists(filepath.Join(cwd, "xs-app.json")) {
		return "Fiori / BAS app"
	}

	// Plain package.json — generic Node.js
	if fileExists(pkgPath) {
		return "Node.js"
	}

	return ""
}

// hasSAPCDS reports whether the package.json data contains @sap/cds in any dependency map.
func hasSAPCDS(data []byte) bool {
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	if _, ok := pkg.Dependencies["@sap/cds"]; ok {
		return true
	}
	if _, ok := pkg.DevDependencies["@sap/cds"]; ok {
		return true
	}
	return false
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
		if a.MCPConfig == nil || a.MCPConfig.Path == "" {
			continue
		}
		path, err := adapter.ExpandHome(a.MCPConfig.Path)
		if err != nil {
			continue
		}
		installed := readMCPServerIDs(path, a.MCPConfig.Key)
		var matched []string
		for _, id := range installed {
			if sapIDs[id] {
				matched = append(matched, id)
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
