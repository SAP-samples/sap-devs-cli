package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"text/tabwriter"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/videos"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var videosCmd = &cobra.Command{
	Use:   "videos",
	Short: i18n.T(i18n.ActiveLang, "videos.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return videosListCmd.RunE(cmd, args)
	},
}

var videosListN int
var videosListSource string
var videosListPack string

var videosListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T(i18n.ActiveLang, "videos.list.short"),
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
			activeProfile, _ = loader.FindProfile(profileCfg.ID)
		}
		packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
		if err != nil {
			return err
		}
		var allVids []content.Video
		for _, p := range packs {
			vids, _ := videos.ResolveAll(p.YouTubeSources, paths.CacheDir)
			allVids = append(allVids, vids...)
		}
		if videosListSource != "" {
			var filtered []content.Video
			for _, v := range allVids {
				if v.SourceID == videosListSource {
					filtered = append(filtered, v)
				}
			}
			allVids = filtered
		}
		if videosListPack != "" {
			var filtered []content.Video
			for _, v := range allVids {
				if v.PackID == videosListPack {
					filtered = append(filtered, v)
				}
			}
			allVids = filtered
		}
		if len(allVids) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "videos.no_videos"))
			return nil
		}
		sort.Slice(allVids, func(i, j int) bool {
			return allVids[i].Published.After(allVids[j].Published)
		})
		n := videosListN
		if n <= 0 || n > len(allVids) {
			n = len(allVids)
		}
		allVids = allVids[:n]
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "#\tDATE\tPACK\tSOURCE\tTITLE")
		for i, v := range allVids {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
				i+1, v.Published.Format("2006-01-02"), v.PackID, v.SourceID, v.Title)
		}
		w.Flush()
		return nil
	},
}

var videosSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T(i18n.ActiveLang, "videos.search.short"),
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
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		var allVids []content.Video
		for _, p := range packs {
			vids, _ := videos.ResolveAll(p.YouTubeSources, paths.CacheDir)
			allVids = append(allVids, vids...)
		}
		matched := videos.FilterVideos(allVids, args[0])
		if len(matched) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "videos.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DATE\tPACK\tSOURCE\tTITLE\tURL")
		for _, v := range matched {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				v.Published.Format("2006-01-02"), v.PackID, v.SourceID, v.Title, v.URL)
		}
		w.Flush()
		return nil
	},
}

var videosOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: i18n.T(i18n.ActiveLang, "videos.open.short"),
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
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		var allVids []content.Video
		for _, p := range packs {
			vids, _ := videos.ResolveAll(p.YouTubeSources, paths.CacheDir)
			allVids = append(allVids, vids...)
		}
		sort.Slice(allVids, func(i, j int) bool {
			return allVids[i].Published.After(allVids[j].Published)
		})

		var target *content.Video
		if id, err := strconv.Atoi(args[0]); err == nil && id >= 1 && id <= len(allVids) {
			target = &allVids[id-1]
		} else {
			target = videos.FindVideo(allVids, args[0])
		}
		if target == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "videos.open.not_found", map[string]any{"ID": args[0]}))
		}
		if err := browser.OpenURL(target.URL); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "videos.open.browser_fail", map[string]any{"Err": err, "URL": target.URL}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "videos.open.opening", map[string]any{"Title": target.Title, "URL": target.URL}))
		return nil
	},
}

func init() {
	videosListCmd.Flags().IntVarP(&videosListN, "count", "n", 20, "number of videos to show")
	videosListCmd.Flags().StringVar(&videosListSource, "source", "", "filter by source ID")
	videosListCmd.Flags().StringVar(&videosListPack, "pack", "", "filter by pack ID")
	videosCmd.Flags().IntVarP(&videosListN, "count", "n", 20, "number of videos to show")
	videosCmd.Flags().StringVar(&videosListSource, "source", "", "filter by source ID")
	videosCmd.Flags().StringVar(&videosListPack, "pack", "", "filter by pack ID")
	videosCmd.AddCommand(videosListCmd, videosSearchCmd, videosOpenCmd)
	rootCmd.AddCommand(videosCmd)
}
