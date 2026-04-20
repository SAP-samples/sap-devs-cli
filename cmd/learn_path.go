package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/glamour"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/learn"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var learnPathCmd = &cobra.Command{
	Use:   "path",
	Short: i18n.T(i18n.ActiveLang, "learn.path.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return learnPathListCmd.RunE(cmd, args)
	},
}

var learnPathListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T(i18n.ActiveLang, "learn.path.list.short"),
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

		journeys, tuts, missions, _ := loadLearnIndexes(paths, cmd)

		curated := learn.LoadPaths(packs)
		auto := learn.AutoFillPaths(packs, journeys, tuts, missions)

		allPaths := append(curated, auto...)

		level := effectiveLevel(paths)
		if level != "" {
			var filtered []learn.LearningPath
			for _, p := range allPaths {
				if p.Level == "" || p.Level == level {
					filtered = append(filtered, p)
				}
			}
			allPaths = filtered
		}

		if learnPack != "" {
			var filtered []learn.LearningPath
			for _, p := range allPaths {
				if p.PackID == learnPack {
					filtered = append(filtered, p)
				}
			}
			allPaths = filtered
		}

		if len(allPaths) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.no_results"))
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				i18n.T(i18n.ActiveLang, "learn.path.col_name"),
				i18n.T(i18n.ActiveLang, "learn.path.col_level"),
				i18n.T(i18n.ActiveLang, "learn.path.col_steps"),
				i18n.T(i18n.ActiveLang, "learn.path.col_pack"),
				i18n.T(i18n.ActiveLang, "learn.path.col_source"))
			for _, p := range allPaths {
				source := i18n.T(i18n.ActiveLang, "learn.path.source.curated")
				if p.Generated {
					source = i18n.T(i18n.ActiveLang, "learn.path.source.auto")
			}
			level := titleCaseLevel(p.Level)
			if level == "" {
				level = "-"
			}
			name := truncate(p.Name, 40)
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", name, level, len(p.Steps), p.PackID, source)
		}
		w.Flush()
		return nil
	},
}

var learnPathShowCmd = &cobra.Command{
	Use:   "show <path-id>",
	Short: i18n.T(i18n.ActiveLang, "learn.path.show.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		xdgPaths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		packs, err := loadLearnPacks(xdgPaths, loader)
		if err != nil {
			return err
		}

		journeys, tuts, missions, _ := loadLearnIndexes(xdgPaths, cmd)

		curated := learn.LoadPaths(packs)
		auto := learn.AutoFillPaths(packs, journeys, tuts, missions)
		allPaths := append(curated, auto...)
		allPaths = learn.ResolvePaths(allPaths, journeys, tuts, missions)

		p := learn.FindPath(allPaths, args[0])
		if p == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "learn.path_not_found", map[string]any{"ID": args[0]}))
		}

		source := i18n.T(i18n.ActiveLang, "learn.path.source.curated")
		if p.Generated {
			source = i18n.T(i18n.ActiveLang, "learn.path.source.auto")
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("# %s\n\n", p.Name))
		b.WriteString(fmt.Sprintf("**Level:** %s | **Pack:** %s | **Source:** %s\n\n", titleCaseLevel(p.Level), p.PackID, source))
		if p.Description != "" {
			b.WriteString(p.Description + "\n\n")
		}

		for i, step := range p.Steps {
			if step.Item != nil {
				b.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, typeLabel(step.Item.Type), step.Item.Title))
				if step.Item.Duration != "" {
					b.WriteString(fmt.Sprintf(" (%s)", step.Item.Duration))
				}
				b.WriteString("\n")
				if step.Item.URL != "" {
					b.WriteString(fmt.Sprintf("   %s\n", step.Item.URL))
				}
			} else {
				b.WriteString(fmt.Sprintf("%d. [%s] %s %s\n", i+1, string(step.Type), step.Slug, i18n.T(i18n.ActiveLang, "learn.step_not_found")))
			}
			b.WriteString("\n")
		}

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

var learnPathOpenCmd = &cobra.Command{
	Use:   "open <path-id>",
	Short: i18n.T(i18n.ActiveLang, "learn.path.open.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		xdgPaths, err := xdg.New()
		if err != nil {
			return err
		}
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		packs, err := loadLearnPacks(xdgPaths, loader)
		if err != nil {
			return err
		}

		journeys, tuts, missions, _ := loadLearnIndexes(xdgPaths, cmd)

		curated := learn.LoadPaths(packs)
		auto := learn.AutoFillPaths(packs, journeys, tuts, missions)
		allPaths := append(curated, auto...)
		allPaths = learn.ResolvePaths(allPaths, journeys, tuts, missions)

		p := learn.FindPath(allPaths, args[0])
		if p == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "learn.path_not_found", map[string]any{"ID": args[0]}))
		}

		for _, step := range p.Steps {
			if step.Item != nil && step.Item.URL != "" {
				if err := browser.OpenURL(step.Item.URL); err != nil {
						fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "learn.path.open.browser_fail", map[string]any{"URL": step.Item.URL}))
						return nil
					}
					fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "learn.path.open.opening", map[string]any{"URL": step.Item.URL}))
				return nil
			}
		}

		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.path.no_urls"))
		return nil
	},
}

func init() {
	learnPathListCmd.Flags().StringVar(&learnPack, "pack", "", "filter to a specific pack")
	learnPathCmd.AddCommand(learnPathListCmd, learnPathShowCmd, learnPathOpenCmd)
	learnCmd.AddCommand(learnPathCmd)
}
