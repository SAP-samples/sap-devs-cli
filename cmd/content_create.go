package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/editor"
)

var contentCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new content pack",
	Long:  "Guided wizard to scaffold a new content pack with pack.yaml, context.md, and optional content files.",
	RunE:  runContentCreate,
}

func init() {
	contentCmd.AddCommand(contentCreateCmd)
}

func runContentCreate(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	schemasDir, err := findSchemasDir(cwd)
	if err != nil {
		return fmt.Errorf("cannot locate schemas directory: %w", err)
	}

	return editor.RunCreateWizard(cwd, schemasDir)
}
