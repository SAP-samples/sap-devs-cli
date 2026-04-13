package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
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
			fmt.Println("No profiles found. Run 'sap-devs sync' to download content.")
			return nil
		}
		for _, p := range profiles {
			fmt.Printf("  %-25s %s\n", p.ID, p.Description)
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
			return fmt.Errorf("profile %q not found — run 'sap-devs profile list' to see options", args[0])
		}
		if err := config.SaveProfile(paths.ConfigDir, &config.Profile{ID: p.ID}); err != nil {
			return err
		}
		fmt.Printf("Profile set to: %s (%s)\n", p.ID, p.Name)
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
			fmt.Println("No profile set. Run 'sap-devs profile list' to see options.")
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
			fmt.Printf("Profile: %s (definition not found in content)\n", saved.ID)
			return nil
		}
		fmt.Printf("Profile: %s — %s\n\n", p.Name, p.Description)
		fmt.Println("Pack weights:")
		for _, pw := range p.Packs {
			fmt.Printf("  %-20s %d\n", pw.ID, pw.Weight)
		}
		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd, profileSetCmd, profileShowCmd)
	rootCmd.AddCommand(profileCmd)
}
