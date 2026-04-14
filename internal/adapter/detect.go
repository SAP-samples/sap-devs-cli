package adapter

import (
	"os"
	"os/exec"
	"strings"
)

// Detect returns true if the adapter is present on this machine.
// It iterates the adapter's Detect rules and returns true on the first passing rule.
// A "command" rule passes if the command exits with code 0.
// A "path" rule passes if the expanded path exists on the filesystem.
// Returns false if Detect is empty or all rules fail.
func Detect(a Adapter) bool {
	for _, rule := range a.Detect {
		if rule.Command != "" {
			parts := strings.Fields(rule.Command)
			if len(parts) == 0 {
				continue
			}
			if err := exec.Command(parts[0], parts[1:]...).Run(); err == nil {
				return true
			}
		}
		if rule.Path != "" {
			expanded, err := ExpandHome(rule.Path)
			if err != nil {
				continue
			}
			if _, err := os.Stat(expanded); err == nil {
				return true
			}
		}
	}
	return false
}
