package content

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	reCodeBlock   = regexp.MustCompile("(?m)^```[^\n]*\n((?:[^`]|`[^`]|``[^`])*?)^```")
	reATXHeader   = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	reBold        = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reItalic      = regexp.MustCompile(`\*([^*\n]+)\*`)
	reInlineCode  = regexp.MustCompile("`([^`\n]+)`")
	reHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reBlankLines  = regexp.MustCompile(`\n{3,}`)
)

func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

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

	if dynamic != nil && len(dynamic.ScratchNotes) > 0 {
		b.WriteString("## Current Context\n\n")
		for _, note := range dynamic.ScratchNotes {
			sanitized := strings.ReplaceAll(note, "\r\n", " ")
			sanitized = strings.ReplaceAll(sanitized, "\r", " ")
			sanitized = strings.ReplaceAll(sanitized, "\n", " ")
			if len(sanitized) > 500 {
				sanitized = TrimToBytes(sanitized, 500) + "..."
			}
			b.WriteString("- " + sanitized + "\n")
		}
		b.WriteString("\n")
	}

	if dynamic != nil {
		b.WriteString(renderDynamic(dynamic))
		// renderDynamic ends with \n; add one more for blank line before pack content
		b.WriteString("\n")
	}

	// Render preamble from base packs (before all ContextMD)
	for _, p := range packs {
		if p.Base && strings.TrimSpace(p.PreambleMD) != "" {
			b.WriteString(strings.TrimSpace(p.PreambleMD))
			b.WriteString("\n\n")
		}
	}

	var constraints []string
	for _, p := range packs {
		if trimmed := strings.TrimSpace(p.ConstraintsMD); trimmed != "" {
			constraints = append(constraints, trimmed)
		}
	}
	if len(constraints) > 0 {
		b.WriteString("## Constraints\n\n")
		b.WriteString(strings.Join(constraints, "\n\n"))
		b.WriteString("\n\n")
	}

	for _, p := range packs {
		if strings.TrimSpace(p.ContextMD) == "" {
			continue
		}
		b.WriteString(strings.TrimSpace(p.ContextMD))
		b.WriteString("\n\n")
	}

	var injectable []Sample
	for _, p := range packs {
		for _, s := range p.Samples {
			if s.Inject {
				injectable = append(injectable, s)
			}
		}
	}
	if len(injectable) > 0 {
		b.WriteString("## Canonical Patterns\n\n")
		b.WriteString("These are authoritative code samples — prefer these patterns over generating from training data.\n\n")
		b.WriteString("| Pattern | Description | URL |\n")
		b.WriteString("|---------|-------------|-----|\n")
		for _, s := range injectable {
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", s.Label, s.Description, s.URL))
		}
		b.WriteString("\n")
	}

	var learningRows []string
	for _, p := range packs {
		for _, lj := range p.LearningForInject {
			learningRows = append(learningRows, fmt.Sprintf("| [%s](%s) | %s | %s |",
				lj.Title, lj.URL, lj.Level, lj.Duration))
		}
	}
	if len(learningRows) > 0 {
		b.WriteString("## Recommended Learning Journeys\n\n")
		b.WriteString("| Journey | Level | Duration |\n")
		b.WriteString("|---------|-------|----------|\n")
		for _, row := range learningRows {
			b.WriteString(row + "\n")
		}
		b.WriteString("\n")
	}

	var knownErrors []KnownError
	for _, p := range packs {
		knownErrors = append(knownErrors, p.KnownErrors...)
	}
	if len(knownErrors) > 0 {
		b.WriteString("## Known Errors\n\n")
		b.WriteString("| Error Pattern | Cause | Fix |\n")
		b.WriteString("|---|---|---|\n")
		for _, e := range knownErrors {
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
				escapePipe(e.Pattern), escapePipe(e.Cause), escapePipe(e.Fix)))
		}
		b.WriteString("\n")
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

	// Project context (omit if no project detected)
	if d.Project != nil && len(d.Project.Facts) > 0 {
		b.WriteString("\n**Project Context (detected):**\n")
		for _, f := range d.Project.Facts {
			b.WriteString(fmt.Sprintf("- %s: %s\n", f.Key, f.Value))
		}
		for _, f := range d.ProjectFindings {
			if f.Severity == "error" || f.Severity == "warning" {
				b.WriteString(fmt.Sprintf("- ⚠ %s\n", f.Message))
			}
		}
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
// Base packs (Pack.Base == true) are exempt from both trimming passes and always
// appear first in the returned slice.
func TrimPacks(packs []*Pack, maxBytes int) []*Pack {
	// Separate base packs — always included, never trimmed, always first.
	var base, nonBase []*Pack
	for _, p := range packs {
		if p.Base {
			base = append(base, p)
		} else {
			nonBase = append(nonBase, p)
		}
	}
	return append(base, trimNonBase(nonBase, maxBytes)...)
}

// trimNonBase applies deduplication and byte-budget enforcement to non-base packs.
func trimNonBase(packs []*Pack, maxBytes int) []*Pack {
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
		size := len(p.ContextMD) + len(p.ConstraintsMD)
		if used+size > maxBytes {
			break
		}
		result = append(result, p)
		used += size
	}
	return result
}

// FormatOutput converts text to the target format.
// format == "markdown" (or empty): returns text unchanged.
// format == "plain-prose": strips Markdown syntax for plain-text UI fields.
func FormatOutput(text, format string) string {
	if format != "plain-prose" {
		return text
	}
	s := text

	// Strip fenced code blocks — keep body, remove fences.
	// Pattern anchors both ``` fences to line starts to avoid merging adjacent blocks.
	s = reCodeBlock.ReplaceAllString(s, "$1")

	// Strip ATX headers (# through ######)
	s = reATXHeader.ReplaceAllString(s, "")

	// Strip bold (**text**)
	s = reBold.ReplaceAllString(s, "$1")

	// Strip italic (*text*)
	s = reItalic.ReplaceAllString(s, "$1")

	// Strip inline code (`text`)
	s = reInlineCode.ReplaceAllString(s, "$1")

	// Strip HTML comments
	s = reHTMLComment.ReplaceAllString(s, "")

	// Normalize 3+ consecutive blank lines to 2
	s = reBlankLines.ReplaceAllString(s, "\n\n")

	return s
}

// TrimToBytes truncates s to at most maxBytes bytes, cutting at the last
// complete UTF-8 rune boundary. Returns s unchanged if maxBytes <= 0 or
// len(s) <= maxBytes.
func TrimToBytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	i := maxBytes
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return s[:i]
}
