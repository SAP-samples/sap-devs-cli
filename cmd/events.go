package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/events"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/geo"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/notify"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
	eventsAll   bool
	eventsType  string
	eventsCount int
)

var (
	eventsExportType   string
	eventsExportOutput string
	eventsExportFormat string
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Browse upcoming SAP community events",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
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

		eventTypes := content.FlattenEventTypes(packs)
		var allEvents []content.EventInstance

		for _, et := range eventTypes {
			if et.Source == "rss" {
				resolved, _ := events.Resolve(et, paths.CacheDir, false)
				allEvents = append(allEvents, resolved...)
			}
		}

		manual := content.FlattenEventInstances(packs)
		allEvents = events.MergeAndSort(allEvents, manual)

		if eventsType != "" {
			allEvents = content.FilterEventsByType(allEvents, eventsType)
		}

		if !eventsAll && cfg.Location != "" {
			userLat, userLon, ok := geo.Lookup(cfg.Location)
			if ok {
				allEvents = events.FilterByLocation(allEvents, userLat, userLon,
					cfg.Events.EffectiveLocalRadius(), cfg.Events.EffectiveRegionalRadius())
			}
		}

		if len(allEvents) == 0 {
			if eventsType != "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.none_type", map[string]any{"Type": eventsType}))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "events.none"))
			}
			return nil
		}

		if eventsCount > 0 && len(allEvents) > eventsCount {
			allEvents = allEvents[:eventsCount]
		}

		printEventTable(cmd, allEvents)
		return nil
	},
}

var eventsOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: "Open an event URL in the browser",
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

		eventTypes := content.FlattenEventTypes(packs)
		var allEvents []content.EventInstance
		for _, et := range eventTypes {
			if et.Source == "rss" {
				resolved, _ := events.Resolve(et, paths.CacheDir, false)
				allEvents = append(allEvents, resolved...)
			}
		}
		manual := content.FlattenEventInstances(packs)
		allEvents = events.MergeAndSort(allEvents, manual)

		e := content.FindEvent(allEvents, args[0])
		if e == nil {
			return fmt.Errorf("%s", i18n.Tf(i18n.ActiveLang, "events.not_found", map[string]any{"ID": args[0]}))
		}
		if err := browser.OpenURL(e.URL); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.open.browser_fail", map[string]any{"Err": err, "URL": e.URL}))
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.open.opening", map[string]any{"Title": e.Title, "URL": e.URL}))
		return nil
	},
}

var eventsTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List available event types",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		packs, err := loader.LoadPacks(nil, i18n.ActiveLang)
		if err != nil {
			return err
		}
		types := content.FlattenEventTypes(packs)
		if len(types) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "events.types.none"))
			return nil
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			i18n.T(i18n.ActiveLang, "events.types.col_id"),
			i18n.T(i18n.ActiveLang, "events.types.col_source"),
			i18n.T(i18n.ActiveLang, "events.types.col_name"),
		)
		for _, et := range types {
			fmt.Fprintf(w, "%s\t%s\t%s\n", et.ID, et.Source, et.Name)
		}
		w.Flush()
		return nil
	},
}

var eventsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export events to a calendar file or generate calendar URLs",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
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

		eventTypes := content.FlattenEventTypes(packs)
		var allEvents []content.EventInstance
		for _, et := range eventTypes {
			if et.Source == "rss" {
				resolved, _ := events.Resolve(et, paths.CacheDir, false)
				allEvents = append(allEvents, resolved...)
			}
		}
		manual := content.FlattenEventInstances(packs)
		allEvents = events.MergeAndSort(allEvents, manual)

		if eventsExportType != "" {
			allEvents = content.FilterEventsByType(allEvents, eventsExportType)
		}

		if !eventsAll && cfg.Location != "" {
			userLat, userLon, ok := geo.Lookup(cfg.Location)
			if ok {
				allEvents = events.FilterByLocation(allEvents, userLat, userLon,
					cfg.Events.EffectiveLocalRadius(), cfg.Events.EffectiveRegionalRadius())
			}
		}

		if len(allEvents) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "events.export.none"))
			return nil
		}

		format := eventsExportFormat
		if format == "" {
			format = "ics"
		}

		switch format {
		case "google":
			for _, e := range allEvents {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n  %s\n\n", e.Title, events.GoogleCalendarURL(e))
			}
			return nil
		case "outlook":
			for _, e := range allEvents {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n  %s\n\n", e.Title, events.OutlookWebURL(e))
			}
			return nil
		case "ics", "vcs":
			outPath := eventsExportOutput
			if outPath == "" {
				outPath = "sap-devs-events." + format
			}
			f, err := os.Create(outPath)
			if err != nil {
				return err
			}
			defer f.Close()

			if format == "vcs" {
				err = events.ExportVCS(allEvents, f)
			} else {
				err = events.ExportICS(allEvents, f)
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.export.done", map[string]any{"Count": len(allEvents), "Path": outPath}))
			return nil
		default:
			return fmt.Errorf("unknown format %q: must be ics, vcs, google, or outlook", format)
		}
	},
}

