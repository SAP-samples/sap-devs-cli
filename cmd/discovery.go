package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/discovery"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var discoveryCmd = &cobra.Command{
	Use:   "discovery",
	Short: i18n.T(i18n.ActiveLang, "discovery.short"),
	Long:  i18n.T(i18n.ActiveLang, "discovery.long"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return discoveryMissionsCmd.RunE(cmd, args)
	},
}

var discoveryMissionsCmd = &cobra.Command{
	Use:   "missions",
	Short: i18n.T(i18n.ActiveLang, "discovery.missions.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return discoveryMissionsListCmd.RunE(cmd, args)
	},
}

var discoveryMissionsListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T(i18n.ActiveLang, "discovery.missions.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var packs []*content.Pack
		if !discoveryAll {
			profileCfg, err := config.LoadProfile(paths.ConfigDir)
			if err == nil && profileCfg.ID != "" {
				if p, _ := loader.FindProfile(profileCfg.ID); p != nil {
					packs, err = loader.LoadPacks(p, i18n.ActiveLang)
				}
			}
		}
		if packs == nil {
			packs, err = loader.LoadPacks(nil, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}

		refs := content.FlattenDiscoveryMissionRefs(packs)
		filters := content.DiscoveryProfileFilters{}
		if !discoveryAll {
			filters = content.CollectProfileFilters(packs)
		}

		client := discovery.NewClient()
		missions, err := discovery.ResolveMissions(refs, filters, paths.CacheDir, discoveryForce, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		if len(missions) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "discovery.missions.no_missions"))
			return nil
		}

		// Apply flags.
		if discoveryCategory != "" {
			missions = filterMissionsByCategory(missions, discoveryCategory)
		}
		if discoveryProduct != "" {
			missions = filterMissionsByProduct(missions, discoveryProduct)
		}
		if discoveryEffort != "" {
			missions = filterMissionsByEffort(missions, discoveryEffort)
		}

		n := discoveryCount
		if n <= 0 || n > len(missions) {
			n = len(missions)
		}
		missions = missions[:n]

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", "#", "FEATURED", "EFFORT", "NAME", "CATEGORY")
		for i, m := range missions {
			featured := ""
			for _, ref := range refs {
				if ref.ID == m.ID && ref.Featured {
					featured = "★"
					break
				}
			}
			effort := discovery.EffortLabels[m.Effort]
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, featured, effort, m.Name, formatCategories(m.Category))
		}
		w.Flush()
		return nil
	},
}

var discoveryMissionsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T(i18n.ActiveLang, "discovery.missions.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}

		filters := discovery.SearchFilters{Top: discoveryCount}
		if !discoveryAll {
			loader, err := newContentLoader()
			if err != nil {
				return err
			}
			profileCfg, _ := config.LoadProfile(paths.ConfigDir)
			if profileCfg.ID != "" {
				if p, _ := loader.FindProfile(profileCfg.ID); p != nil {
					packs, _ := loader.LoadPacks(p, i18n.ActiveLang)
					pf := content.CollectProfileFilters(packs)
					filters.Product = joinCSV(pf.Products)
					filters.Category = joinCSV(pf.Categories)
					filters.FocusTags = joinCSV(pf.FocusTags)
				}
			}
		}
		if discoveryCategory != "" {
			filters.Category = discoveryCategory
		}

		cacheKey := discovery.SearchCacheKey(args[0], filters)
		var missions []discovery.Mission
		if !discoveryForce {
			missions, _ = discovery.LoadCache[[]discovery.Mission](paths.CacheDir, cacheKey, discovery.SearchCacheTTL)
		}
		if missions == nil {
			client := discovery.NewClient()
			missions, err = client.SearchMissions(args[0], filters)
			if err != nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
			}
			_ = discovery.SaveCache(paths.CacheDir, cacheKey, missions)
		}

		if len(missions) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.missions.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", "#", "EFFORT", "NAME", "CATEGORY", "PARTNER")
		for i, m := range missions {
			effort := discovery.EffortLabels[m.Effort]
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, effort, m.Name, formatCategories(m.Category), m.PartnerCompany)
		}
		w.Flush()
		return nil
	},
}

var discoveryMissionsOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: i18n.T(i18n.ActiveLang, "discovery.missions.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		client := discovery.NewClient()
		allMissions, err := discovery.ResolveMissions(nil, content.DiscoveryProfileFilters{}, paths.CacheDir, discoveryForce, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		var target *discovery.Mission
		if id, err := strconv.Atoi(args[0]); err == nil {
			for i := range allMissions {
				if allMissions[i].ID == id {
					target = &allMissions[i]
					break
				}
			}
		}
		if target == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.missions.open.not_found", map[string]any{"ID": args[0]}))
		}

		url := fmt.Sprintf("https://discovery-center.cloud.sap/missiondetail/%d/", target.ID)
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.missions.open.browser_fail", map[string]any{"Err": err, "URL": url}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.missions.open.opening", map[string]any{"Name": target.Name, "URL": url}))
		return nil
	},
}

// Flags
var (
	discoveryAll      bool
	discoveryForce    bool
	discoveryCount    int
	discoveryCategory string
	discoveryProduct  string
	discoveryEffort   string
)

func init() {
	// Shared flags on parent
	discoveryCmd.PersistentFlags().BoolVar(&discoveryAll, "all", false, "bypass profile filtering")
	discoveryCmd.PersistentFlags().BoolVar(&discoveryForce, "force", false, "bypass cache")
	discoveryCmd.PersistentFlags().IntVarP(&discoveryCount, "count", "n", 20, "limit results")

	// Missions flags
	discoveryMissionsCmd.PersistentFlags().StringVar(&discoveryCategory, "category", "", "filter by category code")
	discoveryMissionsCmd.PersistentFlags().StringVar(&discoveryProduct, "product", "", "filter by product")
	discoveryMissionsCmd.PersistentFlags().StringVar(&discoveryEffort, "effort", "", "filter by effort level (0-3)")

	// Also expose missions flags on parent (since parent defaults to missions list)
	discoveryCmd.Flags().StringVar(&discoveryCategory, "category", "", "filter by category code")
	discoveryCmd.Flags().StringVar(&discoveryEffort, "effort", "", "filter by effort level (0-3)")

	discoveryMissionsCmd.AddCommand(discoveryMissionsListCmd, discoveryMissionsSearchCmd, discoveryMissionsOpenCmd)
	discoveryCmd.AddCommand(discoveryMissionsCmd)
	rootCmd.AddCommand(discoveryCmd)
}

// Helpers

func filterMissionsByCategory(missions []discovery.Mission, cat string) []discovery.Mission {
	var out []discovery.Mission
	for _, m := range missions {
		if discovery.ContainsCSV(m.Category, cat) {
			out = append(out, m)
		}
	}
	return out
}

func filterMissionsByProduct(missions []discovery.Mission, product string) []discovery.Mission {
	var out []discovery.Mission
	for _, m := range missions {
		if discovery.ContainsCSV(m.Product, product) {
			out = append(out, m)
		}
	}
	return out
}

func filterMissionsByEffort(missions []discovery.Mission, effort string) []discovery.Mission {
	var out []discovery.Mission
	for _, m := range missions {
		if m.Effort == effort {
			out = append(out, m)
		}
	}
	return out
}

func formatCategories(csv string) string {
	parts := make([]string, 0)
	for _, code := range splitCSV(csv) {
		if name, ok := discovery.CategoryMapping[code]; ok {
			parts = append(parts, name)
		} else {
			parts = append(parts, code)
		}
	}
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return joinCSV(parts)
}

func splitCSV(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func joinCSV(parts []string) string {
	return strings.Join(parts, ",")
}
