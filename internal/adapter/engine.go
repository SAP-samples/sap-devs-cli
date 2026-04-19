// internal/adapter/engine.go
package adapter

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"
)

// RunResult holds the outcome of an Engine.Run() call.
type RunResult struct {
	Found    int   // sections/files removed in live mode
	DryFound int   // sections/files that would be removed in dry-run mode
	Err      error
}

// Options controls inject scope, filtering, dry-run, and stats behaviour.
type Options struct {
	Scope      string                 // "global" | "project"
	ToolFilter string                 // if non-empty, only run this adapter ID
	DryRun     bool
	Stats      bool
	Out        io.Writer              // for stats/warning output; nil → io.Discard
	Dynamic    *content.DynamicContext // nil = no dynamic section
	Uninstall  bool
	// Lang is the active language for i18n. Always use e.opts.Lang inside engine code.
	Lang       string
}

// Engine runs injection for a set of adapters, rendering per-adapter with its own budget.
type Engine struct {
	adapters []Adapter
	packs    []*content.Pack
	profile  *content.Profile
	opts     Options
}

// adapterStats records what was injected for one adapter.
type adapterStats struct {
	AdapterID    string
	PackIDs      []string // IDs of packs included after TrimPacks
	ApproxTokens int      // len(rendered) / 4
	BudgetBytes  int      // effective budget in bytes; 0 = unconstrained
	Format       string   // "markdown" | "plain-prose" | ""
	Trimmed      bool     // true if any packs were dropped by TrimPacks
}

// NewEngine constructs an Engine. A nil Out is normalised to io.Discard.
func NewEngine(adapters []Adapter, packs []*content.Pack, profile *content.Profile, opts Options) *Engine {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	return &Engine{adapters: adapters, packs: packs, profile: profile, opts: opts}
}

// Run dispatches to the appropriate handler for each adapter.
func (e *Engine) Run() RunResult {
	var result RunResult
	var stats []adapterStats

	for _, a := range e.adapters {
		if e.opts.ToolFilter != "" && a.ID != e.opts.ToolFilter {
			continue
		}
		if e.opts.Uninstall {
			if a.Type == "file-inject" {
				n, dn, err := e.runFileUninstall(a)
				result.Found += n
				result.DryFound += dn
				if err != nil {
					result.Err = errors.Join(result.Err, err)
				}
			}
			continue
		}
		maxBytes := a.MaxBytes
		if maxBytes == 0 && a.MaxTokens > 0 {
			maxBytes = a.MaxTokens * 4
		}
		trimmed := content.TrimPacks(e.packs, maxBytes, "full")
		// Note: when base packs exist, TrimPacks always returns at least those packs,
		// so len(trimmed) == 0 only occurs when no base packs are configured and the
		// budget is too small for all non-base packs.
		if len(trimmed) == 0 && maxBytes > 0 {
			fmt.Fprintf(os.Stderr, "sap-devs: adapter %s: budget too small to include any pack content\n", a.ID)
			if e.opts.Stats {
				stats = append(stats, adapterStats{
					AdapterID:   a.ID,
					PackIDs:     nil,
					BudgetBytes: maxBytes, // resolved value
					Format:      a.Format,
					Trimmed:     true,
				})
			}
			continue
		}
		ctx := content.RenderContext(trimmed, e.profile, e.opts.Dynamic, "full")

		// Apply format transform (skipped for file-export — ExportFileAndClip handles it internally)
		var formattedCtx string
		if a.Type != "file-export" {
			formattedCtx = content.FormatOutput(ctx, a.Format)
		} else {
			formattedCtx = ctx // raw Markdown passed to ExportFileAndClip
		}

		switch a.Type {
		case "file-inject":
			if err := e.runFileInject(a, formattedCtx); err != nil {
				return RunResult{Err: fmt.Errorf("adapter %s: %w", a.ID, err)}
			}
		case "clipboard-export":
			// clipboard-export is only for global scope
			if e.opts.Scope == "project" {
				continue
			}
			if err := ExportToClipboard(formattedCtx, a.Instructions, e.opts.DryRun); err != nil {
				return RunResult{Err: fmt.Errorf("adapter %s: %w", a.ID, err)}
			}
		case "file-export":
			if e.opts.Scope == "project" {
				continue
			}
			if err := ExportFileAndClip(a, formattedCtx, e.opts); err != nil {
				return RunResult{Err: fmt.Errorf("adapter %s: %w", a.ID, err)}
			}
		case "mcp-wire":
			// mcp-wire is handled by the mcp command; inject skips it
			continue
		}

		if e.opts.Stats {
			packIDs := make([]string, len(trimmed))
			for i, p := range trimmed {
				packIDs[i] = p.ID
			}
			stats = append(stats, adapterStats{
				AdapterID:    a.ID,
				PackIDs:      packIDs,
				ApproxTokens: len(formattedCtx) / 4,
				BudgetBytes:  maxBytes, // resolved value (MaxBytes or MaxTokens*4)
				Format:       a.Format,
				// Trimmed is true when any pack was dropped. With base packs always
				// surviving TrimPacks, this correctly reflects whether non-base packs
				// were dropped by budget or deduplication.
				Trimmed:      len(trimmed) < len(e.packs),
			})
		}
	}

	if e.opts.Stats && len(stats) > 0 {
		printStats(e.opts.Out, stats)
	}
	return result
}

