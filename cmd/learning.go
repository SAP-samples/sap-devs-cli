package cmd

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/glamour"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var learningCmd = &cobra.Command{
	Use:   "learning",
	Short: i18n.T(i18n.ActiveLang, "learning.short"),
	Long:  i18n.T(i18n.ActiveLang, "learning.long"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return learningListCmd.RunE(cmd, args)
	},
}

var learningListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T(i18n.ActiveLang, "learning.list.short"),
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
		if !learningAll {
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

		index, ok := learning.LoadIndex(paths.CacheDir, learning.CacheTTL)
		if !ok {
			return fmt.Errorf("learning index not cached — run 'sap-devs sync' first")
		}

		refs := content.FlattenLearningRefs(packs)
		filters := content.LearningProfileFilters{}
		if !learningAll {
			filters = content.CollectLearningFilters(packs)
		}

		journeys := resolveLearningJourneys(index, refs, filters, learningAll)

		if learningPack != "" {
			journeys = filterLearningByPack(journeys, refs, learningPack)
		}
		if learningLevel != "" {
			journeys = learning.FilterByLevel(journeys, learningLevel)
		}
		if learningRole != "" {
			journeys = learning.FilterByRole(journeys, learningRole)
		}

		if len(journeys) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No learning journeys found.")
			return nil
		}

		n := learningCount
		if n <= 0 || n > len(journeys) {
			n = len(journeys)
		}
		journeys = journeys[:n]

		featuredSlugs := make(map[string]bool)
		for _, ref := range refs {
			if ref.Featured {
				featuredSlugs[ref.Slug] = true
			}
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "FEATURED", "TITLE", "LEVEL", "DURATION")
		for _, j := range journeys {
			featured := ""
			if featuredSlugs[j.Slug] {
				featured = "★"
			}
			level := formatLevel(j.Level)
			duration := formatDuration(j.DurationHours)
			title := j.Title
			if len(title) > 55 {
				title = title[:52] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", featured, title, level, duration)
		}
		w.Flush()
		return nil
	},
}

var learningSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T(i18n.ActiveLang, "learning.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}

		cacheKey := learning.SearchCacheKey(args[0], learningLevel, learningRole)
		var results []learning.LearningJourney

		results, ok := learning.LoadSearchCache(paths.CacheDir, cacheKey)
		if !ok {
			results, err = learning.SearchAPI(args[0], learningCount)
			if err != nil {
				// Fallback to local search
				index, indexOK := learning.LoadIndex(paths.CacheDir, learning.CacheTTL)
				if !indexOK {
					return fmt.Errorf("search API failed and no cached index: %w", err)
				}
				results = learning.Search(index, args[0])
			} else {
				_ = learning.SaveSearchCache(paths.CacheDir, cacheKey, results)
			}
		}

		if learningLevel != "" {
			results = learning.FilterByLevel(results, learningLevel)
		}
		if learningRole != "" {
			results = learning.FilterByRole(results, learningRole)
		}

		if len(results) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No learning journeys found for %q.\n", args[0])
			return nil
		}

		n := learningCount
		if n <= 0 || n > len(results) {
			n = len(results)
		}
		results = results[:n]

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "#", "TITLE", "LEVEL", "DURATION")
		for i, j := range results {
			level := formatLevel(j.Level)
			duration := formatDuration(j.DurationHours)
			title := j.Title
			if len(title) > 55 {
				title = title[:52] + "..."
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i+1, title, level, duration)
		}
		w.Flush()
		return nil
	},
}

var learningShowCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: i18n.T(i18n.ActiveLang, "learning.show.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}

		index, ok := learning.LoadIndex(paths.CacheDir, learning.CacheTTL)
		if !ok {
			return fmt.Errorf("learning index not cached — run 'sap-devs sync' first")
		}

		j := learning.FindBySlug(index, args[0])
		if j == nil {
			return fmt.Errorf("learning journey %q not found", args[0])
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("# %s\n\n", j.Title))
		b.WriteString(fmt.Sprintf("**Level:** %s | **Duration:** %s | **Product:** %s\n\n",
			formatLevel(j.Level), formatDuration(j.DurationHours), j.Product))
		if len(j.Roles) > 0 {
			b.WriteString(fmt.Sprintf("**Roles:** %s\n\n", strings.Join(j.Roles, ", ")))
		}
		if j.Description != "" {
			b.WriteString("## Description\n\n")
			b.WriteString(j.Description + "\n\n")
		}
		if j.Objectives != "" {
			b.WriteString("## Learning Objectives\n\n")
			b.WriteString(htmlToMarkdown(j.Objectives) + "\n\n")
		}
		b.WriteString(fmt.Sprintf("**URL:** %s\n", j.URL))

		renderer, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(80))
		if err != nil {
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		}
		rendered, err := renderer.Render(b.String())
		if err != nil {
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		}
		fmt.Fprint(cmd.OutOrStdout(), rendered)
		return nil
	},
}

