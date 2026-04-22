package adapter

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteSkillFile writes a skill file to the adapter's skill directory.
// The skill is placed at <basePath>/<skillID>/SKILL.md.
// Idempotent: overwrites if content differs.
func WriteSkillFile(basePath, skillID, content string, dryRun bool) error {
	destPath := filepath.Join(basePath, skillID, "SKILL.md")
	if dryRun {
		fmt.Printf("[dry-run] would write skill %q to %s (%d bytes)\n", skillID, destPath, len(content))
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(destPath, []byte(content), 0644)
}

// RemoveSkillFile removes a skill file from the adapter's skill directory.
// Also removes the parent directory if it becomes empty.
func RemoveSkillFile(basePath, skillID string, dryRun bool) error {
	destDir := filepath.Join(basePath, skillID)
	destPath := filepath.Join(destDir, "SKILL.md")
	if dryRun {
		fmt.Printf("[dry-run] would remove skill %q from %s\n", skillID, destPath)
		return nil
	}
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = os.Remove(destDir)
	return nil
}

// SkillFileInstalled reports whether the skill file exists at the expected location.
func SkillFileInstalled(basePath, skillID string) bool {
	_, err := os.Stat(filepath.Join(basePath, skillID, "SKILL.md"))
	return err == nil
}
