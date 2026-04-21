package cmd

import (
	"bytes"
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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/community"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/credentials"
	"github.com/SAP-samples/sap-devs-cli/internal/discovery"
	"github.com/SAP-samples/sap-devs-cli/internal/events"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/news"
	sapSync "github.com/SAP-samples/sap-devs-cli/internal/sync"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/SAP-samples/sap-devs-cli/internal/ui"
	"github.com/SAP-samples/sap-devs-cli/internal/videos"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
	"github.com/SAP-samples/sap-devs-cli/internal/youtube"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

const officialRepoArchive = "https://github.com/SAP-samples/sap-devs-cli/archive/refs/heads/main.zip"

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

// categoryToPhase maps independent sync categories to their PhaseID.
var categoryToPhase = map[string]ui.PhaseID{
	"events":    ui.PhaseEvents,
	"youtube":   ui.PhaseYouTube,
	"news":      ui.PhaseNews,
	"discovery": ui.PhaseDiscovery,
	"tutorials": ui.PhaseTutorials,
	"learning":  ui.PhaseLearning,
}

// syncPlan holds pre-computed state for a sync run.
type syncPlan struct {
	visiblePhases []ui.PhaseID
	archiveNeeded bool
	companyNeeded bool
	activeArchive []string
	activeIndep   []string
	indepPhases   map[string]ui.PhaseID
	force         bool
	officialCache string
	cfg           *config.Config
	paths         *xdg.Paths
	engine        *sapSync.Engine
	token         string
}

// buildSyncPlan checks staleness for each category and builds the list of visible phases.
func buildSyncPlan(cfg *config.Config, paths *xdg.Paths, engine *sapSync.Engine, token string, force bool) *syncPlan {
	categories := allCategories()
	if syncCategory != "" {
		categories = []string{syncCategory}
	}

	archiveCats := []string{"tips", "tools", "resources", "context", "mcp", "advocates"}
	independentCats := []string{"events", "youtube", "news", "discovery", "tutorials", "learning"}

	activeArchive := intersectStrings(archiveCats, categories)
	activeIndep := intersectStrings(independentCats, categories)

	plan := &syncPlan{
		activeArchive: activeArchive,
		activeIndep:   activeIndep,
		indepPhases:   make(map[string]ui.PhaseID),
		force:         force,
		officialCache: filepath.Join(paths.CacheDir, "official"),
		cfg:           cfg,
		paths:         paths,
		engine:        engine,
		token:         token,
	}

	// Check archive block staleness
	plan.archiveNeeded = force
	for _, cat := range activeArchive {
		if engine.IsStale(cat) {
			plan.archiveNeeded = true
			break
		}
	}

	if plan.archiveNeeded {
		plan.visiblePhases = append(plan.visiblePhases, ui.PhaseContent)
		if cfg.CompanyRepo != "" {
			plan.companyNeeded = true
			plan.visiblePhases = append(plan.visiblePhases, ui.PhaseCompany)
		}
		// Markers phase is always included when archive syncs
		plan.visiblePhases = append(plan.visiblePhases, ui.PhaseMarkers)
	}

	// Check each independent category
	for _, cat := range activeIndep {
		if force || engine.IsStale(cat) {
			pid, ok := categoryToPhase[cat]
			if !ok {
				continue
			}
			plan.visiblePhases = append(plan.visiblePhases, pid)
			plan.indepPhases[cat] = pid
		}
	}

	return plan
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
	ttls := map[string]time.Duration{
		"tips": cfg.Sync.Tips, "tools": cfg.Sync.Tools,
		"advocates": cfg.Sync.Advocates, "resources": cfg.Sync.Resources,
		"context": cfg.Sync.Context, "mcp": cfg.Sync.MCP,
		"events": cfg.Sync.Events, "youtube": cfg.Sync.YouTube,
		"news": cfg.Sync.News, "discovery": cfg.Sync.Discovery, "tutorials": cfg.Sync.Tutorials,
		"learning": cfg.Sync.Learning,
	}
	engine := sapSync.NewEngine(paths.CacheDir, 24*time.Hour, ttls)

	plan := buildSyncPlan(cfg, paths, engine, token, force)

	// Fast path: everything is up to date
	if len(plan.visiblePhases) == 0 {
		fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.up_to_date"))
		return nil
	}

	// Detect TTY to choose between Bubbletea and plain text output
	isTTY := false
	if f, ok := out.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	if isTTY {
		return runSyncTTY(plan)
	}
	return runSyncPlain(plan, out)
}

// indepPhase pairs a category with its PhaseID and fetch function.
type indepPhase struct {
	cat string
	id  ui.PhaseID
	fn  func() (string, error)
}

// syncWorker runs in a goroutine and drives all sync phases, sending messages
// to the Bubbletea program for progress display.
func syncWorker(plan *syncPlan, p *tea.Program) {
	var fatalErr error
	defer func() {
		p.Send(ui.SyncDoneMsg{FatalErr: fatalErr})
	}()

	// --- Archive block ---
	if plan.archiveNeeded {
		// Phase: content
		p.Send(ui.PhaseStartMsg{ID: ui.PhaseContent})
		if err := sapSync.FetchArchive(officialRepoArchive, plan.officialCache, plan.token); err != nil {
			fatalErr = fmt.Errorf("sync official content: %w", err)
			p.Send(ui.PhaseDoneMsg{ID: ui.PhaseContent, Err: fatalErr})
			return
		}
		if err := plan.engine.MarkAllSynced(plan.activeArchive); err != nil {
			fatalErr = err
			p.Send(ui.PhaseDoneMsg{ID: ui.PhaseContent, Err: fatalErr})
			return
		}
		p.Send(ui.PhaseDoneMsg{ID: ui.PhaseContent})

		// Phase: company
		companySynced := false
		if plan.companyNeeded {
			p.Send(ui.PhaseStartMsg{ID: ui.PhaseCompany})
			if !strings.HasPrefix(plan.cfg.CompanyRepo, "https://") {
				p.Send(ui.PhaseDoneMsg{ID: ui.PhaseCompany, Err: fmt.Errorf("company repo must use https://")})
			} else {
				companyCache := filepath.Join(plan.paths.CacheDir, "company")
				repoURL := strings.TrimRight(plan.cfg.CompanyRepo, "/")
				companyArchive := repoURL + "/archive/refs/heads/main.zip"
				if err := sapSync.FetchArchive(companyArchive, companyCache, plan.token); err != nil {
					p.Send(ui.PhaseDoneMsg{ID: ui.PhaseCompany, Err: err})
				} else {
					companySynced = true
					p.Send(ui.PhaseDoneMsg{ID: ui.PhaseCompany})
				}
			}
		}

		// Phase: markers
		p.Send(ui.PhaseStartMsg{ID: ui.PhaseMarkers})
		if err := runMarkerExpansion(plan.officialCache, plan.engine, p); err != nil {
			p.Send(ui.PhaseDoneMsg{ID: ui.PhaseMarkers, Err: err})
		} else {
			p.Send(ui.PhaseDoneMsg{ID: ui.PhaseMarkers})
		}

		// Changelog (hidden — no phase messages)
		changelogDirs := []string{filepath.Join(plan.officialCache, "content", "packs")}
		if companySynced {
			companyCache := filepath.Join(plan.paths.CacheDir, "company")
			changelogDirs = append(changelogDirs, filepath.Join(companyCache, "content", "packs"))
		}
		syncedAt := time.Now()
		clEntries, err := sapSync.CollectChangelog(changelogDirs)
		if err != nil {
			p.Send(ui.WarnMsg{Text: fmt.Sprintf("changelog collection warning: %v", err)})
		}
		if writeErr := sapSync.WriteChangelog(plan.paths.CacheDir, syncedAt, clEntries); writeErr != nil {
			p.Send(ui.WarnMsg{Text: fmt.Sprintf("changelog write warning: %v", writeErr)})
		}
	}

	// --- Independent phases ---
	phases := buildIndepPhases(plan)
	for _, ip := range phases {
		p.Send(ui.PhaseStartMsg{ID: ip.id})
		_, err := ip.fn()
		if err != nil {
			p.Send(ui.PhaseDoneMsg{ID: ip.id, Err: err})
		} else {
			_ = plan.engine.MarkSynced(ip.cat)
			p.Send(ui.PhaseDoneMsg{ID: ip.id})
		}
	}
}

// buildIndepPhases returns the list of independent phases to run.
func buildIndepPhases(plan *syncPlan) []indepPhase {
	var phases []indepPhase
	for _, cat := range plan.activeIndep {
		pid, ok := plan.indepPhases[cat]
		if !ok {
			continue
		}
		cat := cat // capture loop variable
		var fn func() (string, error)
		switch cat {
		case "events":
			fn = func() (string, error) {
				return "", runEventsFetch(plan.paths.CacheDir, plan.officialCache, plan.force)
			}
		case "youtube":
			fn = func() (string, error) {
				return "", runYouTubeFetch(plan.paths.CacheDir, plan.officialCache, plan.cfg.CompanyRepo, plan.paths.ConfigDir, plan.force, plan.cfg.Sync.YouTube)
			}
		case "news":
			fn = func() (string, error) {
				return "", runNewsFetch(plan.paths.CacheDir, plan.officialCache, plan.paths.ConfigDir, plan.force, plan.cfg.Sync.News)
			}
		case "discovery":
			fn = func() (string, error) {
				return "", runDiscoveryFetch(plan.paths.CacheDir, plan.force)
			}
		case "tutorials":
			fn = func() (string, error) {
				return "", runTutorialsFetch(plan.paths.CacheDir, plan.force)
			}
		case "learning":
			fn = func() (string, error) {
				return "", runLearningFetch(plan.paths.CacheDir, plan.force)
			}
		default:
			continue
		}
		phases = append(phases, indepPhase{cat: cat, id: pid, fn: fn})
	}
	return phases
}

// countMarkerSlots scans packs for fetch markers and returns the total count,
// so the Bubbletea model can pre-allocate view lines and avoid height changes.
func countMarkerSlots(officialCache string) int {
	packsDir := filepath.Join(officialCache, "content", "packs")
	entries, err := os.ReadDir(packsDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(packsDir, entry.Name(), "context.md"))
		if err != nil {
			continue
		}
		markers, _ := sapSync.ScanMarkers(entry.Name(), string(data))
		count += len(markers)
	}
	return count
}

