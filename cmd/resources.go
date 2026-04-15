package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var resourcesCmd = &cobra.Command{
	Use:   "resources",
	Short: "Browse curated SAP resources",
}

var resourcesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List curated resources for your active profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		profileCfg, err := config.LoadProfile(paths.ConfigDir)
		if err != nil {
			return err
		}
		if profileCfg.ID == "" {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "resources.list.no_profile"))
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		activeProfile, err := loader.FindProfile(profileCfg.ID)
		if err != nil {
			return err
		}
		if activeProfile == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "resources.list.profile_not_found", map[string]any{"ID": profileCfg.ID}))
		}
		packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
		if err != nil {
			return err
		}
		resources := content.FlattenResources(packs)
		if len(resources) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "resources.list.no_resources"))
			return nil
		}
		printResourceTable(resources, false)
		return nil
	},
}

var resourcesSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across all SAP resources",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		resources := content.FilterResources(content.FlattenResources(packs), args[0])
		if len(resources) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "resources.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}
		printResourceTable(resources, true)
		return nil
	},
}

var resourcesOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: "Open a resource URL in the default browser",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		r := content.FindResource(content.FlattenResources(packs), args[0])
		if r == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "resources.open.not_found", map[string]any{"ID": args[0]}))
		}
		if err := browser.OpenURL(r.URL); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "resources.open.browser_fail", map[string]any{"Err": err, "URL": r.URL}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "resources.open.opening", map[string]any{"Title": r.Title, "URL": r.URL}))
		return nil
	},
}

// printResourceTable prints an aligned table of resources.
// showPack adds a PACK column between ID and TYPE (used by search).
func printResourceTable(resources []content.Resource, showPack bool) {
	colID := i18n.T(i18n.ActiveLang, "resources.col_id")
	colPack := i18n.T(i18n.ActiveLang, "resources.col_pack")
	colType := i18n.T(i18n.ActiveLang, "resources.col_type")
	colTitle := i18n.T(i18n.ActiveLang, "resources.col_title")
	if showPack {
		fmt.Printf("%-38s %-12s %-15s %s\n", colID, colPack, colType, colTitle)
		fmt.Println(strings.Repeat("-", 90))
		for _, r := range resources {
			fmt.Printf("%-38s %-12s %-15s %s\n", r.ID, r.PackID, r.Type, r.Title)
		}
	} else {
		fmt.Printf("%-38s %-15s %s\n", colID, colType, colTitle)
		fmt.Println(strings.Repeat("-", 75))
		for _, r := range resources {
			fmt.Printf("%-38s %-15s %s\n", r.ID, r.Type, r.Title)
		}
	}
}

func init() {
	resourcesCmd.AddCommand(resourcesListCmd, resourcesSearchCmd, resourcesOpenCmd)
	rootCmd.AddCommand(resourcesCmd)
}
