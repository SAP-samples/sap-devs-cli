package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/credentials"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
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
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.company_repo", map[string]any{"Value": cfg.CompanyRepo}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.language", map[string]any{"Value": cfg.Language}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.location", map[string]any{"Value": cfg.Location}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.sync_tips", map[string]any{"Value": cfg.Sync.Tips}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.sync_tools", map[string]any{"Value": cfg.Sync.Tools}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.sync_advocates", map[string]any{"Value": cfg.Sync.Advocates}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.sync_resources", map[string]any{"Value": cfg.Sync.Resources}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.sync_context", map[string]any{"Value": cfg.Sync.Context}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.sync_mcp", map[string]any{"Value": cfg.Sync.MCP}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.sync_youtube", map[string]any{"Value": cfg.Sync.YouTube}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.sync_disabled", map[string]any{"Value": cfg.Sync.Disabled}))
		tipRotationDisplay := cfg.Tip.Rotation
		if tipRotationDisplay == "" {
			tipRotationDisplay = "daily"
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.tip_rotation", map[string]any{"Value": tipRotationDisplay}))
		method := cfg.Events.NotifyMethod
		if method == "" {
			method = "hook"
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.events.show", map[string]any{
			"LocalRadius":    cfg.Events.EffectiveLocalRadius(),
			"RegionalRadius": cfg.Events.EffectiveRegionalRadius(),
			"NotifyDays":     cfg.Events.EffectiveNotifyDays(),
			"NotifyMethod":   method,
		}))

		// Show token status (masked — never show the full value)
		tok, loadErr := credentials.Load(paths.ConfigDir)
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.github_token", map[string]any{"Value": maskedToken(tok, loadErr, i18n.ActiveLang)}))
		ytTok, ytLoadErr := credentials.LoadService(paths.ConfigDir, "youtube")
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.youtube_token", map[string]any{"Value": maskedToken(ytTok, ytLoadErr, i18n.ActiveLang)}))
		return nil
	},
}

// maskedToken returns a display-safe representation of a stored token.
// It is extracted here so tests can verify masking logic directly.
func maskedToken(tok string, err error, lang string) string {
	switch {
	case err == nil:
		if len(tok) < 4 {
			return i18n.T(lang, "config.token.masked_set")
		}
		return tok[:4] + "****"
	case errors.Is(err, credentials.ErrNotFound):
		return i18n.T(lang, "config.token.masked_not_set")
	default:
		return i18n.T(lang, "config.token.masked_unavailable")
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
		case "sync.youtube":
			d, parseErr := time.ParseDuration(args[1])
			if parseErr != nil {
				return fmt.Errorf("invalid duration %q for %s: %w", args[1], args[0], parseErr)
			}
			cfg.Sync.YouTube = d
		default:
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "config.set.unknown_key", map[string]any{"Key": args[0]}))
		}
		if err := cfg.Save(paths.ConfigDir); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.set.done", map[string]any{"Key": args[0], "Value": args[1]}))
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
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.company.done", map[string]any{"Value": args[0]}))
		return nil
	},
}

var tokenDeleteFlag bool
var tokenServiceFlag string

var configTokenCmd = &cobra.Command{
	Use:   "token [value]",
	Short: "Store a GitHub token for authenticating with GitHub",
	Long: `Store a Personal Access Token for authenticating with GitHub.

Only required when syncing content from a private repository or when
unauthenticated API rate limits are exceeded. Not needed if you already
have GH_TOKEN or GITHUB_TOKEN set in your environment.

The token is stored in the OS keychain (macOS Keychain, Windows Credential
Manager, Linux Secret Service). On systems without a keychain, it falls back
to a credentials file with restricted permissions.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if tokenDeleteFlag && len(args) > 0 {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "config.token.delete_with_value"))
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		if tokenDeleteFlag {
			var err error
			if tokenServiceFlag != "" {
				err = credentials.DeleteService(paths.ConfigDir, tokenServiceFlag)
			} else {
				err = credentials.Delete(paths.ConfigDir)
			}
			if errors.Is(err, credentials.ErrNotFound) {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "config.token.no_token"))
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "config.token.removed"))
			return nil
		}

		var token string
		if len(args) == 1 {
			token = args[0]
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "config.token.warn_history"))
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "config.token.history_hint"))
		} else {
			fmt.Fprint(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "config.token.prompt"))
			raw, readErr := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(cmd.OutOrStdout()) // newline after hidden input
			if readErr != nil {
				return fmt.Errorf("%s: %w", i18n.T(i18n.ActiveLang, "config.token.interactive_unavailable"), readErr)
			}
			token = strings.TrimSpace(string(raw))
			if token == "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "config.token.empty"))
				return nil
			}
		}

		var storeErr error
		if tokenServiceFlag != "" {
			storeErr = credentials.StoreService(paths.ConfigDir, tokenServiceFlag, token)
		} else {
			storeErr = credentials.Store(paths.ConfigDir, token)
		}
		if storeErr != nil {
			return storeErr
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "config.token.stored"))
		return nil
	},
}

func init() {
	configTokenCmd.Flags().BoolVar(&tokenDeleteFlag, "delete", false, "Remove the stored token")
	configTokenCmd.Flags().StringVar(&tokenServiceFlag, "service", "", "Service to store token for (e.g. youtube)")
	configCmd.AddCommand(configShowCmd, configSetCmd, configCompanyCmd, configTokenCmd, configLocationCmd, configTipRotationCmd, configEventsCmd)
	rootCmd.AddCommand(configCmd)
}
