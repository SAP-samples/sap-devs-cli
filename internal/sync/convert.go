package sync

import (
	"fmt"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

// convertContent applies format post-processing to a fetched response body.
// selector scopes HTML extraction to a matching DOM element before conversion;
// it is silently ignored when format is "raw".
// Returns: processed content, non-fatal warnings (selector miss, invalid selector),
// and any fatal conversion error.
func convertContent(body, format, selector string) (string, []string, error) {
	if format == "raw" {
		return body, nil, nil
	}

	var warns []string

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		// golang.org/x/net/html is very lenient; this path is unlikely in practice.
		warns = append(warns, fmt.Sprintf("html parse error: %v — using raw body", err))
		return body, warns, nil
	}

	root := doc

	if selector != "" {
		sel, compileErr := cascadia.Compile(selector)
		if compileErr != nil {
			warns = append(warns, fmt.Sprintf("invalid selector %q: %v — using full body", selector, compileErr))
		} else {
			match := cascadia.Query(doc, sel)
			if match == nil {
				warns = append(warns, fmt.Sprintf("selector %q matched no elements — using full body", selector))
			} else {
				root = match
			}
		}
	}

	switch format {
	case "text":
		return extractText(root), warns, nil

	case "markdown":
		var buf strings.Builder
		if err := html.Render(&buf, root); err != nil {
			return "", warns, fmt.Errorf("render selected node: %w", err)
		}
		md, err := htmltomarkdown.ConvertString(buf.String())
		if err != nil {
			return "", warns, fmt.Errorf("html-to-markdown conversion: %w", err)
		}
		return md, warns, nil

	default:
		// Unknown format — ScanMarkers already warned at parse time.
		// Fall back to markdown so the content is still useful.
		var buf strings.Builder
		if err := html.Render(&buf, root); err != nil {
			return "", warns, fmt.Errorf("render selected node: %w", err)
		}
		md, err := htmltomarkdown.ConvertString(buf.String())
		if err != nil {
			return "", warns, fmt.Errorf("html-to-markdown conversion: %w", err)
		}
		return md, warns, nil
	}
}

// extractText recursively walks an HTML node tree and returns all text node content.
// Note: no whitespace is inserted between block elements — adjacent text from
// <p>First</p><p>Second</p> will be concatenated as "FirstSecond".
func extractText(n *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return buf.String()
}
