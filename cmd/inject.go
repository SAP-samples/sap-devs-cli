package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var (
	injectProject bool
	injectTool    string
	injectDryRun  bool
	injectSync    bool
	injectNoSync  bool
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

		rendered := content.RenderContext(packs, activeProfile)

		opts := adapter.Options{
			Scope:      scope,
			ToolFilter: injectTool,
			DryRun:     injectDryRun,
		}
		eng, err := newAdapterEngine(rendered, opts)
		if err != nil {
			return err
		}

		if injectDryRun {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "inject.dry_run"))
		}
		if err := eng.Run(); err != nil {
			return err
		}
		if !injectDryRun {
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

func init() {
	injectCmd.Flags().BoolVar(&injectProject, "project", false, "inject at project scope (current directory)")
	injectCmd.Flags().StringVar(&injectTool, "tool", "", "inject into a specific tool only (e.g. claude-code)")
	injectCmd.Flags().BoolVar(&injectDryRun, "dry-run", false, "preview changes without writing files")
	injectCmd.Flags().BoolVar(&injectSync, "sync", false, "sync dynamic content before injecting (no prompt)")
	injectCmd.Flags().BoolVar(&injectNoSync, "no-sync", false, "skip freshness check; use cached content as-is")
	rootCmd.AddCommand(injectCmd)
}
