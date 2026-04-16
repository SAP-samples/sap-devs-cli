// internal/adapter/engine.go
package adapter

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// Options controls inject scope, filtering, dry-run, and stats behaviour.
type Options struct {
	Scope      string                 // "global" | "project"
	ToolFilter string                 // if non-empty, only run this adapter ID
	DryRun     bool
	Stats      bool
	Out        io.Writer              // for stats/warning output; nil → io.Discard
	Dynamic    *content.DynamicContext // nil = no dynamic section
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
func (e *Engine) Run() error {
	var stats []adapterStats

	for _, a := range e.adapters {
		if e.opts.ToolFilter != "" && a.ID != e.opts.ToolFilter {
			continue
		}
		maxBytes := a.MaxBytes
		if maxBytes == 0 && a.MaxTokens > 0 {
			maxBytes = a.MaxTokens * 4
		}
		trimmed := content.TrimPacks(e.packs, maxBytes)
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
		ctx := content.RenderContext(trimmed, e.profile, e.opts.Dynamic)

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
				return fmt.Errorf("adapter %s: %w", a.ID, err)
			}
		case "clipboard-export":
			// clipboard-export is only for global scope
			if e.opts.Scope == "project" {
				continue
			}
			if err := ExportToClipboard(formattedCtx, a.Instructions, e.opts.DryRun); err != nil {
				return fmt.Errorf("adapter %s: %w", a.ID, err)
			}
		case "file-export":
			if e.opts.Scope == "project" {
				continue
			}
			if err := ExportFileAndClip(a, formattedCtx, e.opts); err != nil {
				return fmt.Errorf("adapter %s: %w", a.ID, err)
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
				Trimmed:      len(trimmed) < len(e.packs),
			})
		}
	}

	if e.opts.Stats && len(stats) > 0 {
		printStats(e.opts.Out, stats)
	}
	return nil
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
