package cmd

import (
	"fmt"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
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

		packs, err := loader.LoadPacks(activeProfile)
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
			fmt.Println("No tips available. Run 'sap-devs sync' to download content.")
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

func init() {
	rootCmd.AddCommand(tipCmd)
}
