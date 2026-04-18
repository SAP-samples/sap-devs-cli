package tutorials

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type frontmatter struct {
	Parser     string   `yaml:"parser"`
	Title      string   `yaml:"title"`
	Desc       string   `yaml:"description"`
	Time       any      `yaml:"time"`
	Tags       []string `yaml:"tags"`
	PrimaryTag string   `yaml:"primary_tag"`
	AuthorName string   `yaml:"author_name"`
}

// Parse parses a full tutorial markdown into a Tutorial struct.
func Parse(md, slug, repo string) (*Tutorial, error) {
	fm, body, err := splitFrontmatter(md)
	if err != nil {
		return nil, err
	}

	meta := buildMeta(fm, body, slug, repo)
	tut := &Tutorial{TutorialMeta: meta}

	tut.Prerequisites = extractSection(body, "Prerequisites")
	tut.YouWillLearn = extractBulletList(body, "You will learn")

	if fm.Parser == "v2" {
		tut.Steps = parseV2Steps(body)
	} else {
		tut.Steps = parseV1Steps(body)
	}

	return tut, nil
}

// ParseFrontmatterOnly extracts metadata without parsing steps.
func ParseFrontmatterOnly(md, slug, repo string) (*TutorialMeta, error) {
	fm, body, err := splitFrontmatter(md)
	if err != nil {
		return nil, err
	}
	meta := buildMeta(fm, body, slug, repo)
	return &meta, nil
}

// ExtractLevel derives the experience level from tutorial tags.
func ExtractLevel(tags []string) string {
	for _, t := range tags {
		lower := strings.ToLower(t)
		if strings.HasPrefix(lower, "tutorial>") {
			level := strings.TrimPrefix(lower, "tutorial>")
			switch level {
			case "beginner", "intermediate", "advanced":
				return level
			}
		}
	}
	return ""
}

func splitFrontmatter(md string) (*frontmatter, string, error) {
	if !strings.HasPrefix(strings.TrimSpace(md), "---") {
		return &frontmatter{}, md, nil
	}
	trimmed := strings.TrimSpace(md)
	parts := strings.SplitN(trimmed[3:], "---", 2)
	if len(parts) < 2 {
		return &frontmatter{}, md, nil
	}
	var fm frontmatter
	if err := yaml.Unmarshal([]byte(parts[0]), &fm); err != nil {
		return nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	return &fm, strings.TrimSpace(parts[1]), nil
}

func buildMeta(fm *frontmatter, body, slug, repo string) TutorialMeta {
	meta := TutorialMeta{
		Slug:       slug,
		Tags:       fm.Tags,
		PrimaryTag: fm.PrimaryTag,
		Author:     fm.AuthorName,
		Repo:       repo,
		URL:        fmt.Sprintf("https://developers.sap.com/tutorials/%s.html", slug),
		Parser:     fm.Parser,
		Level:      ExtractLevel(fm.Tags),
	}

	switch v := fm.Time.(type) {
	case int:
		meta.Time = v
	case float64:
		meta.Time = int(v)
	case string:
		meta.Time, _ = strconv.Atoi(v)
	}

	if fm.Title != "" {
		meta.Title = fm.Title
	} else {
		meta.Title = extractH1(body)
		if meta.Title == "" {
			meta.Title = slug
		}
	}

	if fm.Desc != "" {
		meta.Description = fm.Desc
	} else {
		meta.Description = extractDescriptionComment(body)
	}

	return meta
}

func extractH1(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

var descCommentRE = regexp.MustCompile(`<!--\s*description\s*-->\s*(.+)`)

func extractDescriptionComment(body string) string {
	m := descCommentRE.FindStringSubmatch(body)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractSection(body, heading string) string {
	marker := "## " + heading
	idx := strings.Index(body, marker)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(marker):]
	rest = strings.TrimLeft(rest, " \t\r\n")
	end := strings.Index(rest, "\n## ")
	if end < 0 {
		end = strings.Index(rest, "\n### ")
	}
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

func extractBulletList(body, heading string) []string {
	section := extractSection(body, heading)
	if section == "" {
		return nil
	}
	var items []string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			items = append(items, strings.TrimPrefix(line, "- "))
		} else if strings.HasPrefix(line, "* ") {
			items = append(items, strings.TrimPrefix(line, "* "))
		}
	}
	return items
}

func parseV2Steps(body string) []TutorialStep {
	var steps []TutorialStep
	parts := strings.Split("\n"+body, "\n### ")
	for i, part := range parts {
		if i == 0 {
			continue
		}
		lines := strings.SplitN(part, "\n", 2)
		title := strings.TrimSpace(lines[0])
		content := ""
		if len(lines) > 1 {
			content = strings.TrimSpace(lines[1])
		}
		content = normalizeOptionBlocks(content)
		steps = append(steps, TutorialStep{
			Number:  i,
			Title:   title,
			Content: content,
		})
	}
	return steps
}

var accordionBeginRE = regexp.MustCompile(`\[ACCORDION-BEGIN\s+\[Step\s+\d+:\s*\]\((.+?)\)\]`)

func parseV1Steps(body string) []TutorialStep {
	var steps []TutorialStep
	matches := accordionBeginRE.FindAllStringSubmatchIndex(body, -1)
	for i, match := range matches {
		title := body[match[2]:match[3]]
		contentStart := match[1]
		var contentEnd int
		endTag := "[ACCORDION-END]"
		endIdx := strings.Index(body[contentStart:], endTag)
		if endIdx >= 0 {
			contentEnd = contentStart + endIdx
		} else if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		} else {
			contentEnd = len(body)
		}
		content := strings.TrimSpace(body[contentStart:contentEnd])
		content = normalizeOptionBlocks(content)
		steps = append(steps, TutorialStep{
			Number:  i + 1,
			Title:   title,
			Content: content,
		})
	}
	return steps
}

var optionBeginRE = regexp.MustCompile(`\[OPTION BEGIN \[(.+?)\]\]`)

func normalizeOptionBlocks(content string) string {
	content = optionBeginRE.ReplaceAllString(content, "\n#### Option: $1\n")
	content = strings.ReplaceAll(content, "[OPTION END]", "")
	return strings.TrimSpace(content)
}
