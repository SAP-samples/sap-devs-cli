package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const markerFmt = "<!-- sap-devs:start:%s -->"
const markerEndFmt = "<!-- sap-devs:end:%s -->"

// ReplaceSection writes `content` into `filePath` between HTML comment markers
// for the named section. If the section already exists it is replaced in-place;
// otherwise it is appended. Parent directories are created as needed.
// When dryRun is true the function prints what it would do but writes nothing.
func ReplaceSection(filePath, section, content string, dryRun bool) error {
	start := fmt.Sprintf(markerFmt, section)
	end := fmt.Sprintf(markerEndFmt, section)
	block := start + "\n" + strings.TrimRight(content, "\n") + "\n" + end + "\n"

	if dryRun {
		fmt.Printf("[dry-run] would write section %q to %s\n", section, filePath)
		return nil
	}

	// Read existing content (OK if file doesn't exist)
	existing := ""
	data, err := os.ReadFile(filePath)
	if err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return err
	}

	var result string
	startIdx := strings.Index(existing, start)
	endIdx := strings.Index(existing, end)
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		// Replace in-place; consume the trailing newline after the end marker if present
		afterEnd := endIdx + len(end)
		if afterEnd < len(existing) && existing[afterEnd] == '\n' {
			afterEnd++
		}
		result = existing[:startIdx] + block + existing[afterEnd:]
	} else {
		// Append with separator
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		if existing != "" {
			existing += "\n"
		}
		result = existing + block
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(result), 0644)
}

// ExpandHome replaces a leading ~ with the user's home directory.
func ExpandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path[1:]), nil
}
