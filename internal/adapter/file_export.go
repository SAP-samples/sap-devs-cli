package adapter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

const exportGuidanceFmt = "Full SAP context saved to %s — upload to a ChatGPT Project for comprehensive knowledge."

// ExportFileAndClip writes fullCtx (raw Markdown) to a.ExportPath and copies
// a trimmed, formatted short summary plus a guidance line to the clipboard.
// fullCtx must be raw Markdown — FormatOutput is NOT applied to the file, only
// to the clipboard payload.
func ExportFileAndClip(a Adapter, fullCtx string, opts Options) error {
	if a.ExportPath == "" {
		return fmt.Errorf("adapter %s: export_path is required for file-export type", a.ID)
	}

	path, err := ExpandHome(a.ExportPath)
	if err != nil {
		return fmt.Errorf("adapter %s: %w", a.ID, err)
	}

	if opts.DryRun {
		fmt.Printf("[dry-run] would write export file %s (%d bytes)\n", path, len(fullCtx))
	} else {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("adapter %s: %w", a.ID, err)
		}
		if err := os.WriteFile(path, []byte(fullCtx), 0644); err != nil {
			return fmt.Errorf("adapter %s: write export file: %w", a.ID, err)
		}
	}

	// Build short clipboard payload: trim → format → append guidance
	short := content.TrimToBytes(fullCtx, a.MaxBytes)
	short = content.FormatOutput(short, a.Format)
	short = short + "\n" + fmt.Sprintf(exportGuidanceFmt, a.ExportPath)

	return ExportToClipboard(short, a.Instructions, opts.DryRun)
}
