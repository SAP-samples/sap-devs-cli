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
	Project         *ProjectInfo
	ProjectFindings []ProjectFinding
	WiredMCPServers []WiredMCPEntry
	Commands        []CommandInfo
}

// ProjectInfo holds detected facts about the current project, mirroring
// project.ProjectContext fields needed for rendering. Populated by
// internal/dynamic at gather time — kept in content to avoid an import cycle
// (internal/project imports internal/content for pack access).
type ProjectInfo struct {
	Type       string
	CAPVersion string
	Facts      []ProjectFact
}

// ProjectFact is a single key/value pair detected from the project.
type ProjectFact struct {
	Key   string
	Value string
	Warn  string
}

// ProjectFinding is a single health-check result surfaced during inject.
type ProjectFinding struct {
	Severity string // "error", "warning", "info"
	Message  string
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
