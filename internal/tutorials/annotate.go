package tutorials

import (
	"regexp"
	"strings"
)

type StepAnnotations struct {
	Commands          []CommandAnnotation      `json:"commands,omitempty"`
	FileCreates       []FileCreateAnnotation   `json:"file_creates,omitempty"`
	Verifications     []VerificationAnnotation `json:"verifications,omitempty"`
	PrerequisiteTools []string                 `json:"prerequisite_tools,omitempty"`
}

type CommandAnnotation struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	WorkingDir  string `json:"working_dir,omitempty"`
}

type FileCreateAnnotation struct {
	Filename string `json:"filename"`
	Language string `json:"language"`
	Content  string `json:"content"`
}

type VerificationAnnotation struct {
	Command      string `json:"command,omitempty"`
	ExpectOutput string `json:"expect_output,omitempty"`
	Description  string `json:"description,omitempty"`
	Confidence   string `json:"confidence,omitempty"`
}

var (
	fencedBlockRe    = regexp.MustCompile("(?m)^```(\\w*)\\n([\\s\\S]*?)^```")
	outputOrVerifyRe = regexp.MustCompile(`(?i)\b(output|result|you should see|returns?|prints?|logs?|response|verify|check|confirm|make sure|expected output|the result)\b`)
)

func AnnotateStep(md string) StepAnnotations {
	var ann StepAnnotations
	if md == "" {
		return ann
	}

	blocks := parseFencedBlocks(md)
	for _, b := range blocks {
		kind, confidence := classifyBlock(b)
		switch kind {
		case blockCommand:
			ann.Commands = append(ann.Commands, extractCommands(b)...)
		case blockFileCreate:
			if fc := extractFileCreate(b); fc != nil {
				ann.FileCreates = append(ann.FileCreates, *fc)
			}
		case blockVerification:
			ann.Verifications = append(ann.Verifications, extractVerification(b, confidence))
		}
	}
	ann.PrerequisiteTools = extractPrerequisites(md)
	return ann
}

type blockKind int

const (
	blockIgnored blockKind = iota
	blockCommand
	blockFileCreate
	blockVerification
)

type fencedBlock struct {
	lang          string
	content       string
	precedingText string
}

func parseFencedBlocks(md string) []fencedBlock {
	var blocks []fencedBlock
	matches := fencedBlockRe.FindAllStringSubmatchIndex(md, -1)
	for _, m := range matches {
		lang := md[m[2]:m[3]]
		content := md[m[4]:m[5]]
		preceding := precedingParagraph(md, m[0])
		blocks = append(blocks, fencedBlock{
			lang:          lang,
			content:       strings.TrimSpace(content),
			precedingText: preceding,
		})
	}
	return blocks
}

func precedingParagraph(md string, blockStart int) string {
	before := md[:blockStart]
	before = strings.TrimRight(before, " \t\n")
	lines := strings.Split(before, "\n")
	var para []string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			break
		}
		para = append([]string{line}, para...)
	}
	return strings.Join(para, " ")
}

var commandLangs = map[string]bool{"bash": true, "sh": true, "": true}

func classifyBlock(b fencedBlock) (blockKind, string) {
	if commandLangs[b.lang] {
		if outputOrVerifyRe.MatchString(b.precedingText) {
			return blockVerification, "high"
		}
		if isCommentOnly(b.content) {
			return blockIgnored, ""
		}
		return blockCommand, ""
	}
	if isCodeLang(b.lang) {
		if isVerificationContext(b.precedingText) {
			return blockVerification, "low"
		}
		if extractFilename(b.precedingText) != "" {
			return blockFileCreate, ""
		}
	}
	return blockIgnored, ""
}

func isCommentOnly(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return false
		}
	}
	return true
}

func extractCommands(b fencedBlock) []CommandAnnotation {
	desc := firstSentence(b.precedingText)
	var cmds []CommandAnnotation
	for _, line := range strings.Split(b.content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cmds = append(cmds, CommandAnnotation{
			Command:     line,
			Description: desc,
		})
		desc = ""
	}
	return cmds
}

func firstSentence(text string) string {
	if text == "" {
		return ""
	}
	text = strings.TrimRight(text, ":")
	if i := strings.Index(text, ". "); i >= 0 {
		return text[:i+1]
	}
	return text
}

var codeLangs = map[string]bool{
	"json": true, "yaml": true, "yml": true, "cds": true,
	"xml": true, "js": true, "ts": true, "java": true,
	"html": true, "css": true, "sql": true, "properties": true,
	"toml": true, "csv": true, "graphql": true,
}

func isCodeLang(lang string) bool {
	return codeLangs[strings.ToLower(lang)]
}

var verifyWordsRe = regexp.MustCompile(`(?i)\b(verify|check|you should see|the output should|confirm|make sure|expected output|the result)\b`)

func isVerificationContext(text string) bool {
	return verifyWordsRe.MatchString(text)
}

var filenameRe = regexp.MustCompile("`([^`]+\\.[a-zA-Z]+)`")

var fileActionRe = regexp.MustCompile(`(?i)\b(create|add the following to|open|edit|paste into|replace the content of|update|modify)\b`)

func extractFilename(text string) string {
	if !fileActionRe.MatchString(text) {
		return ""
	}
	m := filenameRe.FindStringSubmatch(text)
	if m != nil {
		return m[1]
	}
	return ""
}

func extractFileCreate(b fencedBlock) *FileCreateAnnotation {
	fn := extractFilename(b.precedingText)
	if fn == "" {
		return nil
	}
	return &FileCreateAnnotation{
		Filename: fn,
		Language: b.lang,
		Content:  b.content,
	}
}

func extractVerification(b fencedBlock, confidence string) VerificationAnnotation {
	return VerificationAnnotation{
		ExpectOutput: b.content,
		Description:  firstSentence(b.precedingText),
		Confidence:   confidence,
	}
}

var prerequisiteRe = regexp.MustCompile(`(?i)\b(?:make sure|ensure|you need|requires?|must have|install)\b[^.]*?` + "(?:`([^`]+)`|\\*\\*([^*]+)\\*\\*)")

func extractPrerequisites(md string) []string {
	seen := make(map[string]bool)
	var tools []string
	for _, m := range prerequisiteRe.FindAllStringSubmatch(md, -1) {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		name = strings.TrimSpace(name)
		if name != "" && !seen[name] {
			seen[name] = true
			tools = append(tools, name)
		}
	}
	return tools
}
