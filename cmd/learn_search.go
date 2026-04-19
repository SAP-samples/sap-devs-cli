package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learn"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var learnSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: i18n.T(i18n.ActiveLang, "learn.search.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}

		journeys, tuts, missions, anyLoaded := loadLearnIndexes(paths, cmd)
		if !anyLoaded {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "learn.no_content"))
		}

		level := effectiveLevel(paths)
		opts := learn.RecommendOptions{
			Level: level,
			All:   true,
			Limit: learnCount,
		}

		results := learn.Search(journeys, tuts, missions, args[0], opts)

		if len(results) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "learn.no_results"))
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", "#", "TYPE", "TITLE", "LEVEL", "TIME")
		for i, item := range results {
			typeName := typeLabel(item.Type)
			title := truncate(item.Title, 50)
			level := titleCaseLevel(item.Level)
			duration := item.Duration
			if duration == "" {
				duration = "-"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, typeName, title, level, duration)
		}
		w.Flush()
		return nil
	},
}

func typeLabel(t learn.ItemType) string {
	switch t {
	case learn.ItemJourney:
		return "Journey"
	case learn.ItemTutorial:
		return "Tutorial"
	case learn.ItemMission:
		return "Mission"
	default:
		return string(t)
	}
}

func init() {
	learnCmd.AddCommand(learnSearchCmd)
}
