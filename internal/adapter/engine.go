// internal/adapter/engine.go
package adapter

import "fmt"

// Options controls inject scope, filtering, and dry-run behaviour.
type Options struct {
	Scope      string // "global" | "project"
	ToolFilter string // if non-empty, only run this adapter ID
	DryRun     bool
}

// Engine runs injection for a set of adapters with a given rendered context.
type Engine struct {
	adapters []Adapter
	context  string
	opts     Options
}

// NewEngine constructs an Engine.
func NewEngine(adapters []Adapter, renderedContext string, opts Options) *Engine {
	return &Engine{adapters: adapters, context: renderedContext, opts: opts}
}

// Run dispatches to the appropriate handler for each adapter.
func (e *Engine) Run() error {
	for _, a := range e.adapters {
		if e.opts.ToolFilter != "" && a.ID != e.opts.ToolFilter {
			continue
		}
		switch a.Type {
		case "file-inject":
			if err := e.runFileInject(a); err != nil {
				return fmt.Errorf("adapter %s: %w", a.ID, err)
			}
		case "clipboard-export":
			// clipboard-export is only for global scope
			if e.opts.Scope == "project" {
				continue
			}
			if err := ExportToClipboard(e.context, a.Instructions, e.opts.DryRun); err != nil {
				return fmt.Errorf("adapter %s: %w", a.ID, err)
			}
		case "mcp-wire":
			// mcp-wire is handled by the mcp command; inject skips it
		}
	}
	return nil
}

func (e *Engine) runFileInject(a Adapter) error {
	for _, target := range a.Targets {
		if target.Scope != e.opts.Scope {
			continue
		}
		path, err := ExpandHome(target.Path)
		if err != nil {
			return err
		}
		if target.Mode == "replace-section" {
			if err := ReplaceSection(path, target.Section, e.context, e.opts.DryRun); err != nil {
				return err
			}
		}
	}
	return nil
}
