// internal/content/dynamic.go
package content

import "time"

// DynamicContext holds runtime-gathered information injected before pack content.
// All fields are optional; zero values mean "not available".
type DynamicContext struct {
	CLIVersion      string
	ActiveProfile   string // profile.Name, or profile.ID, or ""
	LoadedPackIDs   []string
	LastSynced      *time.Time
	ProjectType     string
	WiredMCPServers []WiredMCPEntry
	Commands        []CommandInfo
}

// WiredMCPEntry records SAP MCP servers registered in a specific AI tool's config.
type WiredMCPEntry struct {
	AdapterName string
	ServerIDs   []string
}

// CommandInfo describes a single CLI command for injection into AI context.
type CommandInfo struct {
	Name  string
	Short string
}
