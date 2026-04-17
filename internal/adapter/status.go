package adapter

import (
	"regexp"
	"strings"
)

// SectionInfo describes a non-sap-devs fenced block found in a target file.
type SectionInfo struct {
	Name   string `json:"name"`
	Tokens int    `json:"tokens"`
}

// StatusRow is the result of inspecting one adapter target (one row per adapter+target pair).
// An adapter with both a global and a project target produces two StatusRows.
type StatusRow struct {
	AdapterName string `json:"adapter_name"`
	AdapterID   string `json:"adapter"`
	Scope       string `json:"scope"`
	TargetPath  string `json:"path"` // unexpanded (~-form)

	FileExists bool `json:"file_exists"`
	Injected   bool `json:"injected"`  // sap-devs section present and well-formed
	Orphaned   bool `json:"orphaned"`  // markers found but mismatched/reversed

	// Stale is true when the on-disk section content differs from what inject would write today.
	// Always false when FileExists=false, Injected=false, or engine has no packs loaded.
	Stale bool `json:"stale"`

	// Stretch-goal fields — always populated when FileExists=true.
	FileSizeBytes int           `json:"file_size_bytes"`
	FileTokenEst  int           `json:"file_token_est"`  // word count × 1.3
	SapDevsTokens int           `json:"sap_devs_tokens"` // token estimate for sap-devs section only
	OtherSections []SectionInfo `json:"other_sections"`  // non-sap-devs fenced blocks
}

// reOtherSection matches <!-- <prefix>:start:<name> --> where prefix != "sap-devs".
var reOtherSection = regexp.MustCompile(`<!-- ([^:>]+):start:([^>]+) -->`)

// EstimateTokens returns a rough token estimate: word count × 1.3.
// Exported for testing.
func EstimateTokens(s string) int {
	words := len(strings.Fields(s))
	return words * 13 / 10
}

// ScanOtherSections finds non-sap-devs HTML-comment fenced blocks in content.
// Returns []SectionInfo{} (never nil) so it marshals as [] in JSON.
func ScanOtherSections(content string) []SectionInfo {
	result := []SectionInfo{}
	matches := reOtherSection.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		prefix := content[m[2]:m[3]]
		if prefix == "sap-devs" {
			continue
		}
		// Find matching end marker
		endMarker := "<!-- " + prefix + ":end:"
		startPos := m[1] // position after the start marker
		endPos := strings.Index(content[startPos:], endMarker)
		var tokens int
		if endPos >= 0 {
			inner := content[startPos : startPos+endPos]
			tokens = EstimateTokens(inner)
		}
		result = append(result, SectionInfo{Name: prefix, Tokens: tokens})
	}
	return result
}
