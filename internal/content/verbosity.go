package content

import (
	"fmt"
	"os"
	"regexp"
)

type VerbositySections struct {
	Core     string
	Detail   string
	Extended string
}

var reVerbosityMarker = regexp.MustCompile(`(?m)^<!--\s*verbosity:(\w+)\s*-->\n?`)

func ParseVerbositySections(md string) VerbositySections {
	var v VerbositySections
	currentLevel := "core"
	lastEnd := 0

	for _, loc := range reVerbosityMarker.FindAllStringSubmatchIndex(md, -1) {
		chunk := md[lastEnd:loc[0]]
		appendChunk(&v, currentLevel, chunk)

		level := md[loc[2]:loc[3]]
		switch level {
		case "core", "detail", "extended":
			currentLevel = level
		default:
			fmt.Fprintf(os.Stderr, "sap-devs: unknown verbosity level %q, treating as core\n", level)
			currentLevel = "core"
		}
		lastEnd = loc[1]
	}

	if lastEnd < len(md) {
		appendChunk(&v, currentLevel, md[lastEnd:])
	}
	return v
}

func appendChunk(v *VerbositySections, level, chunk string) {
	switch level {
	case "core":
		v.Core += chunk
	case "detail":
		v.Detail += chunk
	case "extended":
		v.Extended += chunk
	}
}

func (v VerbositySections) AtLevel(level string) string {
	switch level {
	case "minimal":
		return v.Core
	case "standard":
		return v.Core + v.Detail
	default:
		return v.Core + v.Detail + v.Extended
	}
}

func mergeVerbositySections(first, second VerbositySections) VerbositySections {
	return VerbositySections{
		Core:     joinNonEmpty(first.Core, second.Core),
		Detail:   joinNonEmpty(first.Detail, second.Detail),
		Extended: joinNonEmpty(first.Extended, second.Extended),
	}
}

func joinNonEmpty(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "\n\n" + b
}
