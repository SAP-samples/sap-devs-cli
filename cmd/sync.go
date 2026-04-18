package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/credentials"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/events"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/ui"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/videos"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/youtube"
	"gopkg.in/yaml.v3"
)

const officialRepoArchive = "https://github.tools.sap/developer-relations/sap-devs-cli/archive/refs/heads/main.zip"

var syncForce bool
var syncCategory string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull latest SAP developer content",
	Long:  `Syncs content from the official repo (and company repo if configured). Respects per-category TTLs unless --force is set.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd.Context(), syncForce, cmd.OutOrStdout())
	},
}

// runSync is the shared sync implementation used by both the sync command and inline inject sync.
// out receives all progress messages; pass cmd.OutOrStdout() or os.Stdout as appropriate.
func runSync(ctx context.Context, force bool, out io.Writer) error {
	paths, err := xdg.New()
	if err != nil {
		return err
	}
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil {
		return err
	}
	if cfg.Sync.Disabled {
		fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.disabled"))
		return nil
	}

	token := credentials.Resolve(paths.ConfigDir)
	categories := allCategories()
	// Apply --category filter when called directly from syncCmd (syncCategory is set by the flag)
	if syncCategory != "" {
		categories = []string{syncCategory}
	}

	officialCache := filepath.Join(paths.CacheDir, "official")
	ttls := map[string]time.Duration{
		"tips": cfg.Sync.Tips, "tools": cfg.Sync.Tools,
		"advocates": cfg.Sync.Advocates, "resources": cfg.Sync.Resources,
		"context": cfg.Sync.Context, "mcp": cfg.Sync.MCP,
		"events": cfg.Sync.Events, "youtube": cfg.Sync.YouTube,
	}
	engine := sapSync.NewEngine(paths.CacheDir, 24*time.Hour, ttls)

	archiveCats := []string{"tips", "tools", "resources", "context", "mcp", "advocates"}
	independentCats := []string{"events", "youtube"}

	activeArchive := intersectStrings(archiveCats, categories)
	activeIndependent := intersectStrings(independentCats, categories)

	archiveNeedsSync := force
	for _, cat := range activeArchive {
		if engine.IsStale(cat) {
			archiveNeedsSync = true
			break
		}
	}
	independentNeedsSync := force
	for _, cat := range activeIndependent {
		if engine.IsStale(cat) {
			independentNeedsSync = true
			break
		}
	}

	if !archiveNeedsSync && !independentNeedsSync {
		fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.up_to_date"))
		return nil
	}

	if archiveNeedsSync {
		fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.syncing"))
		if err := sapSync.FetchArchive(officialRepoArchive, officialCache, token); err != nil {
			return fmt.Errorf("sync official content: %w", err)
		}
		if err := engine.MarkAllSynced(activeArchive); err != nil {
			return err
		}
		fmt.Fprintln(out, i18n.Tf(i18n.ActiveLang, "sync.updated", map[string]any{"Categories": activeArchive}))

		// Phase 2: marker expansion (Bubbletea progress)
		if err := runMarkerExpansion(officialCache, engine); err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: marker expansion warning: %v\n", err)
			// Non-fatal: sync continues
		}

		// Sync company repo if configured
		if cfg.CompanyRepo != "" {
			if !strings.HasPrefix(cfg.CompanyRepo, "https://") {
				fmt.Fprintln(out, i18n.Tf(i18n.ActiveLang, "sync.warn_https", map[string]any{"URL": cfg.CompanyRepo}))
			} else {
				companyCache := filepath.Join(paths.CacheDir, "company")
				repoURL := strings.TrimRight(cfg.CompanyRepo, "/")
				companyArchive := repoURL + "/archive/refs/heads/main.zip"
				fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.syncing_company"))
				if err := sapSync.FetchArchive(companyArchive, companyCache, token); err != nil {
					fmt.Fprintln(out, i18n.Tf(i18n.ActiveLang, "sync.warn_company_failed", map[string]any{"Err": err}))
				}
			}
		}
	}

	// Phase 3: events RSS cache
	if containsString(activeIndependent, "events") && (force || engine.IsStale("events")) {
		if err := runEventsFetch(paths.CacheDir, officialCache, force); err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: events sync warning: %v\n", err)
		}
		_ = engine.MarkSynced("events")
	}

	// Phase 4: YouTube fetch
	if containsString(activeIndependent, "youtube") && (force || engine.IsStale("youtube")) {
		if err := runYouTubeFetch(paths.CacheDir, officialCache, cfg.CompanyRepo, paths.ConfigDir, force); err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: youtube sync warning: %v\n", err)
		}
		_ = engine.MarkSynced("youtube")
	}

	return nil
}

// runMarkerExpansion scans all official-layer packs for sync:fetch markers,
// fetches them in parallel with a Bubbletea progress display, and writes
// context.expanded.md alongside each context.md.
func runMarkerExpansion(officialCache string, engine *sapSync.Engine) error {
	packsDir := filepath.Join(officialCache, "content", "packs")
	entries, err := os.ReadDir(packsDir)
	if err != nil {
		return nil // No packs directory yet — first run before archive fetch
	}

	var allMarkers []sapSync.Marker
	packContexts := make(map[string]string) // packID → context.md content

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packID := entry.Name()
		contextPath := filepath.Join(packsDir, packID, "context.md")
		data, err := os.ReadFile(contextPath)
		if err != nil {
			continue
		}
		contextContent := string(data)
		markers, warns := sapSync.ScanMarkers(packID, contextContent)
		for _, w := range warns {
			fmt.Fprintf(os.Stderr, "sap-devs: %s\n", w)
		}
		hasMarkers := len(markers) > 0
		if err := engine.SetPackHasMarkers(packID, hasMarkers); err != nil {
			return err
		}
		if hasMarkers {
			packContexts[packID] = contextContent
			allMarkers = append(allMarkers, markers...)
		}
	}

	if len(allMarkers) == 0 {
		return nil
	}

	// Fetch all markers in parallel with progress display
	results, fetchErrs := ui.RunMarkerExpansion(allMarkers)

	// Record marker states and write expanded files
	for packID, contextContent := range packContexts {
		// Collect markers for this pack
		var packMarkers []sapSync.Marker
		for _, m := range allMarkers {
			if m.PackID == packID {
				packMarkers = append(packMarkers, m)
			}
		}

		// Build per-pack results map for ExpandMarkers (keyed by int index)
		packResults := make(map[int]string)
		for _, m := range packMarkers {
			key := m.PackID + "::" + strconv.Itoa(m.Index)
			if content, ok := results[key]; ok {
				packResults[m.Index] = content
			}
		}

		// Record state for each marker
		// Each RecordMarkerState call does a full load-mutate-save cycle.
		// Acceptable for the typical case of 1-5 markers; a batch API can be
		// added if marker counts grow significantly.
		for _, m := range packMarkers {
			ms := sapSync.MarkerState{
				URL:      m.URL,
				TTLHours: m.TTLHours,
				OK:       fetchErrs[m.PackID+"::"+strconv.Itoa(m.Index)] == nil,
			}
			if ms.OK {
				ms.LastFetched = time.Now()
			}
			if err := engine.RecordMarkerState(packID, m.Index, ms); err != nil {
				return err
			}
		}

		// Expand and write context.expanded.md
		expanded := sapSync.ExpandMarkers(contextContent, packMarkers, packResults)
		expandedPath := filepath.Join(packsDir, packID, "context.expanded.md")
		if err := os.WriteFile(expandedPath, []byte(expanded), 0644); err != nil {
			return fmt.Errorf("write %s: %w", expandedPath, err)
		}
	}
	return nil
}

func runEventsFetch(cacheDir, officialCache string, force bool) error {
	packsDir := filepath.Join(officialCache, "content", "packs")
	entries, err := os.ReadDir(packsDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		typesPath := filepath.Join(packsDir, entry.Name(), "event-types.yaml")
		data, err := os.ReadFile(typesPath)
		if err != nil {
			continue
		}
		var types []content.EventType
		if err := yaml.Unmarshal(data, &types); err != nil {
			continue
		}
		for _, et := range types {
			if et.Source != "manual" {
				events.Resolve(et, cacheDir, force)
			}
		}
	}
	return nil
}

func allCategories() []string {
	return []string{"tips", "tools", "resources", "context", "mcp", "advocates", "events", "youtube"}
}

func intersectStrings(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, s := range b {
		set[s] = true
	}
	var out []string
	for _, s := range a {
		if set[s] {
			out = append(out, s)
		}
	}
	return out
}

func runYouTubeFetch(cacheDir, officialCache, companyRepo, configDir string, force bool) error {
	apiKey := credentials.ResolveService(configDir, "youtube", []string{"YOUTUBE_API_KEY"})

	scanDirs := []string{officialCache}
	if companyRepo != "" {
		scanDirs = append(scanDirs, filepath.Join(filepath.Dir(officialCache), "company"))
	}

	for _, base := range scanDirs {
		packsDir := filepath.Join(base, "content", "packs")
		entries, err := os.ReadDir(packsDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			packID := entry.Name()
			ytPath := filepath.Join(packsDir, packID, "youtube.yaml")
			data, err := os.ReadFile(ytPath)
			if err != nil {
				continue
			}
			var sources []content.YouTubeSource
			if err := yaml.Unmarshal(data, &sources); err != nil {
				continue
			}
			for _, src := range sources {
				if src.Type != "playlist" {
					continue
				}
				src.PackID = packID
				fetchAndCacheSource(cacheDir, src, apiKey)
			}
		}
	}
	return nil
}

func fetchAndCacheSource(cacheDir string, src content.YouTubeSource, apiKey string) {
	episodes, err := youtube.Resolve(src, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sap-devs: fetch %s/%s: %v\n", src.PackID, src.ID, err)
		return
	}
	vids := make([]content.Video, 0, len(episodes))
	for _, ep := range episodes {
		v := content.Video{
			ID:          fmt.Sprintf("%s/%s/%s", src.PackID, src.ID, ep.ID),
			Title:       ep.Title,
			URL:         ep.URL,
			VideoID:     ep.ID,
			Published:   ep.Published,
			Description: ep.Description,
			Duration:    ep.Duration,
			SourceID:    src.ID,
			PackID:      src.PackID,
		}
		tagSet := make(map[string]bool)
		for _, t := range src.Tags {
			tagSet[t] = true
		}
		for _, t := range ep.Tags {
			tagSet[t] = true
		}
		for t := range tagSet {
			v.Tags = append(v.Tags, t)
		}
		vids = append(vids, v)
	}
	_ = videos.SaveCache(cacheDir, src.PackID, src.ID, vids)
}

func init() {
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "Re-sync all categories regardless of TTL")
	syncCmd.Flags().StringVar(&syncCategory, "category", "", "Sync a single category only")
	rootCmd.AddCommand(syncCmd)
}
