package content

import (
	"fmt"
	"strings"
	"time"
)

// RenderContext builds the Markdown string injected into AI tool configuration.
// Packs are rendered in the order provided (caller applies profile weights first).
// dynamic may be nil; when non-nil a runtime context section is prepended.
func RenderContext(packs []*Pack, profile *Profile, dynamic *DynamicContext) string {
	var b strings.Builder

	b.WriteString("# SAP Developer Context\n\n")
	b.WriteString("This context is maintained by sap-devs and provides up-to-date SAP developer knowledge.\n\n")

	if profile != nil {
		b.WriteString(fmt.Sprintf("**Developer Profile:** %s — %s\n\n", profile.Name, profile.Description))
	}

	if dynamic != nil {
		b.WriteString(renderDynamic(dynamic))
		// renderDynamic ends with \n; add one more for blank line before pack content
		b.WriteString("\n")
	}

	for _, p := range packs {
		if strings.TrimSpace(p.ContextMD) == "" {
			continue
		}
		b.WriteString(strings.TrimSpace(p.ContextMD))
		b.WriteString("\n\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// renderDynamic produces the ## sap-devs Runtime Context markdown section.
func renderDynamic(d *DynamicContext) string {
	var b strings.Builder
	b.WriteString("## sap-devs Runtime Context\n\n")

	// Status line: CLI version, profile, packs
	var statusParts []string
	if d.CLIVersion != "" {
		statusParts = append(statusParts, fmt.Sprintf("**CLI:** sap-devs v%s", d.CLIVersion))
	}
	if d.ActiveProfile != "" {
		statusParts = append(statusParts, fmt.Sprintf("**Profile:** %s", d.ActiveProfile))
	}
	if len(d.LoadedPackIDs) > 0 {
		statusParts = append(statusParts, fmt.Sprintf("**Packs:** %s", strings.Join(d.LoadedPackIDs, ", ")))
	}
	if len(statusParts) > 0 {
		b.WriteString(strings.Join(statusParts, " | "))
		b.WriteString("\n")
	}

	// Last synced
	if d.LastSynced != nil {
		ago := time.Since(*d.LastSynced).Truncate(time.Minute)
		b.WriteString(fmt.Sprintf("**Last synced:** %s (%s ago)\n",
			d.LastSynced.Format("2006-01-02 15:04"), ago))
	} else {
		b.WriteString("**Last synced:** never — run `sap-devs sync`\n")
	}

	// Project type (omit if empty)
	if d.ProjectType != "" {
		b.WriteString(fmt.Sprintf("**Project type:** %s\n", d.ProjectType))
	}

	// Wired MCP servers (omit if none)
	for _, entry := range d.WiredMCPServers {
		if len(entry.ServerIDs) > 0 {
			b.WriteString(fmt.Sprintf("**Wired SAP MCP servers (%s):** %s\n",
				entry.AdapterName, strings.Join(entry.ServerIDs, ", ")))
		}
	}

	// Commands
	if len(d.Commands) > 0 {
		b.WriteString("\n**Available commands:**\n")
		for _, c := range d.Commands {
			b.WriteString(fmt.Sprintf("- `%s` — %s\n", c.Name, c.Short))
		}
	}

	b.WriteString("\nRun `sap-devs inject` to refresh this context · `sap-devs sync --force` to update content\n")
	return b.String()
}

// TrimPacks filters packs to fit within maxBytes, applying overlap deduplication
// and pack-level budget enforcement. Pass maxBytes=0 for unconstrained.
// Packs must already be sorted by weight descending (LoadPacks guarantees this).
func TrimPacks(packs []*Pack, maxBytes int) []*Pack {
	// Pass 1 — deduplication
	// A pack is dropped if a higher-weight pack it overlaps with is already included.
	included := make(map[string]bool)
	var deduped []*Pack
	for _, p := range packs {
		dominated := false
		for _, overlapID := range p.Overlaps {
			if included[overlapID] {
				dominated = true
				break
			}
		}
		if !dominated {
			deduped = append(deduped, p)
			included[p.ID] = true
		}
	}

	// Pass 2 — budget enforcement
	if maxBytes <= 0 {
		return deduped
	}
	var result []*Pack
	used := 0
	for _, p := range deduped {
		size := len(p.ContextMD)
		if used+size > maxBytes {
			break
		}
		result = append(result, p)
		used += size
	}
	return result
}
