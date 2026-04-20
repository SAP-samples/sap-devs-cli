package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/SAP-samples/sap-devs-cli/internal/discovery"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var discoveryGuidanceCmd = &cobra.Command{
	Use:   "guidance",
	Short: i18n.T(i18n.ActiveLang, "discovery.guidance.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		client := discovery.NewClient()
		tree, err := discovery.ResolveGuidanceTree(paths.CacheDir, discoveryForce, guidanceDomain, client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\n", "PHASE", "TOPIC", "DOMAIN")
		for _, phase := range tree {
			for i, child := range phase.Children {
				phaseLabel := ""
				if i == 0 {
					phaseLabel = phase.Name
				}
				domain := ""
				if child.Domain != nil {
					domain = *child.Domain
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", phaseLabel, child.Name, domain)
			}
		}
		w.Flush()
		return nil
	},
}

var discoveryGuidanceShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: i18n.T(i18n.ActiveLang, "discovery.guidance.show.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		client := discovery.NewClient()
		content, err := discovery.ResolveGuidanceContent(paths.CacheDir, discoveryForce, args[0], client)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "discovery.err_fetch", map[string]any{"Err": err}))
		}
		// Strip HTML <br> tags for terminal display.
		content = strings.ReplaceAll(content, "<br>", "\n")
		content = strings.ReplaceAll(content, "<br/>", "\n")
		content = strings.ReplaceAll(content, "<br />", "\n")
		fmt.Fprintln(cmd.OutOrStdout(), content)
		return nil
	},
}

var discoveryGuidanceOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: i18n.T(i18n.ActiveLang, "discovery.guidance.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := fmt.Sprintf("https://discovery-center.cloud.sap/guidance-framework/%s", args[0])
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.guidance.open.browser_fail", map[string]any{"Err": err, "URL": url}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "discovery.guidance.open.opening", map[string]any{"Name": args[0], "URL": url}))
		return nil
	},
}

var guidanceDomain string

func init() {
	discoveryGuidanceCmd.PersistentFlags().StringVar(&guidanceDomain, "domain", "", "filter by domain (e.g., Extensibility, Integration)")
	discoveryGuidanceCmd.AddCommand(discoveryGuidanceShowCmd, discoveryGuidanceOpenCmd)
	discoveryCmd.AddCommand(discoveryGuidanceCmd)
}
