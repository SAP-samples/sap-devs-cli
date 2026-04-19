package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/community"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/news"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/youtube"
)

const (
	newsPlaylistRSS  = "https://www.youtube.com/feeds/videos.xml?playlist_id=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg"
	newsPlaylistURL  = "https://www.youtube.com/playlist?list=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg"
	newsCommunityRSS = "https://community.sap.com/t5/developer-news/bg-p/developer-news/rss"
	newsLinkedIn     = "https://www.linkedin.com/newsletters/sap-developer-news-7155319074263044096/"
	newsYTMusic      = "" // footer line is suppressed when empty
)

var newsCmd = &cobra.Command{
	Use:   "news",
	Short: i18n.T("en", "news.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return newsListCmd.RunE(cmd, args)
	},
}

var newsListN int

var newsListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("en", "news.list.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
		if err != nil {
			return fmt.Errorf("%s: %w", i18n.T(i18n.ActiveLang, "news.error.fetch"), err)
		}
		posts, _ := community.FetchBlogPosts(newsCommunityRSS) // failure is silent
		items := news.Correlate(episodes, posts)

		n := newsListN
		if n <= 0 || n > len(items) {
			n = len(items)
		}
		items = items[:n]

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			i18n.T(i18n.ActiveLang, "news.col_num"),
			i18n.T(i18n.ActiveLang, "news.col_date"),
			i18n.T(i18n.ActiveLang, "news.col_title"),
			i18n.T(i18n.ActiveLang, "news.col_video"),
			i18n.T(i18n.ActiveLang, "news.col_community"))
		for i, item := range items {
			com := "--"
			if item.Community != nil {
				com = "[com]"
			}
			date := item.Episode.Published.Format("2006-01-02")
			fmt.Fprintf(w, "%d\t%s\t%s\t[yt]\t%s\n", i+1, date, item.Episode.Title, com)
		}
		w.Flush()
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "news.list.linkedin", map[string]any{"URL": newsLinkedIn}))
		if newsYTMusic != "" {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "news.list.youtube_music", map[string]any{"URL": newsYTMusic}))
		}
		return nil
	},
}

var newsLatestCmd = &cobra.Command{
	Use:   "latest",
	Short: i18n.T("en", "news.latest.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
		if err != nil {
			return fmt.Errorf("%s: %w", i18n.T(i18n.ActiveLang, "news.error.fetch"), err)
		}
		if len(episodes) == 0 {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "news.error.no_episodes"))
		}
		if err := browser.OpenURL(episodes[0].URL); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), episodes[0].URL)
		}
		return nil
	},
}

var newsOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: i18n.T("en", "news.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil || id < 1 {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "news.error.invalid_id"))
		}
		episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
		if err != nil {
			return fmt.Errorf("%s: %w", i18n.T(i18n.ActiveLang, "news.error.fetch"), err)
		}
		if id > len(episodes) {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "news.error.id_range", map[string]any{"ID": id, "Count": len(episodes)}))
		}
		ep := episodes[id-1]
		if err := browser.OpenURL(ep.URL); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), ep.URL)
		}
		return nil
	},
}

var newsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T("en", "news.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q := strings.ToLower(args[0])
		episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
		if err != nil {
			return fmt.Errorf("%s: %w", i18n.T(i18n.ActiveLang, "news.error.fetch"), err)
		}
		var matched []youtube.Episode
		for _, ep := range episodes {
			if strings.Contains(strings.ToLower(ep.Title), q) ||
				strings.Contains(strings.ToLower(ep.Description), q) {
				matched = append(matched, ep)
			}
		}
		if len(matched) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "news.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			i18n.T(i18n.ActiveLang, "news.col_date"),
			i18n.T(i18n.ActiveLang, "news.col_title"),
			i18n.T(i18n.ActiveLang, "news.col_url"))
		for _, ep := range matched {
			fmt.Fprintf(w, "%s\t%s\t%s\n", ep.Published.Format("2006-01-02"), ep.Title, ep.URL)
		}
		w.Flush()
		return nil
	},
}

var newsReadPlain bool

var newsReadCmd = &cobra.Command{
	Use:   "read <id>",
	Short: i18n.T("en", "news.read.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil || id < 1 {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "news.error.invalid_id"))
		}
		episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
		if err != nil {
			return fmt.Errorf("%s: %w", i18n.T(i18n.ActiveLang, "news.error.fetch"), err)
		}
		if id > len(episodes) {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "news.error.id_range", map[string]any{"ID": id, "Count": len(episodes)}))
		}
		posts, err := community.FetchBlogPosts(newsCommunityRSS)
		if err != nil {
			return fmt.Errorf("%s: %w", i18n.T(i18n.ActiveLang, "news.error.fetch_community"), err)
		}
		items := news.Correlate(episodes, posts)
		item := items[id-1]
		if item.Community == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "news.error.no_community_post", map[string]any{"ID": id}))
		}
		content, err := community.FetchPostContent(item.Community.URL)
		if err != nil {
			return fmt.Errorf("%s: %w", i18n.T(i18n.ActiveLang, "news.error.fetch_content"), err)
		}
		if newsReadPlain {
			fmt.Fprintln(cmd.OutOrStdout(), content)
			return nil
		}
		return openPager(cmd.OutOrStdout(), content)
	},
}

// openPager displays content via $PAGER, less (if available), or plain print.
// Uses os.Stdout/Stderr directly so the pager gets a real TTY descriptor.
// w is used only for the no-pager fallback path.
func openPager(w io.Writer, content string) error {
	var pagerArgs []string
	pager := os.Getenv("PAGER")
	if pager != "" {
		parts := strings.Fields(pager)
		pager = parts[0]
		pagerArgs = parts[1:]
	} else if _, err := exec.LookPath("less"); err == nil {
		pager = "less"
	}
	if pager == "" {
		fmt.Fprint(w, content)
		return nil
	}
	c := exec.Command(pager, pagerArgs...) //nolint:gosec // pager comes from env or LookPath
	c.Stdin = strings.NewReader(content)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func fridayHookMessage(day time.Weekday) string {
	if day != time.Friday {
		return ""
	}
	return i18n.T(i18n.ActiveLang, "news.hook.friday_message")
}

var newsHookCmd = &cobra.Command{
	Use:    "hook",
	Short:  i18n.T("en", "news.hook.short"),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if msg := fridayHookMessage(time.Now().Weekday()); msg != "" {
			fmt.Fprintln(cmd.OutOrStdout(), msg)
		}
		return nil
	},
}

func init() {
	newsCmd.Flags().IntVarP(&newsListN, "count", "n", 10, "number of episodes to show")
	newsListCmd.Flags().IntVarP(&newsListN, "count", "n", 10, "number of episodes to show")
	newsReadCmd.Flags().BoolVar(&newsReadPlain, "plain", false, "print plain text to stdout")
	newsCmd.AddCommand(newsListCmd, newsLatestCmd, newsOpenCmd, newsSearchCmd, newsReadCmd, newsHookCmd)
	rootCmd.AddCommand(newsCmd)
}
