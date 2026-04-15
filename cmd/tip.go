package cmd

import (
	"fmt"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/shellhook"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var tipCmd = &cobra.Command{
	Use:   "tip",
	Short: "Print a SAP developer tip (add to your shell profile)",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		profileCfg, err := config.LoadProfile(paths.ConfigDir)
		if err != nil {
			return err
		}

		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var activeProfile *content.Profile
		if profileCfg.ID != "" {
			activeProfile, err = loader.FindProfile(profileCfg.ID)
			if err != nil {
				return err
			}
		}

		packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
		if err != nil {
			return err
		}

		var tipTags []string
		if activeProfile != nil {
			tipTags = activeProfile.TipTags
		}

		// Use year*1000+yearday as seed for daily consistency
		now := time.Now()
		seed := int64(now.Year()*1000 + now.YearDay())

		tip, err := content.SelectTip(packs, tipTags, seed)
		if err != nil {
			// No tips available — not an error worth surfacing as exit code 1
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tip.no_tips"))
			return nil
		}

		md := fmt.Sprintf("## 💡 %s\n\n%s\n", tip.Title, tip.Content)
		rendered, err := glamour.Render(md, "dark")
		if err != nil {
			// Fallback to plain output if glamour fails
			fmt.Printf("💡 %s\n\n%s\n", tip.Title, tip.Content)
			return nil
		}
		fmt.Print(rendered)
		return nil
	},
}

var tipInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Add sap-devs tip to your shell profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := shellhook.Add("sap-devs tip", "# SAP developer tips")
		if err != nil && len(results) == 0 {
			// No profiles found — print manual fallback, not an error exit.
			fmt.Fprintln(cmd.OutOrStdout(), "No shell profile found. Add this line to your shell profile manually:")
			fmt.Fprintln(cmd.OutOrStdout(), "  sap-devs tip")
			return nil
		}
		for _, r := range results {
			if r.Updated {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Updated %s\n", r.Path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s — already configured\n", r.Path)
			}
		}
		return err
	},
}

var tipUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove sap-devs tip from your shell profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := shellhook.Remove("sap-devs tip", "# SAP developer tips")
		if err != nil && len(results) == 0 {
			return err
		}
		anyRemoved := false
		for _, r := range results {
			if r.Updated {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Removed from %s\n", r.Path)
				anyRemoved = true
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s — not configured\n", r.Path)
			}
		}
		if !anyRemoved {
			fmt.Fprintln(cmd.OutOrStdout(), "'sap-devs tip' was not found in any shell profile.")
		}
		return err
	},
}

func init() {
	tipCmd.AddCommand(tipInstallCmd)
	tipCmd.AddCommand(tipUninstallCmd)
	rootCmd.AddCommand(tipCmd)
}
