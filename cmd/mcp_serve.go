package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/btpcli"
	"github.com/SAP-samples/sap-devs-cli/internal/cfcli"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/mcpserver"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var mcpServeProfile string

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SAP developer context MCP server (stdio)",
	Long:  "Starts a Model Context Protocol server on stdio. AI tools spawn this as a child process to query SAP developer knowledge on demand.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = ""
		}

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

		cliRunner := func(command string) (string, error) {
			parts := strings.Fields(command)
			if len(parts) == 0 {
				return "", fmt.Errorf("empty command")
			}
			cmd := exec.Command(parts[0], parts[1:]...)
			out, err := cmd.CombinedOutput()
			return string(out), err
		}

		cfConfigPath := resolveCFConfigPath()
		var cfClient *cfcli.Client
		if _, err := exec.LookPath("cf"); err == nil {
			cfClient = cfcli.NewClient(cliRunner, cfConfigPath)
		}

		var btpClient *btpcli.Client
		if _, err := exec.LookPath("btp"); err == nil {
			btpClient = btpcli.NewClient(cliRunner, resolveBTPConfigPath())
		}

		deps := mcpserver.Deps{
			Packs:         packs,
			Profile:       activeProfile,
			TutorialIndex: tutorialIndex,
			LearningIndex: learningIndex,
			CacheDir:      paths.CacheDir,
			ConfigDir:     paths.ConfigDir,
			Version:       Version,
			Cwd:           cwd,
			CFClient:      cfClient,
			BTPClient:     btpClient,
			CFConfigPath:  cfConfigPath,
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

func resolveCFConfigPath() string {
	cfHome := os.Getenv("CF_HOME")
	if cfHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		cfHome = home
	}
	return filepath.Join(cfHome, ".cf", "config.json")
}

func resolveBTPConfigPath() string {
	path := os.Getenv("BTP_CLIENTCONFIG")
	if path != "" {
		return path
	}
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			return filepath.Join(appdata, "SAP", "btp", "config.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	primary := filepath.Join(home, ".config", "btp", "config.json")
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	return filepath.Join(home, ".config", ".btp", "config.json")
}
