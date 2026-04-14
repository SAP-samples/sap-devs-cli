package content

import (
	"regexp"
	"strconv"
	"strings"
)

// compareVersions compares two version strings of exactly three dot-separated
// integer segments and returns -1, 0, or 1. Each segment has any trailing
// non-digit characters stripped before parsing. Both inputs must already be
// zero-padded to three components by the caller.
func compareVersions(a, b string) int {
	aParts := strings.SplitN(a, ".", 3)
	bParts := strings.SplitN(b, ".", 3)
	for i := 0; i < 3; i++ {
		av := parseSegment(aParts[i])
		bv := parseSegment(bParts[i])
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

// parseSegment strips trailing non-digit characters from a version segment
// and returns its integer value, or 0 if unparseable.
func parseSegment(s string) int {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}

// padVersion zero-pads a version string to exactly three dot-separated components.
func padVersion(v string) string {
	parts := strings.SplitN(v, ".", 3) // 3-capped: pre-release on last segment stays intact
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	return strings.Join(parts, ".")
}

// parseConstraint parses a required string of the form ">=1.2.3", ">1.2.3",
// "=1.2.3", "<=1.2.3", or "<1.2.3" and compares it against found.
// Both version strings have a leading "v" stripped and are zero-padded to
// three components before comparison. Returns false if the operator is not
// recognised or either version cannot be usefully parsed.
func parseConstraint(required, found string) bool {
	var op, reqVer string
	switch {
	case strings.HasPrefix(required, ">="):
		op, reqVer = ">=", required[2:]
	case strings.HasPrefix(required, ">"):
		op, reqVer = ">", required[1:]
	case strings.HasPrefix(required, "<="):
		op, reqVer = "<=", required[2:]
	case strings.HasPrefix(required, "<"):
		op, reqVer = "<", required[1:]
	case strings.HasPrefix(required, "="):
		op, reqVer = "=", required[1:]
	default:
		return false
	}

	reqVer = strings.TrimPrefix(strings.TrimSpace(reqVer), "v")
	if len(reqVer) == 0 || reqVer[0] < '0' || reqVer[0] > '9' {
		return false
	}
	reqVer = padVersion(reqVer)
	foundNorm := strings.TrimPrefix(strings.TrimSpace(found), "v")

	// Guard: if found doesn't start with a digit it cannot be parsed — return false.
	if len(foundNorm) == 0 || foundNorm[0] < '0' || foundNorm[0] > '9' {
		return false
	}
	foundVer := padVersion(foundNorm)

	cmp := compareVersions(foundVer, reqVer)
	switch op {
	case ">=":
		return cmp >= 0
	case ">":
		return cmp > 0
	case "<=":
		return cmp <= 0
	case "<":
		return cmp < 0
	case "=":
		return cmp == 0
	}
	return false
}

// ParseConstraintForTest exposes parseConstraint for use in external test packages.
func ParseConstraintForTest(required, found string) bool {
	return parseConstraint(required, found)
}

// CheckStatus represents the result of a tool version check.
type CheckStatus string

const (
	StatusOK      CheckStatus = "ok"
	StatusFail    CheckStatus = "fail"
	StatusMissing CheckStatus = "missing"
	StatusUnknown CheckStatus = "unknown" // required is "latest"
)

// ToolResult holds the outcome of checking a single tool.
type ToolResult struct {
	Tool   ToolDef
	Status CheckStatus
	Found  string // raw captured version string, empty if missing
}

// Runner abstracts exec.Command for testability.
// It receives the full command string (e.g. "node --version") and returns
// the combined stdout+stderr output.
type Runner func(command string) (string, error)

// CheckTool runs the tool's detect command via run, extracts the version using
// the tool's regex pattern, and compares it against tool.Required.
func CheckTool(tool ToolDef, run Runner) ToolResult {
	output, err := run(tool.Detect.Command)
	if err != nil {
		return ToolResult{Tool: tool, Status: StatusMissing}
	}

	re, err := regexp.Compile(tool.Detect.Pattern)
	if err != nil {
		return ToolResult{Tool: tool, Status: StatusMissing}
	}
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return ToolResult{Tool: tool, Status: StatusMissing}
	}
	found := matches[1]

	if tool.Required == "latest" {
		return ToolResult{Tool: tool, Status: StatusUnknown, Found: found}
	}

	if parseConstraint(tool.Required, found) {
		return ToolResult{Tool: tool, Status: StatusOK, Found: found}
	}
	return ToolResult{Tool: tool, Status: StatusFail, Found: found}
}

// CheckTools runs CheckTool for each tool, deduplicating by ID (first seen wins).
func CheckTools(tools []ToolDef, run Runner) []ToolResult {
	seen := make(map[string]bool)
	var results []ToolResult
	for _, t := range tools {
		if seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		results = append(results, CheckTool(t, run))
	}
	return results
}