func (e *Engine) runFileInject(a Adapter, ctx string) error {
	for _, target := range a.Targets {
		if target.Scope != e.opts.Scope {
			continue
		}
		path, err := ExpandHome(target.Path)
		if err != nil {
			return fmt.Errorf("target %s: %w", target.Path, err)
		}
		switch target.Mode {
		case "replace-section":
			if err := ReplaceSection(path, target.Section, ctx, e.opts.DryRun); err != nil {
				return fmt.Errorf("target %s: %w", target.Path, err)
			}
		case "replace-file":
			if err := ReplaceFile(path, target.Preamble, ctx, e.opts.DryRun); err != nil {
				return fmt.Errorf("target %s: %w", target.Path, err)
			}
		default:
			return fmt.Errorf("target %s: unknown mode %q", target.Path, target.Mode)
		}
	}
	return nil
}

func (e *Engine) runFileUninstall(a Adapter) (found, dryFound int, err error) {
	for _, target := range a.Targets {
		if target.Scope != e.opts.Scope {
			continue
		}
		path, expandErr := ExpandHome(target.Path)
		if expandErr != nil {
			err = errors.Join(err, fmt.Errorf("target %s: %w", target.Path, expandErr))
			continue
		}
		switch target.Mode {
		case "replace-section":
			f, removed, rerr := removeSection(path, target.Section, e.opts.DryRun, e.opts.Out)
			if rerr != nil {
				err = errors.Join(err, fmt.Errorf("target %s: %w", target.Path, rerr))
				continue
			}
			if f && removed {
				found++
				fmt.Fprintf(e.opts.Out, "  %s  — %s\n", path, i18n.T(e.opts.Lang, "inject.uninstall.section_removed"))
			} else if f && !removed {
				dryFound++
				// [dry-run] line already written by removeSection
			} else {
				fmt.Fprintf(e.opts.Out, "  %s  — %s\n", path, i18n.T(e.opts.Lang, "inject.uninstall.not_found"))
			}
		case "replace-file":
			f, deleted, rerr := deleteFile(path, e.opts.DryRun, e.opts.Out)
			if rerr != nil {
				err = errors.Join(err, fmt.Errorf("target %s: %w", target.Path, rerr))
				continue
			}
			if f && deleted {
				found++
				fmt.Fprintf(e.opts.Out, "  %s  — %s\n", path, i18n.T(e.opts.Lang, "inject.uninstall.file_deleted"))
			} else if f && !deleted {
				dryFound++
				// [dry-run] line already written by deleteFile
			} else {
				fmt.Fprintf(e.opts.Out, "  %s  — %s\n", path, i18n.T(e.opts.Lang, "inject.uninstall.not_found"))
			}
		case "append":
			fmt.Fprintf(os.Stderr, "%s\n", i18n.Tf(e.opts.Lang, "inject.uninstall.append_warning", map[string]any{"Path": path}))
		default:
			err = errors.Join(err, fmt.Errorf("target %s: unknown mode %q", target.Path, target.Mode))
		}
	}
	return found, dryFound, err
}

// renderSectionContent renders the content string that would be written by inject
// for the given adapter. It mirrors the full pipeline in Run(): TrimPacks →
// RenderContext → FormatOutput. Returns "" when e.packs is nil.
func (e *Engine) renderSectionContent(a Adapter) string {
	if e.packs == nil {
		return ""
	}
	maxBytes := a.MaxBytes
	if maxBytes == 0 && a.MaxTokens > 0 {
		maxBytes = a.MaxTokens * 4
	}
	trimmed := content.TrimPacks(e.packs, maxBytes, "full")
	if len(trimmed) == 0 && maxBytes > 0 {
		return ""
	}
	ctx := content.RenderContext(trimmed, e.profile, e.opts.Dynamic, "full")
	return content.FormatOutput(ctx, a.Format)
}

