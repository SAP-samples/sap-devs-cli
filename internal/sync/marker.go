package sync

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Marker represents a parsed <!-- sync:fetch ... --> directive.
type Marker struct {
	PackID    string
	Index     int    // zero-based position in the file
	URL       string
	MaxLines  int    // 0 = no limit
	MaxTokens int    // 0 = no limit; MaxLines takes precedence when both set
	Label     string
	TTLHours  int    // 0 = use pack/engine default
	LineNum   int
	Format    string // "raw" | "text" | "markdown"; empty string treated as "markdown" by FetchMarker
	Selector  string // CSS selector for DOM scoping; empty = whole body; ignored for "raw"
}

var markerRE = regexp.MustCompile(`<!--\s*sync:fetch\s+(.*?)\s*-->`)
var attrRE = regexp.MustCompile(`(\w+)="([^"]*)"`)

// ScanMarkers parses content for sync:fetch markers.
// Markers inside fenced code blocks (``` delimiters) are skipped.
// Returns parsed markers and any parse warnings (not errors — sync continues regardless).
func ScanMarkers(packID, content string) ([]Marker, []string) {
	var markers []Marker
	var warnings []string

	lines := strings.Split(content, "\n")
	inFence := false
	index := 0

	for lineNum, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		match := markerRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		attrs := parseAttrs(match[1])
		url := attrs["url"]
		if url == "" {
			warnings = append(warnings, fmt.Sprintf(
				"%s: line %d: sync:fetch missing required 'url' attribute", packID, lineNum+1,
			))
			continue
		}
		m := Marker{
			PackID:  packID,
			Index:   index,
			URL:     url,
			Label:   attrs["label"],
			LineNum: lineNum + 1,
		}
		if v := attrs["max_lines"]; v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf(
					"%s: line %d: max_lines %q is not a valid integer, ignoring", packID, lineNum+1, v,
				))
			} else {
				m.MaxLines = n
			}
		}
		if v := attrs["max_tokens"]; v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf(
					"%s: line %d: max_tokens %q is not a valid integer, ignoring", packID, lineNum+1, v,
				))
			} else {
				m.MaxTokens = n
			}
		}
		if m.MaxLines > 0 && m.MaxTokens > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"%s: line %d: both max_lines and max_tokens set; max_lines takes precedence", packID, lineNum+1,
			))
		}
		if v := attrs["ttl_hours"]; v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf(
					"%s: line %d: ttl_hours %q is not a valid integer, ignoring", packID, lineNum+1, v,
				))
			} else {
				m.TTLHours = n
			}
		}
		m.Format = "markdown" // default
		if v := attrs["format"]; v != "" {
			switch v {
			case "raw", "text", "markdown":
				m.Format = v
			default:
				warnings = append(warnings, fmt.Sprintf(
					"%s: line %d: unknown format %q — defaulting to markdown", packID, lineNum+1, v,
				))
			}
		}
		if v := attrs["selector"]; v != "" {
			m.Selector = v
		}
		markers = append(markers, m)
		index++
	}
	return markers, warnings
}

// ExpandMarkers substitutes sync:fetch marker lines in content with fetched results.
// results maps marker.Index → replacement string. Markers with no result entry are left unchanged.
func ExpandMarkers(content string, markers []Marker, results map[int]string) string {
	if len(markers) == 0 {
		return content
	}
	// Build a line-number → marker index map for O(1) lookup.
	lineToMarker := make(map[int]int, len(markers))
	for _, m := range markers {
		lineToMarker[m.LineNum-1] = m.Index // LineNum is 1-based
	}

	lines := strings.Split(content, "\n")
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if idx, ok := lineToMarker[i]; ok {
			if fetched, hasResult := results[idx]; hasResult {
				lines[i] = fetched
			}
		}
	}
	return strings.Join(lines, "\n")
}

func parseAttrs(s string) map[string]string {
	attrs := make(map[string]string)
	for _, m := range attrRE.FindAllStringSubmatch(s, -1) {
		attrs[m[1]] = m[2]
	}
	return attrs
}

// FetchMarker fetches m.URL and returns the content, truncated per m.MaxLines / m.MaxTokens.
// client may be nil; a default 10-second timeout client is used in that case.
func FetchMarker(m Marker, client *http.Client) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Get(m.URL) //nolint:gosec // URL comes from pack author, not untrusted input
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, m.URL)
	}
	const maxBodyBytes = 1 << 20 // 1 MiB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", err
	}
	format := m.Format
	if format == "" {
		format = "markdown"
	}

	content, warns, err := convertContent(string(body), format, m.Selector)
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "WARN  sync:fetch %s\n", w)
	}
	if err != nil {
		return "", err
	}

	if m.MaxLines > 0 {
		content = truncateLines(content, m.MaxLines)
	} else if m.MaxTokens > 0 {
		content = truncateTokens(content, m.MaxTokens)
	}
	return content, nil
}

func truncateLines(s string, max int) string {
	lines := strings.SplitN(s, "\n", max+1)
	if len(lines) > max {
		lines = lines[:max]
	}
	return strings.Join(lines, "\n")
}

func truncateTokens(s string, max int) string {
	// Rough approximation: 1 token ≈ 4 characters.
	limit := max * 4
	if len(s) <= limit {
		return s
	}
	s = s[:limit]
	if idx := strings.LastIndex(s, "\n"); idx >= 0 {
		s = s[:idx]
	}
	return s
}
