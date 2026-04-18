package cmd

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
	influencersAll    bool
	influencersPack   string
	influencersTags   string
	influencersRandom bool
	influencersLink   string
)

var influencersCmd = &cobra.Command{
	Use:   "influencers",
	Short: "Browse SAP community influencers",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var packs []*content.Pack

		if influencersPack != "" || influencersAll {
			packs, err = loader.LoadPacks(nil, i18n.ActiveLang)
			if err != nil {
				return err
			}
		} else {
			paths, err := xdg.New()
			if err != nil {
				return err
			}
			profileCfg, err := config.LoadProfile(paths.ConfigDir)
			if err != nil {
				return err
			}
			if profileCfg.ID == "" {
				return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "influencers.no_profile"))
			}
			activeProfile, err := loader.FindProfile(profileCfg.ID)
			if err != nil {
				return err
			}
			if activeProfile == nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "influencers.profile_not_found", map[string]any{"ID": profileCfg.ID}))
			}
			packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}

		var influencers []content.Influencer
		if influencersPack != "" {
			influencers = content.FilterInfluencersByPack(packs, influencersPack)
		} else {
			influencers = content.FlattenInfluencers(packs)
		}

		if influencersTags != "" {
			tags := strings.Split(influencersTags, ",")
			influencers = content.FilterInfluencersByTags(influencers, tags)
		}

		if len(influencers) == 0 {
			if influencersPack != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "influencers.none_pack", map[string]any{"Pack": influencersPack}))
			} else if influencersTags != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "influencers.none_tags", map[string]any{"Tags": influencersTags}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "influencers.none"))
			}
			return nil
		}

		if influencersRandom {
			inf := content.RandomInfluencer(influencers, time.Now().UnixNano())
			if inf == nil {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "influencers.none"))
				return nil
			}
			printInfluencerCard(cmd, inf)
			return nil
		}

		printInfluencerTable(cmd, influencers)
		return nil
	},
}

var influencersOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: "Open an influencer's link in the browser",
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
		inf := content.FindInfluencer(content.FlattenInfluencers(packs), args[0])
		if inf == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "influencers.not_found", map[string]any{"ID": args[0]}))
		}

		linkType := influencersLink
		var url string
		if linkType != "" {
			url = inf.Links[linkType]
			if url == "" {
				available := sortedLinkTypes(inf.Links)
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "influencers.link_not_found", map[string]any{
					"ID":        inf.ID,
					"Link":      linkType,
					"Available": strings.Join(available, ", "),
				}))
			}
		} else {
			url = primaryLink(inf.Links)
		}

		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "influencers.open.browser_fail", map[string]any{"Err": err, "URL": url}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "influencers.open.opening", map[string]any{"Name": inf.Name, "URL": url}))
		return nil
	},
}

func printInfluencerTable(cmd *cobra.Command, influencers []content.Influencer) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		i18n.T(i18n.ActiveLang, "influencers.col_name"),
		i18n.T(i18n.ActiveLang, "influencers.col_role"),
		i18n.T(i18n.ActiveLang, "influencers.col_org"),
		i18n.T(i18n.ActiveLang, "influencers.col_focus"),
		i18n.T(i18n.ActiveLang, "influencers.col_links"),
	)
	for _, inf := range influencers {
		focus := strings.Join(inf.Focus, ",")
		links := strings.Join(sortedLinkTypes(inf.Links), " ")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", inf.Name, inf.Role, inf.Org, focus, links)
	}
	w.Flush()
}

func printInfluencerCard(cmd *cobra.Command, inf *content.Influencer) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s — %s @ %s\n", inf.Name, inf.Role, inf.Org)
	fmt.Fprintf(cmd.OutOrStdout(), "Focus: %s\n", strings.Join(inf.Focus, ", "))
	for _, k := range sortedLinkTypes(inf.Links) {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %s\n", k+":", inf.Links[k])
	}
}

func primaryLink(links map[string]string) string {
	if u, ok := links["blog"]; ok {
		return u
	}
	for _, k := range sortedLinkTypes(links) {
		return links[k]
	}
	return ""
}

func sortedLinkTypes(links map[string]string) []string {
	keys := make([]string, 0, len(links))
	for k := range links {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func init() {
	influencersCmd.Flags().BoolVarP(&influencersAll, "all", "a", false, "show all influencers regardless of profile")
	influencersCmd.Flags().StringVarP(&influencersPack, "pack", "p", "", "filter to a specific pack")
	influencersCmd.Flags().StringVarP(&influencersTags, "tags", "t", "", "comma-separated focus tags (OR match)")
	influencersCmd.Flags().BoolVarP(&influencersRandom, "random", "r", false, "show one random influencer")
	influencersOpenCmd.Flags().StringVarP(&influencersLink, "link", "l", "", "link type to open (blog, github, twitter, etc.)")
	influencersCmd.AddCommand(influencersOpenCmd)
	rootCmd.AddCommand(influencersCmd)
}