// runSyncTTY runs the sync with the Bubbletea inline progress display.
func runSyncTTY(plan *syncPlan) error {
	markerSlots := countMarkerSlots(plan.officialCache)
	p, model := ui.RunSyncProgress(plan.visiblePhases, markerSlots)

	// Capture stderr during Bubbletea to prevent cursor corruption.
	// On Windows, stderr and stdout share the console buffer; any write
	// to stderr shifts the cursor and leaves ghost lines in the display.
	var stderrBuf bytes.Buffer
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	done := make(chan struct{})
	go func() {
		io.Copy(&stderrBuf, r)
		close(done)
	}()

	go syncWorker(plan, p)

	_, runErr := p.Run()

	// Restore stderr and flush captured output.
	w.Close()
	<-done
	os.Stderr = origStderr
	if stderrBuf.Len() > 0 {
		stderrBuf.WriteTo(os.Stderr)
	}
	for _, warn := range model.Warnings() {
		fmt.Fprintf(os.Stderr, "sap-devs: %s\n", warn)
	}

	if runErr != nil {
		return fmt.Errorf("progress display: %w", runErr)
	}
	return model.FatalErr()
}

// runSyncPlain runs the sync with plain text progress output (non-TTY).
func runSyncPlain(plan *syncPlan, out io.Writer) error {
	fmt.Fprintln(out, i18n.T(i18n.ActiveLang, "sync.syncing"))

	// --- Archive block ---
	if plan.archiveNeeded {
		if err := sapSync.FetchArchive(officialRepoArchive, plan.officialCache, plan.token); err != nil {
			ui.PrintPlainProgress(out, ui.PhaseContent, "failed", "", err)
			return fmt.Errorf("sync official content: %w", err)
		}
		if err := plan.engine.MarkAllSynced(plan.activeArchive); err != nil {
			ui.PrintPlainProgress(out, ui.PhaseContent, "failed", "", err)
			return err
		}
		ui.PrintPlainProgress(out, ui.PhaseContent, "done", "", nil)

		// Company
		companySynced := false
		if plan.companyNeeded {
			if !strings.HasPrefix(plan.cfg.CompanyRepo, "https://") {
				ui.PrintPlainProgress(out, ui.PhaseCompany, "failed", "", fmt.Errorf("company repo must use https://"))
			} else {
				companyCache := filepath.Join(plan.paths.CacheDir, "company")
				repoURL := strings.TrimRight(plan.cfg.CompanyRepo, "/")
				companyArchive := repoURL + "/archive/refs/heads/main.zip"
				if err := sapSync.FetchArchive(companyArchive, companyCache, plan.token); err != nil {
					ui.PrintPlainProgress(out, ui.PhaseCompany, "failed", "", err)
				} else {
					companySynced = true
					ui.PrintPlainProgress(out, ui.PhaseCompany, "done", "", nil)
				}
			}
		}

		// Markers (silent in plain mode — no Bubbletea program)
		if err := runMarkerExpansion(plan.officialCache, plan.engine, nil); err != nil {
			ui.PrintPlainProgress(out, ui.PhaseMarkers, "failed", "", err)
		} else {
			ui.PrintPlainProgress(out, ui.PhaseMarkers, "done", "", nil)
		}

		// Changelog (hidden)
		changelogDirs := []string{filepath.Join(plan.officialCache, "content", "packs")}
		if companySynced {
			companyCache := filepath.Join(plan.paths.CacheDir, "company")
			changelogDirs = append(changelogDirs, filepath.Join(companyCache, "content", "packs"))
		}
		syncedAt := time.Now()
		clEntries, err := sapSync.CollectChangelog(changelogDirs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: changelog collection warning: %v\n", err)
		}
		if writeErr := sapSync.WriteChangelog(plan.paths.CacheDir, syncedAt, clEntries); writeErr != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: changelog write warning: %v\n", writeErr)
		}
	}

	// --- Independent phases ---
	phases := buildIndepPhases(plan)
	for _, ip := range phases {
		err := func() error { _, e := ip.fn(); return e }()
		if err != nil {
			ui.PrintPlainProgress(out, ip.id, "failed", "", err)
			fmt.Fprintf(os.Stderr, "sap-devs: %s sync: %v\n", ip.cat, err)
		} else {
			_ = plan.engine.MarkSynced(ip.cat)
			ui.PrintPlainProgress(out, ip.id, "done", "", nil)
		}
	}

	return nil
}

