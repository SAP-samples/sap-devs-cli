package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/dynamic"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/project"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/scratch"
	sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
	injectProject   bool
	injectTool      string
	injectDryRun    bool
	injectSync      bool
	injectNoSync    bool
	injectStats     bool
	injectUninstall bool
	injectStatus    bool
	injectJSON      bool
	injectVerbose    bool
	injectVerbosity  string
)

var injectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Push SAP context to your AI tools",
	Long: `Inject up-to-date SAP developer context into all detected AI tools.

Injects at global (user) scope by default into tools such as Claude Code,
Cursor, and GitHub Copilot. Use --project to opt into project scope and inject
into project-level files (CLAUDE.md, .cursorrules, etc.) in the current directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if injectSync && injectNoSync {
			return fmt.Errorf("--sync and --no-sync are mutually exclusive")
		}

		if injectUninstall && (injectSync || injectNoSync) {
			return fmt.Errorf("--uninstall is incompatible with --sync and --no-sync")
		}

		if injectStatus && (injectUninstall || injectSync || injectNoSync || injectDryRun || injectStats) {
			return fmt.Errorf("--status is incompatible with --uninstall, --sync, --no-sync, --dry-run, and --stats")
		}

		if injectVerbosity != "" && injectVerbosity != "minimal" && injectVerbosity != "standard" && injectVerbosity != "full" {
			return fmt.Errorf("--verbosity must be minimal, standard, or full")
		}

		if injectUninstall {
			lang := i18n.ActiveLang
			var buf bytes.Buffer
			gatheredAdapters, err := loadAdapters()
			if err != nil {
				return err
			}
			scope := "global"
			if injectProject {
				scope = "project"
			}
			opts := adapter.Options{
				Uninstall:  true,
				Scope:      scope,
				ToolFilter: injectTool,
				DryRun:     injectDryRun,
				Lang:       lang,
				Out:        &buf,
			}
			eng := adapter.NewEngine(gatheredAdapters, nil, nil, opts)
			res := eng.Run()
			if res.Err != nil {
				return res.Err
			}
			if injectDryRun {
				if res.DryFound > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "inject.uninstall.dry_run_header"))
					fmt.Fprint(cmd.OutOrStdout(), buf.String())
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "inject.uninstall.nothing_found"))
				}
			} else {
				if res.Found > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "inject.uninstall.header"))
					fmt.Fprint(cmd.OutOrStdout(), buf.String())
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), i18n.T(lang, "inject.uninstall.nothing_found"))
				}
			}
			return nil
		}

		if injectStatus {
			lang := i18n.ActiveLang
			gatheredAdapters, err := loadAdapters()
			if err != nil {
				return err
			}
			scope := "global"
			if injectProject {
				scope = "project"
			}

			// Load packs for staleness check (errors are non-fatal — status still works without packs)
			loader, loaderErr := newContentLoader()
			var packs []*content.Pack
			var activeProfile *content.Profile
			if loaderErr == nil {
				paths, pathsErr := xdg.New()
				if pathsErr == nil {
					configProfile, _ := config.LoadProfile(paths.ConfigDir)
					if configProfile.ID != "" {
						activeProfile, _ = loader.FindProfile(configProfile.ID)
					}
				}
				packs, _ = loader.LoadPacks(activeProfile, lang)
			}

			opts := adapter.Options{
				Scope:      scope,
				ToolFilter: injectTool,
				Lang:       lang,
				Verbosity:  injectVerbosity,
			}
			eng := adapter.NewEngine(gatheredAdapters, packs, activeProfile, opts)
			rows, statusErr := eng.Status()
			if statusErr != nil {
				return statusErr
			}

			if injectJSON {
				return printStatusJSON(cmd, rows)
			}
			printStatusTable(cmd, rows, lang, injectVerbose)
			return nil
		}

		scope := "global"
		if injectProject {
			scope = "project"
		}

		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}
		configProfile, err := config.LoadProfile(paths.ConfigDir)
		if err != nil {
			return err
		}

		var activeProfile *content.Profile
		if configProfile.ID != "" {
			activeProfile, err = loader.FindProfile(configProfile.ID)
			if err != nil {
				return err
			}
			if activeProfile == nil {
				return fmt.Errorf("profile %q not found in any content layer", configProfile.ID)
			}
		}

		packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
		if err != nil {
			return err
		}

		// Staleness check — skip if --no-sync or no packs have markers, run unconditionally if --sync
		engine := sapSync.NewEngine(paths.CacheDir, 24*time.Hour, nil)
		if !injectNoSync {
			if injectSync || isStaleDynamicContent(engine, packs, paths) {
				if injectSync || shouldSyncNow(cmd) {
					if err := runSync(cmd.Context(), false, cmd.OutOrStdout()); err != nil {
						fmt.Fprintf(os.Stderr, "sap-devs: sync failed: %v\n", err)
						// Non-fatal: continue with cached content
					} else {
						// Reload packs to pick up newly expanded content
						packs, err = loader.LoadPacks(activeProfile, i18n.ActiveLang)
						if err != nil {
							return err
						}
					}
				}
			}
		}

		// Read changelog for What's New injection block
		clEntries, clTime, clErr := sapSync.ReadChangelog(paths.CacheDir)
		if clErr != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: read changelog: %v\n", clErr)
		}

		// Resolve featured learning journeys for injection
		learningIndex, _ := learning.LoadIndex(paths.CacheDir, learning.CacheTTL)
		if learningIndex != nil {
			bySlug := make(map[string]learning.LearningJourney, len(learningIndex))
			for _, j := range learningIndex {
				bySlug[j.Slug] = j
			}
			for _, p := range packs {
				for _, ref := range p.LearningRefs {
					if ref.Featured {
						if j, ok := bySlug[ref.Slug]; ok {
							p.LearningForInject = append(p.LearningForInject, content.LearningJourneyInjection{
								Title:    j.Title,
								URL:      j.URL,
								Level:    j.Level,
								Duration: j.DurationHours + " hr",
							})
						}
					}
				}
			}
		}

		// Gather current working directory for project type detection.
		cwd, _ := os.Getwd() // silently ignore error; GatherDynamic handles empty CWD

		// Build command list from cobra for CLI self-awareness.
		var cmdInfos []content.CommandInfo
		for _, c := range rootCmd.Commands() {
			if !c.Hidden {
				cmdInfos = append(cmdInfos, content.CommandInfo{
					Name:  strings.SplitN(c.Use, " ", 2)[0],
					Short: c.Short,
				})
			}
		}

		gatheredAdapters, err := loadAdapters()
		if err != nil {
			return err
		}

		// Detect project context once — reused by both GatherDynamic and health checks
		pc, _ := project.Detect(cwd)

		dynCtx := dynamic.GatherDynamic(dynamic.GatherOpts{
			CWD:            cwd,
			CLIVersion:     Version,
			Profile:        activeProfile,
			Packs:          packs,
			SyncStateDir:   paths.CacheDir,
			Adapters:       gatheredAdapters,
			Commands:        cmdInfos,
			ProjectContext: pc,
		})

		// Run project health checks and attach findings to dynamic context
		if pc != nil && pc.Type != "" {
			findings := project.Check(pc, cwd, packs)
			for _, f := range findings {
				dynCtx.ProjectFindings = append(dynCtx.ProjectFindings, content.ProjectFinding{
					Severity: f.Severity,
					Message:  f.Message,
				})
			}
		}

		// Load scratch notes for project-scope injection
		if injectProject {
			notes, _ := scratch.Load(cwd)
			dynCtx.ScratchNotes = notes
		}

		// Translate sync changelog entries to content WhatsNewEntry for rendering
		if len(clEntries) > 0 {
			for _, e := range clEntries {
				dynCtx.WhatsNew = append(dynCtx.WhatsNew, content.WhatsNewEntry{
					Pack: e.Pack,
					Text: e.Text,
				})
			}
			dynCtx.WhatsNewDate = &clTime
		}

		opts := adapter.Options{
			Scope:      scope,
			ToolFilter: injectTool,
			DryRun:     injectDryRun,
			Stats:      injectStats,
			Out:        cmd.OutOrStdout(),
			Dynamic:    dynCtx,
			Verbosity:  injectVerbosity,
		}
		eng, err := newAdapterEngine(packs, activeProfile, opts)
		if err != nil {
			return err
		}

		if injectDryRun {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "inject.dry_run"))
		}
		res := eng.Run()
		if res.Err != nil {
			return res.Err
		}
		if !injectDryRun {
			_ = sapSync.ConsumeChangelog(paths.CacheDir)
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "inject.done", map[string]any{"Scope": scope}))
			if injectTool == "" {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "inject.hint"))
			}
		}
		return nil
	},
}

// isStaleDynamicContent returns true if any active pack has markers and its expanded
// content is missing, failed on last fetch, or past TTL.
// paths is used to locate context.expanded.md in the official cache.
func isStaleDynamicContent(engine *sapSync.Engine, packs []*content.Pack, paths *xdg.Paths) bool {
	packsBlock := engine.PacksBlock()
	if packsBlock == nil {
		// No packs block yet — treat as potentially stale
		return len(packs) > 0
	}
	for _, p := range packs {
		ps, known := packsBlock[p.ID]
		if !known || !ps.HasMarkers {
			continue
		}
		// Condition 1: context.expanded.md must exist
		// Only official-layer packs support marker expansion; company/user/project packs are not checked.
		expandedPath := filepath.Join(paths.CacheDir, "official", "content", "packs", p.ID, "context.expanded.md")
		if _, err := os.Stat(expandedPath); err != nil {
			return true
		}
		// Conditions 2+3: iterate recorded marker states until no more found
		for i := 0; ; i++ {
			ms, ok := engine.GetMarkerState(p.ID, i)
			if !ok {
				break // no more markers recorded for this pack
			}
			if !ms.OK {
				return true
			}
			ttl := time.Duration(ms.TTLHours) * time.Hour
			if ttl <= 0 {
				ttl = 7 * 24 * time.Hour // default 7-day TTL for markers
			}
			if time.Since(ms.LastFetched) > ttl {
				return true
			}
		}
	}
	return false
}

// shouldSyncNow prompts the user interactively. Returns true if user answers Y or is non-TTY auto-proceed.
// Non-TTY: auto-proceeds with cached content (returns false) and warns to stderr.
func shouldSyncNow(cmd *cobra.Command) bool {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintln(os.Stderr, `sap-devs: dynamic content is stale; run "sap-devs sync" to refresh`)
		return false
	}
	fmt.Fprint(cmd.OutOrStdout(), "  Dynamic content may be stale. Sync now for latest content? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.TrimSpace(line)
	return answer == "" || answer == "Y" || answer == "y"
}

func printStatusJSON(cmd *cobra.Command, rows []adapter.StatusRow) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func printStatusTable(cmd *cobra.Command, rows []adapter.StatusRow, lang string, verbose bool) {
	w := cmd.OutOrStdout()
	if len(rows) == 0 {
		fmt.Fprintln(w, i18n.T(lang, "inject.status.no_results"))
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	if verbose {
		fmt.Fprintln(tw,
			i18n.T(lang, "inject.status.header_tool")+"\t"+
				i18n.T(lang, "inject.status.header_scope")+"\t"+
				i18n.T(lang, "inject.status.header_file")+"\t"+
				i18n.T(lang, "inject.status.header_status")+"\t"+
				i18n.T(lang, "inject.status.header_size")+"\t"+
				i18n.T(lang, "inject.status.header_tokens")+"\t"+
				i18n.T(lang, "inject.status.header_sap_pct")+"\t"+
				i18n.T(lang, "inject.status.header_other"))
	} else {
		fmt.Fprintln(tw,
			i18n.T(lang, "inject.status.header_tool")+"\t"+
				i18n.T(lang, "inject.status.header_scope")+"\t"+
				i18n.T(lang, "inject.status.header_file")+"\t"+
				i18n.T(lang, "inject.status.header_status"))
	}
	for _, row := range rows {
		status := injectStatusLabel(row, lang)
		if verbose {
			pct := 0
			if row.FileTokenEst > 0 {
				pct = row.SapDevsTokens * 100 / row.FileTokenEst
			}
			other := fmt.Sprintf("%d", len(row.OtherSections))
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d B\t%d\t%d%%\t%s\n",
				row.AdapterName, row.Scope, row.TargetPath, status,
				row.FileSizeBytes, row.FileTokenEst, pct, other)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				row.AdapterName, row.Scope, row.TargetPath, status)
		}
	}
	tw.Flush()
}

func injectStatusLabel(row adapter.StatusRow, lang string) string {
	if !row.FileExists {
		return i18n.T(lang, "inject.status.not_found")
	}
	if row.Orphaned {
		return i18n.T(lang, "inject.status.orphaned")
	}
	if !row.Injected {
		return i18n.T(lang, "inject.status.not_injected")
	}
	if row.Stale {
		return i18n.T(lang, "inject.status.stale")
	}
	return i18n.T(lang, "inject.status.current")
}

func init() {
	injectCmd.Flags().BoolVar(&injectProject, "project", false, "inject at project scope (current directory)")
	injectCmd.Flags().StringVar(&injectTool, "tool", "", "inject into a specific tool only (e.g. claude-code)")
	injectCmd.Flags().BoolVar(&injectDryRun, "dry-run", false, "preview changes without writing files")
	injectCmd.Flags().BoolVar(&injectSync, "sync", false, "sync dynamic content before injecting (no prompt)")
	injectCmd.Flags().BoolVar(&injectNoSync, "no-sync", false, "skip freshness check; use cached content as-is")
	injectCmd.Flags().BoolVar(&injectStats, "stats", false, "show injection stats per adapter")
	injectCmd.Flags().BoolVar(&injectUninstall, "uninstall", false, "remove previously injected SAP developer context")
	injectCmd.Flags().BoolVar(&injectStatus, "status", false, "report injection state for all detected AI tools")
	injectCmd.Flags().BoolVar(&injectJSON, "json", false, "output status as JSON (only with --status)")
	injectCmd.Flags().BoolVar(&injectVerbose, "verbose", false, "show file size and token breakdown (only with --status)")
	injectCmd.Flags().StringVar(&injectVerbosity, "verbosity", "", "verbosity level: minimal, standard, full (overrides adapter default)")
	rootCmd.AddCommand(injectCmd)
}
