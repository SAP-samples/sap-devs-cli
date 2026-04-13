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