// RenderSectionContentForTest exposes renderSectionContent for white-box tests.
// Do not call this from production code.
func (e *Engine) RenderSectionContentForTest(a Adapter) string {
	return e.renderSectionContent(a)
}

// Status inspects each file-inject adapter target and returns one StatusRow per
// (adapter, target) pair for the configured scope.
func (e *Engine) Status() ([]StatusRow, error) {
	var rows []StatusRow
	var err error

	for _, a := range e.adapters {
		if e.opts.ToolFilter != "" && a.ID != e.opts.ToolFilter {
			continue
		}
		if a.Type != "file-inject" {
			continue
		}
		for _, target := range a.Targets {
			if target.Scope != e.opts.Scope {
				continue
			}
			row := StatusRow{
				AdapterName:   a.Name,
				AdapterID:     a.ID,
				Scope:         target.Scope,
				TargetPath:    target.Path,
				OtherSections: []SectionInfo{},
			}

			path, expandErr := ExpandHome(target.Path)
			if expandErr != nil {
				err = errors.Join(err, fmt.Errorf("target %s: %w", target.Path, expandErr))
				rows = append(rows, row)
				continue
			}

			fileBytes, readErr := os.ReadFile(path)
			if readErr != nil {
				if !os.IsNotExist(readErr) {
					err = errors.Join(err, fmt.Errorf("target %s: %w", target.Path, readErr))
				}
				rows = append(rows, row)
				continue
			}

			row.FileExists = true
			fileStr := string(fileBytes)

			switch target.Mode {
			case "replace-section":
				startMarker := fmt.Sprintf(markerFmt, target.Section)
				endMarker := fmt.Sprintf(markerEndFmt, target.Section)
				startIdx, endIdx, sStatus := findSection(fileStr, startMarker, endMarker)
				switch sStatus {
				case sectionFound:
					row.Injected = true
					innerStart := startIdx + len(startMarker) + 1 // +1 for the \n after the marker
					if innerStart > endIdx {
						innerStart = endIdx
					}
					// Staleness check
					if e.packs != nil {
						rendered := e.renderSectionContent(a)
						// Extract on-disk inner content: bytes after startMarker+"\n" up to endIdx
						onDisk := fileStr[innerStart:endIdx]
						row.Stale = strings.TrimSpace(rendered) != strings.TrimSpace(onDisk)
					}
					row.SapDevsTokens = EstimateTokens(fileStr[innerStart:endIdx])
				case sectionOrphaned:
					row.Orphaned = true
				}
			case "replace-file":
				row.Injected = true
				if e.packs != nil {
					rendered := e.renderSectionContent(a)
					var expected string
					if target.Preamble != "" {
						expected = target.Preamble + "\n" + rendered
					} else {
						expected = rendered
					}
					row.Stale = strings.TrimSpace(expected) != strings.TrimSpace(fileStr)
				}
				row.SapDevsTokens = EstimateTokens(fileStr)
			case "append":
				fmt.Fprintf(os.Stderr, "%s\n", i18n.Tf(e.opts.Lang, "inject.status.append_warning", map[string]any{"Path": path}))
				continue
			}

			// Stretch-goal fields
			row.FileSizeBytes = len(fileBytes)
			row.FileTokenEst = EstimateTokens(fileStr)
			row.OtherSections = ScanOtherSections(fileStr)

			rows = append(rows, row)
		}
	}

	return rows, err
}

func printStats(w io.Writer, stats []adapterStats) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "Adapter\tPacks included\tTokens (approx)\tBudget (bytes)\tFormat\tStatus")
	for _, s := range stats {
		budget := "unconstrained"
		if s.BudgetBytes > 0 {
			budget = fmt.Sprintf("%d bytes", s.BudgetBytes)
		}
		packs := strings.Join(s.PackIDs, ", ")
		if packs == "" {
			packs = "(none)"
		}
		format := s.Format
		if format == "" {
			format = "markdown"
		}
		status := ""
		if s.Trimmed {
			status = "trimmed"
		}
		fmt.Fprintf(tw, "%s\t%s\t~%d\t%s\t%s\t%s\n",
			s.AdapterID, packs, s.ApproxTokens, budget, format, status)
	}
	tw.Flush()
}
