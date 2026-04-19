package content

import (
	"fmt"
	"os"
	"regexp"
)

var RecognizedContextSections = []string{
	"Overview",
	"Key Concepts",
	"Best Practices",
	"Anti-patterns",
	"Code Examples",
}

var reH3Heading = regexp.MustCompile(`(?m)^###\s+(.+)$`)

func ValidateContextSections(packID string, content string) {
	matches := reH3Heading.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return
	}

	recognizedIndex := make(map[string]int, len(RecognizedContextSections))
	for i, s := range RecognizedContextSections {
		recognizedIndex[s] = i
	}

	lastRecognizedOrder := -1
	for _, m := range matches {
		heading := m[1]
		idx, recognized := recognizedIndex[heading]
		if !recognized {
			fmt.Fprintf(os.Stderr, "sap-devs: pack %q: unrecognized section %q\n", packID, heading)
			continue
		}
		if idx < lastRecognizedOrder {
			fmt.Fprintf(os.Stderr, "sap-devs: pack %q: section %q is out of order\n", packID, heading)
		} else {
			lastRecognizedOrder = idx
		}
	}
}
