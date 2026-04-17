package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var validRotationModes = []string{"daily", "hourly", "session"}

var configTipRotationCmd = &cobra.Command{
	Use:   "tip-rotation [daily|hourly|session]",
	Short: i18n.T(i18n.ActiveLang, "config.tip_rotation.short"),
	Long:  i18n.T(i18n.ActiveLang, "config.tip_rotation.long"),
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
			mode := args[0]
			if !isValidRotation(mode) {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "config.tip_rotation.invalid", map[string]any{"Mode": mode}))
			}
			cfg.Tip.Rotation = mode
			if err := cfg.Save(paths.ConfigDir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.tip_rotation.done", map[string]any{"Mode": mode}))
			return nil
		}

		// No args: show current value
		mode := cfg.Tip.Rotation
		if mode == "" {
			mode = "daily"
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.tip_rotation.current", map[string]any{"Mode": mode}))
		return nil
	},
}

func isValidRotation(mode string) bool {
	for _, v := range validRotationModes {
		if mode == v {
			return true
		}
	}
	return false
}
