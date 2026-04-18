package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
	tutorialAll         bool
	tutorialPack        string
	tutorialLevel       string
	tutorialTags        string
	tutorialInteractive bool
	tutorialStep        int
)

type tutorialListItem struct {
	tutorials.TutorialMeta
	Featured bool
}

var tutorialCmd = &cobra.Command{
	Use:   "tutorial",
	Short: i18n.T("en", "tutorial.short"),
}

var tutorialListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("en", "tutorial.list.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var packs []*content.Pack

		if tutorialPack != "" || tutorialAll {
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
				return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "tutorial.list.no_profile"))
			}
			activeProfile, err := loader.FindProfile(profileCfg.ID)
			if err != nil {
				return err
			}
			if activeProfile == nil {
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "tutorial.list.profile_not_found", map[string]any{"ID": profileCfg.ID}))
			}
			packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
			if err != nil {
				return err
			}
		}

		var refs []content.TutorialRef
		if tutorialPack != "" {
			refs = content.FilterTutorialRefsByPack(packs, tutorialPack)
		} else {
			refs = content.FlattenTutorialRefs(packs)
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		index, err := tutorials.LoadIndex(paths.CacheDir)
		if err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tutorial.no_index"))
			return nil
		}

		// Build slug→meta lookup from index
		indexMap := make(map[string]*tutorials.TutorialMeta, len(index))
		for i := range index {
			indexMap[index[i].Slug] = &index[i]
		}

		// Join refs with index
		var items []tutorialListItem
		for _, ref := range refs {
			meta, ok := indexMap[ref.Slug]
			if !ok {
				continue
			}
			items = append(items, tutorialListItem{
				TutorialMeta: *meta,
				Featured:     ref.Featured,
			})
		}

		// Apply level filter
		if tutorialLevel != "" {
			var filtered []tutorialListItem
			for _, item := range items {
				if strings.EqualFold(item.Level, tutorialLevel) {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}

		// Apply tags filter
		if tutorialTags != "" {
			tags := strings.Split(tutorialTags, ",")
			needles := make([]string, len(tags))
			for i, t := range tags {
				needles[i] = strings.ToLower(strings.TrimSpace(t))
			}
			var filtered []tutorialListItem
			for _, item := range items {
				if matchesTutorialTags(item.Tags, needles) {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}

		if len(items) == 0 {
			if tutorialPack != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "tutorial.none_pack", map[string]any{"Pack": tutorialPack}))
			} else if tutorialTags != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "tutorial.none_tags", map[string]any{"Tags": tutorialTags}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tutorial.none"))
			}
			return nil
		}

		colSlug := i18n.T(i18n.ActiveLang, "tutorial.col_slug")
		colTitle := i18n.T(i18n.ActiveLang, "tutorial.col_title")
		colTime := i18n.T(i18n.ActiveLang, "tutorial.col_time")
		colLevel := i18n.T(i18n.ActiveLang, "tutorial.col_level")
		fmt.Printf("  %-38s %-45s %-6s %s\n", colSlug, colTitle, colTime, colLevel)
		fmt.Println(strings.Repeat("-", 100))
		for _, item := range items {
			marker := "  "
			if item.Featured {
				marker = "★ "
			}
			timeStr := ""
			if item.Time > 0 {
				timeStr = fmt.Sprintf("%dm", item.Time)
			}
			fmt.Printf("%s%-38s %-45s %-6s %s\n", marker, item.Slug, truncateTutorial(item.Title, 44), timeStr, item.Level)
		}
		return nil
	},
}

var tutorialSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T("en", "tutorial.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}

		index, err := tutorials.LoadIndex(paths.CacheDir)
		if err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tutorial.no_index"))
			return nil
		}

		results := tutorials.Search(index, args[0])

		if tutorialLevel != "" {
			results = tutorials.FilterByLevel(results, tutorialLevel)
		}
		if tutorialTags != "" {
			tags := strings.Split(tutorialTags, ",")
			results = tutorials.FilterByTags(results, tags)
		}

		if len(results) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "tutorial.search.no_results", map[string]any{"Query": args[0]}))
			return nil
		}

		colSlug := i18n.T(i18n.ActiveLang, "tutorial.col_slug")
		colTitle := i18n.T(i18n.ActiveLang, "tutorial.col_title")
		colTime := i18n.T(i18n.ActiveLang, "tutorial.col_time")
		colLevel := i18n.T(i18n.ActiveLang, "tutorial.col_level")
		colPack := i18n.T(i18n.ActiveLang, "tutorial.col_pack")
		fmt.Printf("  %-38s %-35s %-6s %-14s %s\n", colSlug, colTitle, colTime, colLevel, colPack)
		fmt.Println(strings.Repeat("-", 110))
		for _, m := range results {
			timeStr := ""
			if m.Time > 0 {
				timeStr = fmt.Sprintf("%dm", m.Time)
			}
			fmt.Printf("  %-38s %-35s %-6s %-14s %s\n", m.Slug, truncateTutorial(m.Title, 34), timeStr, m.Level, m.Repo)
		}
		return nil
	},
}

var tutorialShowCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: i18n.T("en", "tutorial.show.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		index, err := tutorials.LoadIndex(paths.CacheDir)
		if err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tutorial.no_index"))
			return nil
		}

		meta := tutorials.FindBySlug(index, slug)
		if meta == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "tutorial.not_found", map[string]any{"Slug": slug}))
		}

		if tutorialInteractive {
			fmt.Fprintln(cmd.OutOrStdout(), "Interactive TUI mode coming in next task")
			return nil
		}

		// Try to load cached content
		tut, err := tutorials.LoadContent(paths.CacheDir, slug)
		if err != nil {
			// Not cached — fetch and parse
			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				token = os.Getenv("GH_TOKEN")
			}

			// Determine branch from cached repo info
			branch := "main"
			repos, repoErr := tutorials.LoadRepoInfo(paths.CacheDir)
			if repoErr == nil {
				for _, r := range repos {
					if r.Name == meta.Repo {
						branch = r.DefaultBranch
						break
					}
				}
			}

			client := tutorials.NewClient(tutorials.ClientConfig{Token: token})
			raw, fetchErr := client.FetchRawMarkdown(meta.Repo, branch, slug)
			if fetchErr != nil {
				return fmt.Errorf("fetch tutorial: %w", fetchErr)
			}

			tut, err = tutorials.Parse(raw, slug, meta.Repo)
			if err != nil {
				return fmt.Errorf("parse tutorial: %w", err)
			}

			// Cache for next time (best-effort)
			_ = tutorials.SaveContent(paths.CacheDir, tut)
		}

		// Build markdown output
		var md strings.Builder
		md.WriteString(fmt.Sprintf("# %s\n\n", tut.Title))
		md.WriteString(fmt.Sprintf("**Time:** %dm | **Level:** %s | **URL:** %s\n\n", tut.Time, tut.Level, tut.URL))
		md.WriteString("---\n\n")

		if tut.Prerequisites != "" {
			md.WriteString("## Prerequisites\n\n")
			md.WriteString(tut.Prerequisites)
			md.WriteString("\n\n")
		}

		if len(tut.YouWillLearn) > 0 {
			md.WriteString("## You will learn\n\n")
			for _, item := range tut.YouWillLearn {
				md.WriteString(fmt.Sprintf("- %s\n", item))
			}
			md.WriteString("\n---\n\n")
		}

		for _, step := range tut.Steps {
			md.WriteString(fmt.Sprintf("## Step %d: %s\n\n", step.Number, step.Title))
			md.WriteString(step.Content)
			md.WriteString("\n\n---\n\n")
		}

		rendered, err := glamour.Render(md.String(), "dark")
		if err != nil {
			// Fall back to raw markdown
			fmt.Fprint(cmd.OutOrStdout(), md.String())
			return nil
		}
		fmt.Fprint(cmd.OutOrStdout(), rendered)
		return nil
	},
}

var tutorialOpenCmd = &cobra.Command{
	Use:   "open <slug>",
	Short: i18n.T("en", "tutorial.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		url := fmt.Sprintf("https://developers.sap.com/tutorials/%s.html", slug)
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "tutorial.open.browser_fail", map[string]any{"Err": err, "URL": url}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "tutorial.open.opening", map[string]any{"Title": slug, "URL": url}))
		return nil
	},
}

func matchesTutorialTags(tags, needles []string) bool {
	for _, t := range tags {
		lower := strings.ToLower(t)
		for _, n := range needles {
			if strings.Contains(lower, n) {
				return true
			}
		}
	}
	return false
}

func truncateTutorial(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func init() {
	tutorialListCmd.Flags().BoolVarP(&tutorialAll, "all", "a", false, "show all tutorials regardless of profile")
	tutorialListCmd.Flags().StringVarP(&tutorialPack, "pack", "p", "", "filter to a specific pack")
	tutorialListCmd.Flags().StringVarP(&tutorialLevel, "level", "l", "", "filter by level (beginner, intermediate, advanced)")
	tutorialListCmd.Flags().StringVarP(&tutorialTags, "tags", "t", "", "comma-separated tags (OR match)")
	tutorialSearchCmd.Flags().StringVarP(&tutorialLevel, "level", "l", "", "filter by level")
	tutorialSearchCmd.Flags().StringVarP(&tutorialTags, "tags", "t", "", "comma-separated tags")
	tutorialShowCmd.Flags().BoolVarP(&tutorialInteractive, "interactive", "i", false, "interactive step-by-step mode")
	tutorialShowCmd.Flags().IntVar(&tutorialStep, "step", 0, "jump to a specific step number")
	tutorialCmd.AddCommand(tutorialListCmd, tutorialSearchCmd, tutorialShowCmd, tutorialOpenCmd)
	rootCmd.AddCommand(tutorialCmd)
}
