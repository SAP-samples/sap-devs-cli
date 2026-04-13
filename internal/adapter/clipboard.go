package adapter

import (
	"fmt"
	"strings"

	"golang.design/x/clipboard"
)

// ExportToClipboard writes content to the system clipboard.
// If clipboard access is unavailable (headless, no display), it falls back
// to printing the content to stdout with usage instructions.
func ExportToClipboard(content, instructions string, dryRun bool) error {
	if dryRun {
		fmt.Printf("[dry-run] would copy %d bytes to clipboard\n", len(content))
		if instructions != "" {
			fmt.Printf("[dry-run] instructions: %s\n", instructions)
		}
		return nil
	}

	if err := clipboard.Init(); err != nil {
		// Clipboard unavailable — print to stdout as fallback
		fmt.Printf("--- SAP Developer Context ---\n%s\n--- End ---\n", strings.TrimSpace(content))
		if instructions != "" {
			fmt.Printf("\n%s\n", instructions)
		}
		return nil
	}

	ch := clipboard.Write(clipboard.FmtText, []byte(content))
	if ch == nil {
		// Write failed despite Init succeeding — fall back to stdout
		fmt.Printf("--- SAP Developer Context ---\n%s\n--- End ---\n", strings.TrimSpace(content))
		if instructions != "" {
			fmt.Printf("\n%s\n", instructions)
		}
		return nil
	}
	fmt.Println("SAP developer context copied to clipboard.")
	if instructions != "" {
		fmt.Printf("%s\n", instructions)
	}
	return nil
}
