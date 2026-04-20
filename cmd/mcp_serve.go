package cmd

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/mcpserver"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var mcpServeProfile string

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SAP developer context MCP server (stdio)",
	Long:  "Starts a Model Context Protocol server on stdio. AI tools spawn this as a child process to query SAP developer knowledge on demand.",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return fmt.Errorf("failed to initialise content loader: %w", err)
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		profileID := mcpServeProfile
		if profileID == "" {
			cp, err := config.LoadProfile(paths.ConfigDir)
			if err != nil {
				return err
			}
			profileID = cp.ID
		}

		var activeProfile *content.Profile
		if profileID != "" {
			activeProfile, err = loader.FindProfile(profileID)
			if err != nil {
				return err
			}
			if activeProfile == nil {
				return fmt.Errorf("profile %q not found", profileID)
			}
		}

		packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
		if err != nil {
			return fmt.Errorf("failed to load packs: %w", err)
		}

		tutorialIndex, _ := tutorials.LoadIndex(paths.CacheDir)
		learningIndex, _ := learning.LoadIndex(paths.CacheDir, learning.CacheTTL)

		deps := mcpserver.Deps{
			Packs:         packs,
			Profile:       activeProfile,
			TutorialIndex: tutorialIndex,
			LearningIndex: learningIndex,
			Version:       Version,
		}

		s := mcpserver.NewServer(deps)

		fmt.Fprintln(os.Stderr, "sap-devs MCP server starting...")
		if err := server.ServeStdio(s); err != nil {
			return fmt.Errorf("MCP server error: %w", err)
		}
		return nil
	},
}

func init() {
	mcpServeCmd.Flags().StringVar(&mcpServeProfile, "profile", "", "override active profile")
	mcpCmd.AddCommand(mcpServeCmd)
}
