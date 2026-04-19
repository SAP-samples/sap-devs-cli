package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	gosync "sync"
	"time"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/credentials"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/discovery"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/events"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
	sapSync "github.tools.sap/developer-relations/sap-devs-cli/internal/sync"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/ui"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/videos"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/youtube"
	"golang.org/x/sync/errgroup"
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
		"discovery": cfg.Sync.Discovery, "tutorials": cfg.Sync.Tutorials,
		"learning": cfg.Sync.Learning,
	}
	engine := sapSync.NewEngine(paths.CacheDir, 24*time.Hour, ttls)

	archiveCats := []string{"tips", "tools", "resources", "context", "mcp", "advocates"}
	independentCats := []string{"events", "youtube", "discovery", "tutorials", "learning"}

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
		if err := runYouTubeFetch(paths.CacheDir, officialCache, cfg.CompanyRepo, paths.ConfigDir, force, cfg.Sync.YouTube); err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: youtube sync warning: %v\n", err)
		}
		_ = engine.MarkSynced("youtube")
	}

	// Phase 5: Discovery Center fetch
	if containsString(activeIndependent, "discovery") && (force || engine.IsStale("discovery")) {
		if err := runDiscoveryFetch(paths.CacheDir, force); err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: discovery sync: %v\n", err)
		}
		_ = engine.MarkSynced("discovery")
	}

	// Phase 6: Tutorials index fetch
	if containsString(activeIndependent, "tutorials") && (force || engine.IsStale("tutorials")) {
		if err := runTutorialsFetch(paths.CacheDir, force); err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: tutorials sync: %v\n", err)
		}
		_ = engine.MarkSynced("tutorials")
	}

	// Phase 7: Learning journeys catalog fetch
	if containsString(activeIndependent, "learning") && (force || engine.IsStale("learning")) {
		if err := runLearningFetch(paths.CacheDir, force); err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: learning sync: %v\n", err)
		}
		_ = engine.MarkSynced("learning")
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
	return []string{"tips", "tools", "resources", "context", "mcp", "advocates", "events", "youtube", "discovery", "tutorials", "learning"}
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

func runYouTubeFetch(cacheDir, officialCache, companyRepo, configDir string, force bool, ttl time.Duration) error {
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
				if !force {
					age := videos.CacheAge(cacheDir, packID, src.ID)
					if age >= 0 && age < ttl {
						continue
					}
				}
				fetchAndCacheSource(cacheDir, src, apiKey)
			}
		}
	}
	return nil
}

func runDiscoveryFetch(cacheDir string, force bool) error {
	client := discovery.NewClient()

	if force || discovery.CacheAge(cacheDir, "missions") < 0 || discovery.CacheAge(cacheDir, "missions") > discovery.CacheTTL {
		groups, err := client.FetchMissions()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: fetch discovery missions: %v\n", err)
		} else {
			var all []discovery.Mission
			for _, g := range groups {
				all = append(all, g.Missions...)
			}
			_ = discovery.SaveCache(cacheDir, "missions", all)
		}
	}

	if force || discovery.CacheAge(cacheDir, "services") < 0 || discovery.CacheAge(cacheDir, "services") > discovery.CacheTTL {
		svcs, err := client.FetchServices()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: fetch discovery services: %v\n", err)
		} else {
			_ = discovery.SaveCache(cacheDir, "services", svcs)
		}
	}

	if force || discovery.CacheAge(cacheDir, "guidance-tree") < 0 || discovery.CacheAge(cacheDir, "guidance-tree") > discovery.CacheTTL {
		tree, err := client.FetchGuidanceTree()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: fetch discovery guidance: %v\n", err)
		} else {
			_ = discovery.SaveCache(cacheDir, "guidance-tree", tree)
		}
	}

	if force || discovery.CacheAge(cacheDir, "categories") < 0 || discovery.CacheAge(cacheDir, "categories") > discovery.CacheTTL {
		cats, err := client.FetchCategories()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: fetch discovery categories: %v\n", err)
		} else {
			_ = discovery.SaveCache(cacheDir, "categories", cats)
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
		sort.Strings(v.Tags)
		vids = append(vids, v)
	}
	_ = videos.SaveCache(cacheDir, src.PackID, src.ID, vids)
}

