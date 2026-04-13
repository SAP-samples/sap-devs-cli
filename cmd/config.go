package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage sap-devs configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		fmt.Printf("company_repo:    %s\n", cfg.CompanyRepo)
		fmt.Printf("sync.tips:       %s\n", cfg.Sync.Tips)
		fmt.Printf("sync.tools:      %s\n", cfg.Sync.Tools)
		fmt.Printf("sync.advocates:  %s\n", cfg.Sync.Advocates)
		fmt.Printf("sync.resources:  %s\n", cfg.Sync.Resources)
		fmt.Printf("sync.context:    %s\n", cfg.Sync.Context)
		fmt.Printf("sync.mcp:        %s\n", cfg.Sync.MCP)
		fmt.Printf("sync.disabled:   %v\n", cfg.Sync.Disabled)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		switch args[0] {
		case "company_repo":
			cfg.CompanyRepo = args[1]
		default:
			return fmt.Errorf("unknown config key: %s", args[0])
		}
		if err := cfg.Save(paths.ConfigDir); err != nil {
			return err
		}
		fmt.Printf("Set %s = %s\n", args[0], args[1])
		return nil
	},
}

var configCompanyCmd = &cobra.Command{
	Use:   "company <git-url>",
	Short: "Configure the company content repo URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		cfg.CompanyRepo = args[0]
		if err := cfg.Save(paths.ConfigDir); err != nil {
			return err
		}
		fmt.Printf("Company repo set to: %s\n", args[0])
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd, configSetCmd, configCompanyCmd)
	rootCmd.AddCommand(configCmd)
}
