package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/discovery"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/learn"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var (
	learnLevel string
	learnAll   bool
	learnCount int
	learnPack  string
)

var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: i18n.T(i18n.ActiveLang, "learn.short"),
	Long:  i18n.T(i18n.ActiveLang, "learn.long"),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
			return err
		}
		if learnLevel != "" {
			switch learnLevel {
			case "beginner", "intermediate", "advanced":
			default:
				return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "learn.invalid_level", map[string]any{"Level": learnLevel}))
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return learnRecommendCmd.RunE(cmd, args)
	},
}

var learnRecommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: i18n.T(i18n.ActiveLang, "learn.recommend.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		packs, err := loadLearnPacks(paths, loader)
		if err != nil {
			return err
		}

		journeys, tuts, missions, anyLoaded := loadLearnIndexes(paths, cmd)
		if !anyLoaded {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "learn.no_content"))
		}

		level := effectiveLevel(paths)
		opts := learn.RecommendOptions{
			Level:  level,
			PackID: learnPack,
			All:    learnAll,
			Limit:  learnCount,
		}

		recs := learn.Recommend(journeys, tuts, missions, packs, opts)

		printed := false
		if len(recs.Journeys) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.section_journeys"))
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
					i18n.T(i18n.ActiveLang, "learn.col_featured"),
					i18n.T(i18n.ActiveLang, "learn.col_title"),
					i18n.T(i18n.ActiveLang, "learn.col_level"),
					i18n.T(i18n.ActiveLang, "learn.col_duration"))
			for _, item := range recs.Journeys {
				feat := ""
				if item.Featured {
					feat = "★"
				}
				title := truncate(item.Title, 55)
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", feat, title, titleCaseLevel(item.Level), item.Duration)
			}
			w.Flush()
			printed = true
		}

		if len(recs.Tutorials) > 0 {
			if printed {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.section_tutorials"))
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
					i18n.T(i18n.ActiveLang, "learn.col_featured"),
					i18n.T(i18n.ActiveLang, "learn.col_title"),
					i18n.T(i18n.ActiveLang, "learn.col_level"),
					i18n.T(i18n.ActiveLang, "learn.col_time"))
			for _, item := range recs.Tutorials {
				feat := ""
				if item.Featured {
					feat = "★"
				}
				title := truncate(item.Title, 55)
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", feat, title, titleCaseLevel(item.Level), item.Duration)
			}
			w.Flush()
			printed = true
		}

		if len(recs.Missions) > 0 {
			if printed {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.section_missions"))
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "  %s\t%s\t%s\n",
					i18n.T(i18n.ActiveLang, "learn.col_featured"),
					i18n.T(i18n.ActiveLang, "learn.col_title"),
					i18n.T(i18n.ActiveLang, "learn.col_effort"))
			for _, item := range recs.Missions {
				feat := ""
				if item.Featured {
					feat = "★"
				}
				title := truncate(item.Title, 55)
				fmt.Fprintf(w, "  %s\t%s\t%s\n", feat, title, effortLabel(item.Level))
			}
			w.Flush()
		}

		if !printed && len(recs.Missions) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.no_results"))
		}

		return nil
	},
}

func loadLearnPacks(paths *xdg.Paths, loader *content.ContentLoader) ([]*content.Pack, error) {
	if learnAll {
		return loader.LoadPacks(nil, i18n.ActiveLang)
	}
	profileCfg, err := config.LoadProfile(paths.ConfigDir)
	if err == nil && profileCfg.ID != "" {
		if p, _ := loader.FindProfile(profileCfg.ID); p != nil {
			return loader.LoadPacks(p, i18n.ActiveLang)
		}
	}
	return loader.LoadPacks(nil, i18n.ActiveLang)
}

func loadLearnIndexes(paths *xdg.Paths, cmd *cobra.Command) (
	[]learning.LearningJourney,
	[]tutorials.TutorialMeta,
	[]discovery.Mission,
	bool,
) {
	var journeys []learning.LearningJourney
	var tuts []tutorials.TutorialMeta
	var missions []discovery.Mission
	anyLoaded := false

	if j, ok := learning.LoadIndex(paths.CacheDir, learning.CacheTTL); ok {
		journeys = j
		anyLoaded = true
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf(i18n.ActiveLang, "learn.hint_sync", map[string]any{"Type": "learning journeys"}))
	}

	if t, err := tutorials.LoadIndex(paths.CacheDir); err == nil {
		tuts = t
		anyLoaded = true
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf(i18n.ActiveLang, "learn.hint_sync", map[string]any{"Type": "tutorials"}))
	}

	if m, ok := discovery.LoadCache[[]discovery.Mission](paths.CacheDir, "missions", discovery.CacheTTL); ok {
		missions = m
		anyLoaded = true
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf(i18n.ActiveLang, "learn.hint_sync", map[string]any{"Type": "missions"}))
	}

	return journeys, tuts, missions, anyLoaded
}

func effectiveLevel(paths *xdg.Paths) string {
	if learnLevel != "" {
		return learnLevel
	}
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil {
		return ""
	}
	return cfg.ExperienceLevel
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func titleCaseLevel(level string) string {
	switch level {
	case "beginner":
		return i18n.T(i18n.ActiveLang, "learn.level.beginner")
	case "intermediate":
		return i18n.T(i18n.ActiveLang, "learn.level.intermediate")
	case "advanced":
		return i18n.T(i18n.ActiveLang, "learn.level.advanced")
	default:
		return level
	}
}

func effortLabel(level string) string {
	switch level {
	case "beginner":
		return i18n.T(i18n.ActiveLang, "learn.effort.easy")
	case "intermediate":
		return i18n.T(i18n.ActiveLang, "learn.effort.medium")
	case "advanced":
		return i18n.T(i18n.ActiveLang, "learn.effort.hard")
	default:
		return level
	}
}

func init() {
	learnCmd.PersistentFlags().StringVar(&learnLevel, "level", "", "filter by level (beginner, intermediate, advanced)")
	learnCmd.PersistentFlags().BoolVar(&learnAll, "all", false, "bypass profile filtering")
	learnCmd.PersistentFlags().IntVarP(&learnCount, "count", "n", 10, "limit results per section")

	learnRecommendCmd.Flags().StringVar(&learnPack, "pack", "", "filter to a specific pack")

	learnCmd.AddCommand(learnRecommendCmd)
	rootCmd.AddCommand(learnCmd)
}
