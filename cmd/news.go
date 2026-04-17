package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/community"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/news"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/youtube"
)

const (
	newsPlaylistRSS  = "https://www.youtube.com/feeds/videos.xml?playlist_id=PL6RpkC85SLQAVBSQXN9522_1jNvPavBgg"
	newsCommunityRSS = "https://community.sap.com/t5/developer-news/bg-p/developer-news/rss"
	newsLinkedIn     = "https://www.linkedin.com/newsletters/sap-developer-news-7155319074263044096/"
	newsYTMusic      = "" // fill in with YouTube Music podcast URL before shipping
)

var newsCmd = &cobra.Command{
	Use:   "news",
	Short: "Browse SAP Developer News episodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		return newsListCmd.RunE(cmd, args)
	},
}

var newsListN int

var newsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent SAP Developer News episodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
		if err != nil {
			return fmt.Errorf("could not fetch SAP Developer News: %w", err)
		}
		posts, _ := community.FetchBlogPosts(newsCommunityRSS) // failure is silent
		items := news.Correlate(episodes, posts)

		n := newsListN
		if n <= 0 || n > len(items) {
			n = len(items)
		}
		items = items[:n]

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "#\tDATE\tTITLE\tVIDEO\tCOMMUNITY")
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
		fmt.Fprintf(cmd.OutOrStdout(), "LinkedIn newsletter: %s\n", newsLinkedIn)
		if newsYTMusic != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Listen on YouTube Music: %s\n", newsYTMusic)
		}
		return nil
	},
}

var newsLatestCmd = &cobra.Command{
	Use:   "latest",
	Short: "Open the most recent SAP Developer News episode in the browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
		if err != nil {
			return fmt.Errorf("could not fetch SAP Developer News: %w", err)
		}
		if len(episodes) == 0 {
			return fmt.Errorf("no episodes found")
		}
		if err := browser.OpenURL(episodes[0].URL); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), episodes[0].URL)
		}
		return nil
	},
}

var newsOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: "Open a specific episode in the browser",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil || id < 1 {
			return fmt.Errorf("id must be a positive integer (run 'news list' to see IDs)")
		}
		episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
		if err != nil {
			return fmt.Errorf("could not fetch SAP Developer News: %w", err)
		}
		if id > len(episodes) {
			return fmt.Errorf("id %d out of range (only %d episodes available)", id, len(episodes))
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
	Short: "Search SAP Developer News episodes by title or description",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q := strings.ToLower(args[0])
		episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
		if err != nil {
			return fmt.Errorf("could not fetch SAP Developer News: %w", err)
		}
		var matched []youtube.Episode
		for _, ep := range episodes {
			if strings.Contains(strings.ToLower(ep.Title), q) ||
				strings.Contains(strings.ToLower(ep.Description), q) {
				matched = append(matched, ep)
			}
		}
		if len(matched) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No episodes found matching %q\n", args[0])
			return nil
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DATE\tTITLE\tURL")
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
	Short: "Read the SAP Community blog post for an episode",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil || id < 1 {
			return fmt.Errorf("id must be a positive integer (run 'news list' to see IDs)")
		}
		episodes, err := youtube.FetchPlaylist(newsPlaylistRSS)
		if err != nil {
			return fmt.Errorf("could not fetch SAP Developer News: %w", err)
		}
		if id > len(episodes) {
			return fmt.Errorf("id %d out of range (only %d episodes available)", id, len(episodes))
		}
		posts, err := community.FetchBlogPosts(newsCommunityRSS)
		if err != nil {
			return fmt.Errorf("could not fetch Community posts: %w", err)
		}
		items := news.Correlate(episodes, posts)
		item := items[id-1]
		if item.Community == nil {
			return fmt.Errorf("no SAP Community post found for episode %d", id)
		}
		content, err := community.FetchPostContent(item.Community.URL)
		if err != nil {
			return fmt.Errorf("could not fetch post content: %w", err)
		}
		if newsReadPlain {
			fmt.Fprintln(cmd.OutOrStdout(), content)
			return nil
		}
		return openPager(content)
	},
}

// openPager displays content via $PAGER, less (if available), or plain print.
func openPager(content string) error {
	pager := os.Getenv("PAGER")
	if pager == "" {
		if _, err := exec.LookPath("less"); err == nil {
			pager = "less"
		}
	}
	if pager == "" {
		fmt.Print(content)
		return nil
	}
	c := exec.Command(pager) //nolint:gosec // pager comes from env or LookPath
	c.Stdin = strings.NewReader(content)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func init() {
	newsListCmd.Flags().IntVarP(&newsListN, "count", "n", 10, "number of episodes to show")
	newsReadCmd.Flags().BoolVar(&newsReadPlain, "plain", false, "print plain text to stdout")
	newsCmd.AddCommand(newsListCmd, newsLatestCmd, newsOpenCmd, newsSearchCmd, newsReadCmd)
	rootCmd.AddCommand(newsCmd)
}
