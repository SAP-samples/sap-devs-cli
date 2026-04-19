package cmd

import (
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/editor"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
)

var contentEditCmd = &cobra.Command{
	Use:   "edit <file>",
	Short: "Edit a content YAML file",
	Long: `Edit a content YAML file using the interactive editor.

<file> can be:
  - A bare filename:       resources.yaml
  - A pack/file pair:      cap/resources.yaml
  - A direct path:         ./content/packs/cap/resources.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runContentEdit,
}

func init() {
	contentCmd.AddCommand(contentEditCmd)
}

func runContentEdit(cmd *cobra.Command, args []string) error {
	arg := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Check for ambiguous bare filename (no slash, no path prefix).
	if !strings.Contains(arg, "/") && !strings.HasPrefix(arg, ".") {
		if _, ok := schema.SchemaForFile(arg); ok {
			packs := editor.AmbiguousPacks(cwd, arg)
			if len(packs) > 1 {
				var selected string
				selectForm := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title(fmt.Sprintf("Multiple packs contain %s — pick one:", arg)).
							Options(huh.NewOptions[string](packs...)...).
							Value(&selected),
					),
				)
				if err := selectForm.Run(); err != nil {
					return err
				}
				arg = selected + "/" + arg
			}
		}
	}

	target, err := editor.ResolveEditTarget(cwd, arg)
	if err != nil {
		return err
	}

	schemasDir, err := findSchemasDir(cwd)
	if err != nil {
		return fmt.Errorf("cannot locate schemas directory: %w", err)
	}

	return editor.Run(target, schemasDir)
}