var eventsHookCmd = &cobra.Command{
	Use:    "hook",
	Short:  "Print upcoming event reminders (session-start hook)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
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

		eventTypes := content.FlattenEventTypes(packs)
		var allEvents []content.EventInstance
		for _, et := range eventTypes {
			if et.Source == "rss" {
				resolved, _ := events.Resolve(et, paths.CacheDir, false)
				allEvents = append(allEvents, resolved...)
			}
		}
		manual := content.FlattenEventInstances(packs)
		allEvents = events.MergeAndSort(allEvents, manual)

		upcoming := events.CheckUpcoming(allEvents, cfg.Events.EffectiveNotifyDays())
		if msg := events.FormatHookMessage(upcoming); msg != "" {
			fmt.Fprintln(cmd.OutOrStdout(), msg)
		}
		return nil
	},
}

var eventsNotifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Send OS notification for upcoming events",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
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

		eventTypes := content.FlattenEventTypes(packs)
		var allEvents []content.EventInstance
		for _, et := range eventTypes {
			if et.Source == "rss" {
				resolved, _ := events.Resolve(et, paths.CacheDir, false)
				allEvents = append(allEvents, resolved...)
			}
		}
		manual := content.FlattenEventInstances(packs)
		allEvents = events.MergeAndSort(allEvents, manual)

		days := cfg.Events.EffectiveNotifyDays()
		upcoming := events.CheckUpcoming(allEvents, days)
		if len(upcoming) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.notify.none", map[string]any{"Days": days}))
			return nil
		}

		msg := events.FormatHookMessage(upcoming)

		if !notify.Available() {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "events.notify.unsupported"))
			fmt.Fprintln(cmd.OutOrStdout(), msg)
			return nil
		}

		title := fmt.Sprintf("SAP Events: %d upcoming", len(upcoming))
		if err := notify.Send(title, msg); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), msg)
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "events.notify.sent", map[string]any{"Count": len(upcoming)}))
		return nil
	},
}

func printEventTable(cmd *cobra.Command, evts []content.EventInstance) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		i18n.T(i18n.ActiveLang, "events.col_date"),
		i18n.T(i18n.ActiveLang, "events.col_type"),
		i18n.T(i18n.ActiveLang, "events.col_scope"),
		i18n.T(i18n.ActiveLang, "events.col_location"),
		i18n.T(i18n.ActiveLang, "events.col_title"),
	)
	for _, e := range evts {
		date := e.DateStr
		if len(date) > 10 {
			date = date[:10]
		}
		loc := e.Location
		if loc == "" {
			loc = "virtual"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", date, e.Type, e.Scope, loc, e.Title)
	}
	w.Flush()
}

func init() {
	eventsCmd.Flags().BoolVarP(&eventsAll, "all", "a", false, "show all events regardless of location")
	eventsCmd.Flags().StringVarP(&eventsType, "type", "t", "", "filter by event type ID")
	eventsCmd.Flags().IntVarP(&eventsCount, "count", "n", 10, "max events to display")
	eventsExportCmd.Flags().StringVarP(&eventsExportType, "type", "t", "", "filter by event type ID")
	eventsExportCmd.Flags().StringVarP(&eventsExportOutput, "output", "o", "", "output file path (default: sap-devs-events.ics)")
	eventsExportCmd.Flags().StringVarP(&eventsExportFormat, "format", "f", "ics", "export format: ics, vcs, google, outlook")
	eventsExportCmd.Flags().BoolVarP(&eventsAll, "all", "a", false, "export all events regardless of location")
	eventsCmd.AddCommand(eventsOpenCmd, eventsTypesCmd, eventsExportCmd, eventsHookCmd, eventsNotifyCmd)
	rootCmd.AddCommand(eventsCmd)
}
