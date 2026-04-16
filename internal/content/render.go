package content

import (
	"fmt"
	"strings"
)

// RenderContext builds the Markdown string injected into AI tool configuration.
// Packs are rendered in the order provided (caller applies profile weights first).
func RenderContext(packs []*Pack, profile *Profile) string {
	var b strings.Builder

	b.WriteString("# SAP Developer Context\n\n")
	b.WriteString("This context is maintained by sap-devs and provides up-to-date SAP developer knowledge.\n\n")

	if profile != nil {
		b.WriteString(fmt.Sprintf("**Developer Profile:** %s — %s\n\n", profile.Name, profile.Description))
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
