package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

var rootCmd = &cobra.Command{
	Use:   "sap-devs",
	Short: "AI-first SAP developer toolkit",
	Long:  `sap-devs injects up-to-date SAP developer knowledge into your AI tools.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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

// newAdapterEngine constructs an adapter engine from all configured adapter layers.
// It reads adapter YAML files from: official cache, company cache, and a local
// content/adapters fallback for development use.
func newAdapterEngine(renderedContext string, opts adapter.Options) (*adapter.Engine, error) {
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

	// Fall back to bundled adapters in the working directory (dev mode)
	if len(allAdapters) == 0 {
		if a, err := adapter.LoadAdapters("content/adapters"); err == nil {
			allAdapters = a
		}
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
