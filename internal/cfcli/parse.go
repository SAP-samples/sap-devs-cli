package cfcli

import "strings"

func parseCFTarget(output string) TargetInfo {
	var info TargetInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "org:"):
			info.Org = strings.TrimSpace(strings.TrimPrefix(line, "org:"))
		case strings.HasPrefix(line, "space:"):
			info.Space = strings.TrimSpace(strings.TrimPrefix(line, "space:"))
		case strings.HasPrefix(line, "API endpoint:"):
			info.API = strings.TrimSpace(strings.TrimPrefix(line, "API endpoint:"))
		}
	}
	if m := reCFRegion.FindStringSubmatch(info.API); len(m) >= 2 {
		info.Region = m[1]
	}
	info.LoggedIn = info.Org != ""
	return info
}

func findColumns(headerLine string, names []string) []int {
	lower := strings.ToLower(headerLine)
	positions := make([]int, len(names))
	for i, name := range names {
		positions[i] = strings.Index(lower, strings.ToLower(name))
	}
	return positions
}

func extractField(line string, start, end int) string {
	if start < 0 || start >= len(line) {
		return ""
	}
	if end < 0 || end > len(line) {
		end = len(line)
	}
	return strings.TrimSpace(line[start:end])
}
