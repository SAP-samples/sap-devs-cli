package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage your developer profile",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available developer profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		profiles, err := loader.LoadProfiles()
		if err != nil {
			return err
		}
		if len(profiles) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "profile.list.no_profiles"))
			return nil
		}
		for _, p := range profiles {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-25s %s\n", p.ID, p.Description)
		}
		return nil
	},
}

var profileSetCmd = &cobra.Command{
	Use:   "set <profile-id>",
	Short: "Set your active developer profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		p, err := loader.FindProfile(args[0])
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "profile.set.not_found", map[string]any{"ID": args[0]}))
		}
		if err := config.SaveProfile(paths.ConfigDir, &config.Profile{ID: p.ID}); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "profile.set.done", map[string]any{"ID": p.ID, "Name": p.Name}))
		return nil
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show your current profile and pack weights",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		saved, err := config.LoadProfile(paths.ConfigDir)
		if err != nil {
			return err
		}
		if saved.ID == "" {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "profile.show.not_set"))
			return nil
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		p, err := loader.FindProfile(saved.ID)
		if err != nil {
			return err
		}
		if p == nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "profile.show.not_found", map[string]any{"ID": saved.ID}))
			return nil
		}
		fmt.Fprint(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "profile.show.header", map[string]any{"Name": p.Name, "Description": p.Description}))
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "profile.show.pack_weights"))
		for _, pw := range p.Packs {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %d\n", pw.ID, pw.Weight)
		}
		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd, profileSetCmd, profileShowCmd)
	rootCmd.AddCommand(profileCmd)
}
