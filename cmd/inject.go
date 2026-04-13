package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
	injectGlobal  bool
	injectProject bool
	injectTool    string
	injectDryRun  bool
)

var injectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Push SAP context to your AI tools",
	Long: `Inject up-to-date SAP developer context into all detected AI tools.

By default, injects at global (user) scope into tools such as Claude Code,
Cursor, and GitHub Copilot. Use --project to inject into project-level files
(CLAUDE.md, .cursorrules, etc.) in the current directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve scope
		scope := "global"
		if injectProject {
			scope = "project"
		}

		// Load content
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}
		configProfile, err := config.LoadProfile(paths.ConfigDir)
		if err != nil {
			return err
		}

		var activeProfile *content.Profile
		if configProfile.ID != "" {
			activeProfile, err = loader.FindProfile(configProfile.ID)
			if err != nil {
				return err
			}
		}

		packs, err := loader.LoadPacks(activeProfile)
		if err != nil {
			return err
		}

		rendered := content.RenderContext(packs, activeProfile)

		// Build and run engine
		opts := adapter.Options{
			Scope:      scope,
			ToolFilter: injectTool,
			DryRun:     injectDryRun,
		}
		engine, err := newAdapterEngine(rendered, opts)
		if err != nil {
			return err
		}

		if injectDryRun {
			fmt.Println("[dry-run] no files will be modified")
		}

		if err := engine.Run(); err != nil {
			return err
		}

		if !injectDryRun {
			fmt.Printf("SAP developer context injected (%s scope).\n", scope)
			if injectTool == "" {
				fmt.Println("Run 'sap-devs inject --dry-run' to preview changes before writing.")
			}
		}
		return nil
	},
}

func init() {
	injectCmd.Flags().BoolVar(&injectGlobal, "global", true, "inject at user (global) scope (default)")
	injectCmd.Flags().BoolVar(&injectProject, "project", false, "inject at project scope (current directory)")
	injectCmd.Flags().StringVar(&injectTool, "tool", "", "inject into a specific tool only (e.g. claude-code)")
	injectCmd.Flags().BoolVar(&injectDryRun, "dry-run", false, "preview changes without writing files")
	injectCmd.MarkFlagsMutuallyExclusive("global", "project")
	rootCmd.AddCommand(injectCmd)
}