// runMarkerExpansion scans all official-layer packs for sync:fetch markers,
// fetches them in parallel, and writes context.expanded.md alongside each context.md.
// When p is non-nil, it sends SetMarkersMsg and MarkerDoneMsg to the Bubbletea program.
// When p is nil (plain text path), markers are fetched silently.
func runMarkerExpansion(officialCache string, engine *sapSync.Engine, p *tea.Program) error {
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
			if p != nil {
				p.Send(ui.WarnMsg{Text: w})
			} else {
				fmt.Fprintf(os.Stderr, "sap-devs: %s\n", w)
			}
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

	// Build marker items for UI and send to Bubbletea
	if p != nil {
		items := make([]ui.MarkerItem, len(allMarkers))
		for i, m := range allMarkers {
			label := m.Label
			if label == "" {
				label = m.URL
			}
			items[i] = ui.MarkerItem{
				PackID: m.PackID,
				Index:  m.Index,
				Label:  label,
				State:  "fetching",
			}
		}
		p.Send(ui.SetMarkersMsg{Items: items})
	}

	// Fetch all markers in parallel (max 4 concurrent)
	results := make(map[string]string)
	fetchErrs := make(map[string]error)
	var mu gosync.Mutex
	sem := make(chan struct{}, 4)
	var wg gosync.WaitGroup

	for _, m := range allMarkers {
		wg.Add(1)
		go func(m sapSync.Marker) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			content, err := sapSync.FetchMarker(m, nil)
			label := m.Label
			if label == "" {
				label = m.URL
			}
			key := m.PackID + "::" + strconv.Itoa(m.Index)
			if err != nil {
				mu.Lock()
				fetchErrs[key] = err
				mu.Unlock()
				if p != nil {
					p.Send(ui.MarkerDoneMsg{PackID: m.PackID, Index: m.Index, Label: label, Err: err})
				}
				return
			}
			mu.Lock()
			results[key] = content
			mu.Unlock()
			lineCount := strings.Count(content, "\n") + 1
			if p != nil {
				p.Send(ui.MarkerDoneMsg{PackID: m.PackID, Index: m.Index, Label: label, Lines: lineCount})
			}
		}(m)
	}
	wg.Wait()

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
			if c, ok := results[key]; ok {
				packResults[m.Index] = c
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
	return []string{"tips", "tools", "resources", "context", "mcp", "advocates", "events", "youtube", "news", "discovery", "tutorials", "learning"}
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

func runNewsFetch(cacheDir, officialCache, configDir string, force bool, ttl time.Duration) error {
	if !force {
		age := news.CacheAge(cacheDir)
		if age >= 0 && age < ttl {
			return nil
		}
	}

	// Try RSS feed with retry.
	episodes, rssErr := youtube.FetchPlaylistRetry(newsPlaylistRSS, 3)

	// If RSS fails, try API v3 as fallback (requires key).
	if rssErr != nil {
		apiKey := credentials.ResolveService(configDir, "youtube", []string{"YOUTUBE_API_KEY"})
		if apiKey != "" {
			var apiErr error
			episodes, apiErr = youtube.FetchPlaylistAPI(newsPlaylistID, apiKey)
			if apiErr != nil {
				fmt.Fprintf(os.Stderr, "sap-devs: news API fallback failed: %v\n", apiErr)
			}
		}
	}

	if episodes == nil {
		// If both live fetches failed, try the pre-fetched baseline from the content repo.
		if baseline, ok := news.LoadBaseline(officialCache); ok {
			_ = news.SaveCache(cacheDir, baseline)
			return nil
		}
		// Leave existing stale cache in place if present.
		if rssErr != nil {
			fmt.Fprintf(os.Stderr, "sap-devs: news fetch failed (stale cache preserved): %v\n", rssErr)
		}
		return nil
	}

	posts, _ := community.FetchBlogPosts(newsCommunityRSS)
	items := news.Correlate(episodes, posts)
	return news.SaveCache(cacheDir, items)
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

	// Tutorials use the public GitHub API (github.com/sap-tutorials org).
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
