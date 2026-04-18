package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var configEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Configure event filtering and notification settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
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
		return nil
	},
}

var configEventsLocalRadiusCmd = &cobra.Command{
	Use:   "local-radius [km]",
	Short: "Set the local event radius in km",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return configEventsInt(cmd, args,
			func(cfg *config.Config) int { return cfg.Events.EffectiveLocalRadius() },
			func(cfg *config.Config, v int) { cfg.Events.LocalRadius = v },
			"config.events.local_radius.current", "config.events.local_radius.done")
	},
}

var configEventsRegionalRadiusCmd = &cobra.Command{
	Use:   "regional-radius [km]",
	Short: "Set the regional event radius in km",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return configEventsInt(cmd, args,
			func(cfg *config.Config) int { return cfg.Events.EffectiveRegionalRadius() },
			func(cfg *config.Config, v int) { cfg.Events.RegionalRadius = v },
			"config.events.regional_radius.current", "config.events.regional_radius.done")
	},
}

var configEventsNotifyDaysCmd = &cobra.Command{
	Use:   "notify-days [days]",
	Short: "Set the notification lookahead window in days",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return configEventsInt(cmd, args,
			func(cfg *config.Config) int { return cfg.Events.EffectiveNotifyDays() },
			func(cfg *config.Config, v int) { cfg.Events.NotifyDays = v },
			"config.events.notify_days.current", "config.events.notify_days.done")
	},
}

var validNotifyMethods = []string{"hook", "os", "both"}

var configEventsNotifyMethodCmd = &cobra.Command{
	Use:   "notify-method [hook|os|both]",
	Short: "Set the notification method",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			method := args[0]
			valid := false
			for _, m := range validNotifyMethods {
				if method == m {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "config.events.notify_method.invalid", map[string]any{"Value": method}))
			}
			cfg.Events.NotifyMethod = method
			if err := cfg.Save(paths.ConfigDir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.events.notify_method.done", map[string]any{"Value": method}))
			return nil
		}
		method := cfg.Events.NotifyMethod
		if method == "" {
			method = "hook"
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.events.notify_method.current", map[string]any{"Value": method}))
		return nil
	},
}

func configEventsInt(cmd *cobra.Command, args []string,
	getter func(*config.Config) int, setter func(*config.Config, int),
	currentKey, doneKey string) error {
	paths, err := xdg.New()
	if err != nil {
		return err
	}
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil {
		return err
	}
	if len(args) == 1 {
		v, err := strconv.Atoi(args[0])
		if err != nil || v <= 0 {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "config.events.invalid_number", map[string]any{"Value": args[0]}))
		}
		setter(cfg, v)
		if err := cfg.Save(paths.ConfigDir); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, doneKey, map[string]any{"Value": v}))
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, currentKey, map[string]any{"Value": getter(cfg)}))
	return nil
}

func init() {
	configEventsCmd.AddCommand(configEventsLocalRadiusCmd, configEventsRegionalRadiusCmd,
		configEventsNotifyDaysCmd, configEventsNotifyMethodCmd)
}
