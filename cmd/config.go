package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/credentials"
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
		fmt.Fprintf(cmd.OutOrStdout(), "company_repo:    %s\n", cfg.CompanyRepo)
		fmt.Fprintf(cmd.OutOrStdout(), "language:        %s\n", cfg.Language)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.tips:       %s\n", cfg.Sync.Tips)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.tools:      %s\n", cfg.Sync.Tools)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.advocates:  %s\n", cfg.Sync.Advocates)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.resources:  %s\n", cfg.Sync.Resources)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.context:    %s\n", cfg.Sync.Context)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.mcp:        %s\n", cfg.Sync.MCP)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.disabled:   %v\n", cfg.Sync.Disabled)

		// Show token status (masked — never show the full value)
		tok, loadErr := credentials.Load(paths.ConfigDir)
		fmt.Fprintf(cmd.OutOrStdout(), "github_token:    %s\n", maskedToken(tok, loadErr))
		return nil
	},
}

// maskedToken returns a display-safe representation of a stored token.
// It is extracted here so tests can verify masking logic directly.
func maskedToken(tok string, err error) string {
	switch {
	case err == nil:
		if len(tok) < 4 {
			return "(set)"
		}
		return tok[:4] + "****"
	case errors.Is(err, credentials.ErrNotFound):
		return "(not set)"
	default:
		return "(unavailable)"
	}
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
		case "language":
			cfg.Language = args[1]
		default:
			return fmt.Errorf("unknown config key: %s", args[0])
		}
		if err := cfg.Save(paths.ConfigDir); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", args[0], args[1])
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
		fmt.Fprintf(cmd.OutOrStdout(), "Company repo set to: %s\n", args[0])
		return nil
	},
}

var tokenDeleteFlag bool

var configTokenCmd = &cobra.Command{
	Use:   "token [value]",
	Short: "Store a GitHub token for authenticating with github.tools.sap",
	Long: `Store a Personal Access Token for authenticating with github.tools.sap.

Only required when syncing content from a private GitHub Enterprise instance
(github.tools.sap). Not needed if you are outside the SAP network or already
have GITHUB_TOOLS_SAP_TOKEN set in your environment.

The token is stored in the OS keychain (macOS Keychain, Windows Credential
Manager, Linux Secret Service). On systems without a keychain, it falls back
to a credentials file with restricted permissions.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if tokenDeleteFlag && len(args) > 0 {
			return fmt.Errorf("cannot use --delete with a token value")
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		if tokenDeleteFlag {
			err := credentials.Delete(paths.ConfigDir)
			if errors.Is(err, credentials.ErrNotFound) {
				fmt.Fprintln(cmd.OutOrStdout(), "No token was stored.")
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Token removed.")
			return nil
		}

		var token string
		if len(args) == 1 {
			token = args[0]
			fmt.Fprintln(cmd.OutOrStdout(), "Warning: token passed as argument may be saved in shell history.")
			fmt.Fprintln(cmd.OutOrStdout(), "Consider using 'sap-devs config token' without arguments for interactive entry.")
		} else {
			fmt.Fprint(cmd.OutOrStdout(), "Enter GitHub token (input hidden, will not appear in shell history): ")
			raw, readErr := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(cmd.OutOrStdout()) // newline after hidden input
			if readErr != nil {
				return fmt.Errorf("interactive input not available — pass token as argument: sap-devs config token <value>: %w", readErr)
			}
			token = strings.TrimSpace(string(raw))
			if token == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No token entered.")
				return nil
			}
		}

		if err := credentials.Store(paths.ConfigDir, token); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Token stored securely.")
		return nil
	},
}

func init() {
	configTokenCmd.Flags().BoolVar(&tokenDeleteFlag, "delete", false, "Remove the stored token")
	configCmd.AddCommand(configShowCmd, configSetCmd, configCompanyCmd, configTokenCmd)
	rootCmd.AddCommand(configCmd)
}
