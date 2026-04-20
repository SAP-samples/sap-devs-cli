package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/shellhook"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
	"github.com/SAP-samples/sap-devs-cli/internal/youtube"
)

var tipMarkdown bool
var tipPlain bool
var tipNew bool

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

// tipSeed returns the seed for tip selection.
// useRandom=true (--new flag or dev mode) returns a unique value on every call.
// Otherwise the seed is derived from the current time at the rotation granularity.
func tipSeed(rotation string, useRandom bool) int64 {
	if useRandom {
		return time.Now().UnixNano()
	}
	now := time.Now()
	switch rotation {
	case "hourly", "session": // "session" is a stateless alias for hourly-granularity seeding
		// All terms cast to int64 before arithmetic to avoid 32-bit int overflow
		return int64(now.Year())*100000 + int64(now.YearDay())*24 + int64(now.Hour())
	default: // "daily" and ""
		return int64(now.Year())*1000 + int64(now.YearDay())
	}
}

func formatFridayTip(ep youtube.Episode) *content.Tip {
	desc := ep.Description
	if desc != "" {
		runes := []rune(desc)
		if len(runes) > 280 {
			desc = string(runes[:280]) + "…"
		}
	}
	var c string
	if desc == "" {
		c = ep.URL
	} else {
		c = ep.URL + "\n\n" + desc
	}
	return &content.Tip{
		Title:   "SAP Developer News — " + ep.Title,
		Content: c,
	}
}

func staticFridayTip() *content.Tip {
	return &content.Tip{
		Title:   "It's Friday — SAP Developer News is out!",
		Content: "Watch the latest episode:\n" + newsPlaylistURL,
	}
}

func fridayNewsOverride() *content.Tip {
	if time.Now().Weekday() != time.Friday {
		return nil
	}
	episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
	if err != nil || len(episodes) == 0 {
		return staticFridayTip()
	}
	return formatFridayTip(episodes[0])
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

		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}

		rotation := cfg.Tip.Rotation
		if rotation != "" && rotation != "daily" && rotation != "hourly" && rotation != "session" {
			fmt.Fprintf(cmd.ErrOrStderr(), "sap-devs: unknown tip_rotation value %q, falling back to daily\n", rotation)
			rotation = ""
		}

		useRandom := tipNew || os.Getenv("SAP_DEVS_DEV") == "1"
		seed := tipSeed(rotation, useRandom)

		var selectedTip *content.Tip
		if !useRandom {
			selectedTip = fridayNewsOverride()
		}
		if selectedTip == nil {
			selectedTip, err = content.SelectTip(packs, tipTags, seed)
			if err != nil {
				// No tips available — not an error worth surfacing as exit code 1
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tip.no_tips"))
				return nil
			}
		}
		tip := selectedTip

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
	tipCmd.Flags().BoolVar(&tipNew, "new", false, "show a different tip than the current rotation slot")
	tipCmd.AddCommand(tipInstallCmd)
	tipCmd.AddCommand(tipUninstallCmd)
	rootCmd.AddCommand(tipCmd)
}