var learningOpenCmd = &cobra.Command{
	Use:   "open <slug>",
	Short: i18n.T(i18n.ActiveLang, "learning.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := learning.BaseURL + args[0]
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Could not open browser. Visit: %s\n", url)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Opening %s\n", url)
		return nil
	},
}

// Flags
var (
	learningAll   bool
	learningPack  string
	learningLevel string
	learningRole  string
	learningCount int
)

func init() {
	learningCmd.PersistentFlags().BoolVar(&learningAll, "all", false, "bypass profile filtering")
	learningCmd.PersistentFlags().StringVar(&learningLevel, "level", "", "filter by level (beginner, intermediate, advanced)")
	learningCmd.PersistentFlags().StringVar(&learningRole, "role", "", "filter by role")
	learningCmd.PersistentFlags().IntVarP(&learningCount, "count", "n", 20, "limit results")

	learningListCmd.Flags().StringVar(&learningPack, "pack", "", "filter to a specific pack's curated journeys")

	learningCmd.AddCommand(learningListCmd, learningSearchCmd, learningShowCmd, learningOpenCmd)
	rootCmd.AddCommand(learningCmd)
}

// resolveLearningJourneys implements the three-tier resolution algorithm.
func resolveLearningJourneys(
	index []learning.LearningJourney,
	refs []content.LearningRef,
	filters content.LearningProfileFilters,
	all bool,
) []learning.LearningJourney {
	bySlug := make(map[string]learning.LearningJourney, len(index))
	for _, j := range index {
		bySlug[j.Slug] = j
	}

	var result []learning.LearningJourney
	seen := make(map[string]bool)

	// Tier 1: featured refs
	for _, ref := range refs {
		if !ref.Featured {
			continue
		}
		if j, ok := bySlug[ref.Slug]; ok && !seen[ref.Slug] {
			result = append(result, j)
			seen[ref.Slug] = true
		}
	}

	// Tier 2: non-featured pack refs
	for _, ref := range refs {
		if ref.Featured || seen[ref.Slug] {
			continue
		}
		if j, ok := bySlug[ref.Slug]; ok {
			result = append(result, j)
			seen[ref.Slug] = true
		}
	}

	// Tier 3: profile-filtered (or all if --all)
	for _, j := range index {
		if seen[j.Slug] {
			continue
		}
		if all || content.MatchesLearningFilters(j.Product, j.ProductCategory, j.Roles, filters) {
			result = append(result, j)
			seen[j.Slug] = true
		}
	}

	return result
}

func filterLearningByPack(journeys []learning.LearningJourney, refs []content.LearningRef, packID string) []learning.LearningJourney {
	slugs := make(map[string]bool)
	for _, ref := range refs {
		if ref.PackID == packID {
			slugs[ref.Slug] = true
		}
	}
	var out []learning.LearningJourney
	for _, j := range journeys {
		if slugs[j.Slug] {
			out = append(out, j)
		}
	}
	return out
}

func formatLevel(level string) string {
	switch strings.ToUpper(level) {
	case "BEGINNER":
		return "Beginner"
	case "INTERMEDIATE":
		return "Intermediate"
	case "ADVANCED":
		return "Advanced"
	default:
		return level
	}
}

func formatDuration(hours string) string {
	if hours == "" {
		return ""
	}
	return hours + " hr"
}

var reHTMLTag = regexp.MustCompile(`<[^>]+>`)

func htmlToMarkdown(s string) string {
	s = strings.ReplaceAll(s, "<li>", "- ")
	s = strings.ReplaceAll(s, "</li>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = reHTMLTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(s)
}
