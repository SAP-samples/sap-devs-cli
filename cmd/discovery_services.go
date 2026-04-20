package cmd

import (
	"fmt"
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

var discoveryServicesCmd = &cobra.Command{
	Use:   "services",
	Short: i18n.T(i18n.ActiveLang, "discovery.services.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return discoveryServicesListCmd.RunE(cmd, args)
	},
}

var discoveryServicesListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T(i18n.ActiveLang, "discovery.services.short"),
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

		refs := content.FlattenDiscoveryServiceRefs(packs)
		filters := content.DiscoveryProfileFilters{}
		if !discoveryAll {
			filters = content.CollectProfileFilters(packs)
		}

		client := discovery.NewClient()
		services, err := discovery.ResolveServices(refs, filters, paths.CacheDir, discoveryForce, svcShowDeprecated, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		if svcCategoryFilter != "" {
			var filtered []discovery.Service
			for _, s := range services {
				if strings.Contains(strings.ToLower(s.Category), strings.ToLower(svcCategoryFilter)) {
					filtered = append(filtered, s)
				}
			}
			services = filtered
		}

		if len(services) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "discovery.services.no_services"))
			return nil
		}

		n := discoveryCount
		if n <= 0 || n > len(services) {
			n = len(services)
		}
		services = services[:n]

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "#", "NAME", "CATEGORY", "PRICING")
		for i, s := range services {
			name := s.Name
			for _, ref := range refs {
				if ref.ID == s.ID && ref.Featured {
					name = "★ " + name
					break
				}
			}
			pricing := formatPricing(s.LicenseModelType)
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i+1, name, s.Category, pricing)
		}
		w.Flush()
		return nil
	},
}

var discoveryServicesSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T(i18n.ActiveLang, "discovery.services.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		client := discovery.NewClient()
		allServices, err := discovery.ResolveServices(nil, content.DiscoveryProfileFilters{}, paths.CacheDir, discoveryForce, svcShowDeprecated, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		q := strings.ToLower(args[0])
		var matched []discovery.Service
		for _, s := range allServices {
			if strings.Contains(strings.ToLower(s.Name), q) ||
				strings.Contains(strings.ToLower(s.ShortDescription), q) ||
				strings.Contains(strings.ToLower(s.Category), q) {
				matched = append(matched, s)
			}
		}

		if len(matched) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.services.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "#", "NAME", "CATEGORY", "PRICING")
		for i, s := range matched {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i+1, s.Name, s.Category, formatPricing(s.LicenseModelType))
		}
		w.Flush()
		return nil
	},
}

var discoveryServicesOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: i18n.T(i18n.ActiveLang, "discovery.services.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		client := discovery.NewClient()
		allServices, err := discovery.ResolveServices(nil, content.DiscoveryProfileFilters{}, paths.CacheDir, discoveryForce, true, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		var target *discovery.Service
		for i := range allServices {
			if allServices[i].ID == args[0] || strings.EqualFold(allServices[i].ShortName, args[0]) {
				target = &allServices[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.services.open.not_found", map[string]any{"ID": args[0]}))
		}

		url := fmt.Sprintf("https://discovery-center.cloud.sap/serviceCatalog/%s", target.ID)
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.services.open.browser_fail", map[string]any{"Err": err, "URL": url}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.services.open.opening", map[string]any{"Name": target.Name, "URL": url}))
		return nil
	},
}

var (
	svcCategoryFilter string
	svcShowDeprecated bool
)

func init() {
	discoveryServicesCmd.PersistentFlags().StringVar(&svcCategoryFilter, "category", "", "filter by service category")
	discoveryServicesCmd.PersistentFlags().BoolVar(&svcShowDeprecated, "deprecated", false, "include deprecated services")
	discoveryServicesCmd.AddCommand(discoveryServicesListCmd, discoveryServicesSearchCmd, discoveryServicesOpenCmd)
	discoveryCmd.AddCommand(discoveryServicesCmd)
}

func formatPricing(licenseModel string) string {
	if licenseModel == "" {
		return ""
	}
	if strings.Contains(licenseModel, "free") {
		return "Free Tier"
	}
	if strings.Contains(licenseModel, "cloudcredits") || strings.Contains(licenseModel, "btpea") {
		return "Cloud Credits"
	}
	if strings.Contains(licenseModel, "subscription") {
		return "Subscription"
	}
	if strings.Contains(licenseModel, "payg") {
		return "Pay-as-you-go"
	}
	return licenseModel
}