func runTutorialsFetch(cacheDir string, force bool) error {
	cachedRepos, _ := tutorials.LoadRepoInfo(cacheDir)
	cachedSHAs := make(map[string]string)
	cachedBranches := make(map[string]string)
	for _, r := range cachedRepos {
		cachedSHAs[r.Name] = r.TreeSHA
		cachedBranches[r.Name] = r.DefaultBranch
	}

	// Tutorials live on public github.com, not github.tools.sap.
	// Do NOT use credentials.Resolve() here — it targets the enterprise instance.
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}

	client := tutorials.NewClient(tutorials.ClientConfig{Token: token})

	repoNames, err := client.FetchRepoList()
	if err != nil {
		return err
	}

	// Phase 1: resolve branches + trees (API calls, bounded concurrency)
	type repoResult struct {
		info  tutorials.RepoInfo
		slugs []string
		reuse bool
	}
	results := make([]repoResult, len(repoNames))

	var mu gosync.Mutex
	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(5)

	for i, repo := range repoNames {
		i, repo := i, repo
		g.Go(func() error {
			branch, ok := cachedBranches[repo]
			if !ok || force {
				var err error
				branch, err = client.FetchDefaultBranch(repo)
				if err != nil {
					if !errors.Is(err, tutorials.ErrRepoUnavailable) {
						fmt.Fprintf(os.Stderr, "sap-devs: skip repo %s: %v\n", repo, err)
					}
					return nil
				}
			}

			slugs, sha, err := client.FetchRepoTree(repo, branch)
			if err != nil {
				if !errors.Is(err, tutorials.ErrRepoUnavailable) {
					fmt.Fprintf(os.Stderr, "sap-devs: skip repo %s: %v\n", repo, err)
				}
				return nil
			}

			mu.Lock()
			results[i] = repoResult{
				info:  tutorials.RepoInfo{Name: repo, DefaultBranch: branch, TreeSHA: sha},
				slugs: slugs,
				reuse: !force && cachedSHAs[repo] == sha && sha != "",
			}
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	// Phase 2: fetch frontmatter for changed repos (CDN, bounded concurrency)
	var allMeta []tutorials.TutorialMeta
	var repoInfos []tutorials.RepoInfo
	existingIndex, _ := tutorials.LoadIndex(cacheDir)

	for _, r := range results {
		if r.info.Name == "" {
			continue
		}
		repoInfos = append(repoInfos, r.info)

		if r.reuse {
			for _, m := range existingIndex {
				if m.Repo == r.info.Name {
					allMeta = append(allMeta, m)
				}
			}
			continue
		}

		var repoMeta []tutorials.TutorialMeta
		var metaMu gosync.Mutex
		fg, _ := errgroup.WithContext(context.Background())
		fg.SetLimit(10)

		for _, slug := range r.slugs {
			slug := slug
			repo := r.info.Name
			branch := r.info.DefaultBranch
			fg.Go(func() error {
				md, err := client.FetchRawMarkdown(repo, branch, slug)
				if err != nil {
					return nil
				}
				meta, err := tutorials.ParseFrontmatterOnly(md, slug, repo)
				if err != nil {
					return nil
				}
				metaMu.Lock()
				repoMeta = append(repoMeta, *meta)
				metaMu.Unlock()
				return nil
			})
		}
		_ = fg.Wait()
		allMeta = append(allMeta, repoMeta...)
	}

	allMeta = tutorials.Enrich(allMeta, "sap-devs-cli")

	if err := tutorials.SaveIndex(cacheDir, allMeta); err != nil {
		return err
	}
	return tutorials.SaveRepoInfo(cacheDir, repoInfos)
}

func runLearningFetch(cacheDir string, force bool) error {
	if !force {
		if age := learning.IndexCacheAge(cacheDir); age >= 0 && age <= learning.CacheTTL {
			return nil
		}
	}
	journeys, err := learning.FetchCatalog()
	if err != nil {
		if stale, ok := learning.LoadIndexStale(cacheDir); ok {
			_ = learning.SaveIndex(cacheDir, stale)
			return nil
		}
		return err
	}
	return learning.SaveIndex(cacheDir, journeys)
}

func init() {
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "Re-sync all categories regardless of TTL")
	syncCmd.Flags().StringVar(&syncCategory, "category", "", "Sync a single category only")
	rootCmd.AddCommand(syncCmd)
}
