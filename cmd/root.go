package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
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
