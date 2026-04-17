package cmd

import (
	"os"
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

var tipMarkdown bool
var tipPlain bool

// FormatTip formats a tip for non-interactive output. Returns empty string for
// the default case (caller uses glamour rendering instead).
func FormatTip(tip content.Tip, markdown, plain bool) string {
	if markdown {
		return fmt.Sprintf("## 💡 %s\n\n%s\n", tip.Title, tip.Content)
	}
	if plain {
		return fmt.Sprintf("%s\n\n%s\n", tip.Title, tip.Content)
	}
	return ""
}

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

		// If in local development mode, use a more variable seed
		if os.Getenv("SAP_DEVS_DEV") == "1" {
			seed = now.Unix()
		}

		tip, err := content.SelectTip(packs, tipTags, seed)
		if err != nil {
			// No tips available — not an error worth surfacing as exit code 1
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tip.no_tips"))
			return nil
		}

		if tipMarkdown || tipPlain {
			fmt.Fprint(cmd.OutOrStdout(), FormatTip(*tip, tipMarkdown, tipPlain))
			return nil
		}
		md := fmt.Sprintf("## 💡 %s\n\n%s\n", tip.Title, tip.Content)
		rendered, err := glamour.Render(md, "dark")
		if err != nil {
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
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tip.install.no_profile"))
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tip.install.no_profile_cmd"))
			return nil
		}
		for _, r := range results {
			if r.Updated {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "tip.install.updated", map[string]any{"Path": r.Path}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "tip.install.already", map[string]any{"Path": r.Path}))
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
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "tip.uninstall.removed", map[string]any{"Path": r.Path}))
				anyRemoved = true
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "tip.uninstall.not_configured", map[string]any{"Path": r.Path}))
			}
		}
		if !anyRemoved {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tip.uninstall.not_found"))
		}
		return err
	},
}

func init() {
	tipCmd.Flags().BoolVar(&tipMarkdown, "markdown", false, "output raw Markdown (no ANSI rendering)")
	tipCmd.Flags().BoolVar(&tipPlain, "plain", false, "output plain text (no Markdown or ANSI)")
	tipCmd.AddCommand(tipInstallCmd)
	tipCmd.AddCommand(tipUninstallCmd)
	rootCmd.AddCommand(tipCmd)
}
