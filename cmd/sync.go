package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/credentials"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

const officialRepoArchive = "https://github.tools.sap/developer-relations/sap-devs-cli/archive/refs/heads/main.zip"

var syncForce bool
var syncCategory string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull latest SAP developer content",
	Long:  `Syncs content from the official repo (and company repo if configured). Respects per-category TTLs unless --force is set.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		if cfg.Sync.Disabled {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "sync.disabled"))
			return nil
		}

		token := credentials.Resolve(paths.ConfigDir)

		categories := allCategories()
		if syncCategory != "" {
			categories = []string{syncCategory}
		}

		officialCache := filepath.Join(paths.CacheDir, "official")
		ttls := map[string]time.Duration{
			"tips":      cfg.Sync.Tips,
			"tools":     cfg.Sync.Tools,
			"advocates": cfg.Sync.Advocates,
			"resources": cfg.Sync.Resources,
			"context":   cfg.Sync.Context,
			"mcp":       cfg.Sync.MCP,
		}
		engine := sapSync.NewEngine(paths.CacheDir, 24*time.Hour, ttls)

		needsSync := false
		for _, cat := range categories {
			if syncForce || engine.IsStale(cat) {
				needsSync = true
				break
			}
		}

		if !needsSync {
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "sync.up_to_date"))
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "sync.syncing"))
		if err := sapSync.FetchArchive(officialRepoArchive, officialCache, token); err != nil {
			return fmt.Errorf("sync official content: %w", err)
		}

		if err := engine.MarkAllSynced(categories); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "sync.updated", map[string]any{"Categories": categories}))

		// Sync company repo if configured
		if cfg.CompanyRepo != "" {
			if !strings.HasPrefix(cfg.CompanyRepo, "https://") {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "sync.warn_https", map[string]any{"URL": cfg.CompanyRepo}))
			} else {
				companyCache := filepath.Join(paths.CacheDir, "company")
				repoURL := strings.TrimRight(cfg.CompanyRepo, "/")
				companyArchive := repoURL + "/archive/refs/heads/main.zip"
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "sync.syncing_company"))
				if err := sapSync.FetchArchive(companyArchive, companyCache, token); err != nil {
					fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "sync.warn_company_failed", map[string]any{"Err": err}))
				}
			}
		}
		return nil
	},
}

func allCategories() []string {
	return []string{"tips", "tools", "resources", "context", "mcp", "advocates"}
}

func init() {
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "Re-sync all categories regardless of TTL")
	syncCmd.Flags().StringVar(&syncCategory, "category", "", "Sync a single category only")
	rootCmd.AddCommand(syncCmd)
}
