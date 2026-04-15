package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/credentials"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/update"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

// updateHintCh carries a background update hint to PersistentPostRunE.
// Reset to nil at the top of each PersistentPreRunE to avoid stale channels
// across multiple command invocations in a single process (e.g. tests).
var updateHintCh chan string

var rootCmd = &cobra.Command{
	Use:   "sap-devs",
	Short: "AI-first SAP developer toolkit",
	Long:  `sap-devs injects up-to-date SAP developer knowledge into your AI tools.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		updateHintCh = nil // reset before every invocation

		// Resolve active language before any command body runs.
		// Runs for all commands so that Short/Long are always localized.
		if paths, err := xdg.New(); err == nil {
			if cfg, err := config.Load(paths.ConfigDir); err == nil {
				i18n.ActiveLang = i18n.Resolve(cfg.Language)
			}
		}
		if i18n.ActiveLang == "" {
			i18n.ActiveLang = "en"
		}
		localizeCommands(cmd.Root(), i18n.ActiveLang)

		// Skip background update check for "update" command and dev builds.
		if cmd.Name() == "update" || Version == "dev" {
			return nil
		}
		cacheDir := mustCacheDir()
		if cacheDir == "" {
			return nil // can't determine cache dir; skip check silently
		}
		if !update.ShouldCheck(cacheDir, 168*time.Hour) {
			return nil
		}
		updateHintCh = make(chan string, 1)
		go func() {
			var token string
			if paths, err := xdg.New(); err == nil {
				token = credentials.Resolve(paths.ConfigDir)
			}
			rel, newer, err := update.CheckLatest(repoURL, Version, token)
			if err == nil {
				update.RecordCheck(cacheDir)
				if newer {
					updateHintCh <- i18n.Tf(i18n.ActiveLang, "root.update_hint", map[string]any{"TagName": rel.TagName})
				}
			}
			// on error: channel stays empty, hint is skipped, RecordCheck not called
		}()
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if updateHintCh == nil {
			return nil
		}
		select {
		case hint := <-updateHintCh:
			fmt.Fprintln(os.Stderr, hint)
		case <-time.After(3 * time.Second):
			// goroutine too slow or no update available — skip silently
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// mustCacheDir returns the XDG cache directory, or empty string on failure.
// An empty string causes the background update check to be skipped silently.
func mustCacheDir() string {
	paths, err := xdg.New()
	if err != nil {
		return ""
	}
	return paths.CacheDir
}

// newContentLoader constructs a ContentLoader using platform paths and config.
func newContentLoader() (*content.ContentLoader, error) {
	paths, err := xdg.New()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil {
		return nil, err
	}
	loader := &content.ContentLoader{
		OfficialDir: filepath.Join(paths.CacheDir, "official", "content"),
		UserDir:     paths.DataDir,
	}
	if cfg.CompanyRepo != "" {
		loader.CompanyDir = filepath.Join(paths.CacheDir, "company", "content")
	}
	// Check for per-project .sap-devs dir
	if wd, wdErr := os.Getwd(); wdErr == nil {
		projectDir := filepath.Join(wd, ".sap-devs")
		if _, err := os.Stat(projectDir); err == nil {
			loader.ProjectDir = projectDir
		}
	}
	return loader, nil
}

// loadAdapters returns the merged adapter list across all configured layers:
// official cache, optional company cache, and an optional SAP_DEVS_DEV=1 local fallback.
func loadAdapters() ([]adapter.Adapter, error) {
	paths, err := xdg.New()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil {
		return nil, err
	}

	var allAdapters []adapter.Adapter

	// Official adapters from cache
	officialAdaptersDir := filepath.Join(paths.CacheDir, "official", "content", "adapters")
	if a, err := adapter.LoadAdapters(officialAdaptersDir); err == nil {
		allAdapters = append(allAdapters, a...)
	}

	// Company adapters override official by ID
	if cfg.CompanyRepo != "" {
		companyAdaptersDir := filepath.Join(paths.CacheDir, "company", "content", "adapters")
		if a, err := adapter.LoadAdapters(companyAdaptersDir); err == nil {
			allAdapters = mergeAdapters(allAdapters, a)
		}
	}

	// Dev fallback: only when SAP_DEVS_DEV=1 (local development from repo root)
	if os.Getenv("SAP_DEVS_DEV") == "1" {
		if a, err := adapter.LoadAdapters("content/adapters"); err == nil {
			allAdapters = mergeAdapters(allAdapters, a)
		}
	}

	return allAdapters, nil
}

// newAdapterEngine constructs an adapter engine from all configured adapter layers.
func newAdapterEngine(renderedContext string, opts adapter.Options) (*adapter.Engine, error) {
	allAdapters, err := loadAdapters()
	if err != nil {
		return nil, err
	}
	return adapter.NewEngine(allAdapters, renderedContext, opts), nil
}

// mergeAdapters merges src into dst, overriding by adapter ID.
func mergeAdapters(dst, src []adapter.Adapter) []adapter.Adapter {
	index := make(map[string]int)
	for i, a := range dst {
		index[a.ID] = i
	}
	for _, a := range src {
		if i, ok := index[a.ID]; ok {
			dst[i] = a
		} else {
			dst = append(dst, a)
		}
	}
	return dst
}

// localizeCommands walks root and all its descendants and updates Short and Long
// from the i18n catalog. Uses i18n.Lookup so cobra-registered strings are never
// overwritten with bare key names when a key is absent from both catalogs.
// Key path segments are derived from cmd.Name() (cobra's first word of Use).
// The root command uses the hardcoded prefix "root".
func localizeCommands(root *cobra.Command, lang string) {
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		prefix := buildLocalizeKey(cmd)
		if s, ok := i18n.Lookup(lang, prefix+".short"); ok {
			cmd.Short = s
		}
		if s, ok := i18n.Lookup(lang, prefix+".long"); ok {
			cmd.Long = s
		}
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(root)
}

// buildLocalizeKey returns the dot-separated i18n key prefix for cmd.
// The root command (!cmd.HasParent()) returns "root".
// Other commands build the path by walking up the parent chain
// (excluding the root command itself).
func buildLocalizeKey(cmd *cobra.Command) string {
	if !cmd.HasParent() {
		return "root"
	}
	var parts []string
	for c := cmd; c.HasParent(); c = c.Parent() {
		parts = append([]string{c.Name()}, parts...)
	}
	return strings.Join(parts, ".")
}

// RootCmd returns the root cobra command. Used in tests.
func RootCmd() *cobra.Command { return rootCmd }
