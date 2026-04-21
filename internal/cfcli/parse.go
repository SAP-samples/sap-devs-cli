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

type App struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	Instances string `json:"instances"`
	Routes    string `json:"routes"`
}

func parseCFApps(output string) []App {
	lines := strings.Split(output, "\n")
	headerIdx := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "name") && strings.Contains(lower, "requested state") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil
	}

	cols := findColumns(lines[headerIdx], []string{"name", "requested state", "processes", "routes"})
	var apps []App
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		apps = append(apps, App{
			Name:      extractField(line, cols[0], cols[1]),
			State:     extractField(line, cols[1], cols[2]),
			Instances: extractField(line, cols[2], cols[3]),
			Routes:    extractField(line, cols[3], -1),
		})
	}
	return apps
}

type Service struct {
	Name      string `json:"name"`
	Service   string `json:"service"`
	Plan      string `json:"plan"`
	BoundApps string `json:"bound_apps"`
	Status    string `json:"status"`
}

func parseCFServices(output string) []Service {
	lines := strings.Split(output, "\n")
	headerIdx := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "name") && (strings.Contains(lower, "offering") || strings.Contains(lower, "service")) && strings.Contains(lower, "plan") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil
	}

	header := lines[headerIdx]
	lowerHeader := strings.ToLower(header)

	nameCol := strings.Index(lowerHeader, "name")
	offeringCol := -1
	if idx := strings.Index(lowerHeader, "offering"); idx >= 0 {
		offeringCol = idx
	} else {
		offeringCol = strings.Index(lowerHeader, "service")
	}
	planCol := strings.Index(lowerHeader, "plan")
	boundCol := strings.Index(lowerHeader, "bound apps")
	lastOpCol := strings.Index(lowerHeader, "last operation")

	var services []Service
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		services = append(services, Service{
			Name:      extractField(line, nameCol, offeringCol),
			Service:   extractField(line, offeringCol, planCol),
			Plan:      extractField(line, planCol, boundCol),
			BoundApps: extractField(line, boundCol, lastOpCol),
			Status:    extractField(line, lastOpCol, -1),
		})
	}
	return services
}

type Route struct {
	Domain string `json:"domain"`
	Host   string `json:"host"`
	Path   string `json:"path"`
	Apps   string `json:"apps"`
}

func parseCFRoutes(output string) []Route {
	lines := strings.Split(output, "\n")
	headerIdx := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "space") && strings.Contains(lower, "host") && strings.Contains(lower, "domain") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil
	}

	header := lines[headerIdx]
	lowerHeader := strings.ToLower(header)

	hostCol := strings.Index(lowerHeader, "host")
	domainCol := strings.Index(lowerHeader, "domain")
	pathCol := strings.Index(lowerHeader, "path")
	appsCol := -1
	if idx := strings.Index(lowerHeader, "destination"); idx >= 0 {
		appsCol = idx
	} else if idx := strings.Index(lowerHeader, "apps"); idx >= 0 {
		appsCol = idx
	}

	var routes []Route
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		routes = append(routes, Route{
			Host:   extractField(line, hostCol, domainCol),
			Domain: extractField(line, domainCol, pathCol),
			Path:   extractField(line, pathCol, appsCol),
			Apps:   extractField(line, appsCol, -1),
		})
	}
	return routes
}

type Domain struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

func parseCFDomains(output string) []Domain {
	lines := strings.Split(output, "\n")
	headerIdx := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "name") && (strings.Contains(lower, "type") || strings.Contains(lower, "status")) {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil
	}

	header := lines[headerIdx]
	lowerHeader := strings.ToLower(header)
	nameCol := strings.Index(lowerHeader, "name")
	typeCol := -1
	if idx := strings.Index(lowerHeader, "type"); idx >= 0 {
		typeCol = idx
	}
	statusCol := strings.Index(lowerHeader, "status")

	var domains []Domain
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		d := Domain{Name: extractField(line, nameCol, typeCol)}
		if typeCol >= 0 && statusCol >= 0 {
			d.Type = extractField(line, typeCol, statusCol)
			d.Status = extractField(line, statusCol, -1)
		} else if typeCol >= 0 {
			d.Type = extractField(line, typeCol, -1)
		}
		domains = append(domains, d)
	}
	return domains
}

type Buildpack struct {
	Name     string `json:"name"`
	Position string `json:"position"`
	Enabled  string `json:"enabled"`
	Locked   string `json:"locked"`
	Filename string `json:"filename"`
}

func parseCFBuildpacks(output string) []Buildpack {
	lines := strings.Split(output, "\n")
	headerIdx := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "position") && strings.Contains(lower, "name") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil
	}

	header := lines[headerIdx]
	lowerHeader := strings.ToLower(header)
	posCol := strings.Index(lowerHeader, "position")
	nameCol := strings.Index(lowerHeader, "name")
	enabledCol := strings.Index(lowerHeader, "enabled")
	lockedCol := strings.Index(lowerHeader, "locked")
	filenameCol := strings.Index(lowerHeader, "filename")

	var bps []Buildpack
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		bps = append(bps, Buildpack{
			Position: extractField(line, posCol, nameCol),
			Name:     extractField(line, nameCol, enabledCol),
			Enabled:  extractField(line, enabledCol, lockedCol),
			Locked:   extractField(line, lockedCol, filenameCol),
			Filename: extractField(line, filenameCol, -1),
		})
	}
	return bps
}
