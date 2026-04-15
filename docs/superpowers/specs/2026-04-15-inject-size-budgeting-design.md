# Inject Size Budgeting — Design Spec

**Date:** 2026-04-15
**Status:** Approved
**Repo:** sap-devs-cli

---

## Overview

The `inject` pipeline currently renders all loaded packs into a single Markdown string and writes it identically to every adapter. As the content stack grows — more packs, expanded content from sync:fetch markers, company overlays — the injected section will bloat and eventually hit token or file-size limits in AI tools, while also reducing signal-to-noise for users who only care about a subset of SAP domains.

This spec covers three coordinated improvements:

1. **Per-adapter token budgets** — adapters declare `max_tokens`; the engine trims to fit before injecting.
2. **Overlap deduplication** — packs declare which other packs subsume their content; the engine drops redundant lower-weight packs before trimming.
3. **Profiling output** — a `--stats` flag on `inject` reports what is being injected per adapter and how much budget remains.

Lazy-loading (injecting URL pointers instead of inline content) was considered but dropped — the sync:fetch reference-expansion feature already solves the "fresh content without authoring burden" problem, and lazy-loading would add runtime network dependencies without additional benefit.

---

## Data Model Changes

### `pack.yaml` — new `overlaps` field

```yaml
id: btp-core
overlaps:
  - cap   # if cap is present at higher weight, btp-core content is redundant
```

A pack declares which other packs subsume its content. Semantics: "if any pack in my `overlaps` list is present at a higher weight, I can be dropped during deduplication." The relationship is directional — only the lower-weight pack carries the declaration.

**Go changes — `internal/content/pack.go`:**

```go
type packMeta struct {
    // ... existing fields
    Overlaps []string `yaml:"overlaps,omitempty"`
}

type Pack struct {
    // ... existing fields
    Overlaps []string
}
```

### `Adapter` struct — new `max_tokens` field

**`internal/adapter/adapter.go`:**

```go
type Adapter struct {
    // ... existing fields
    MaxTokens int `yaml:"max_tokens,omitempty"` // 0 = unconstrained
}
```

Zero value means unconstrained — all existing adapter YAMLs work without modification. Budgets are added per-adapter YAML as needed.

---

## `TrimPacks` Function

New function in `internal/content/render.go`, alongside `RenderContext`:

```go
// TrimPacks filters packs to fit within maxBytes, applying overlap deduplication
// and pack-level budget enforcement. Pass maxBytes=0 for unconstrained.
// Packs must already be sorted by weight descending (LoadPacks guarantees this).
func TrimPacks(packs []*Pack, maxBytes int) []*Pack {
    // Pass 1 — deduplication
    // A pack is dropped if a higher-weight pack it overlaps with is already included.
    included := make(map[string]bool)
    var deduped []*Pack
    for _, p := range packs {
        dominated := false
        for _, overlapID := range p.Overlaps {
            if included[overlapID] {
                dominated = true
                break
            }
        }
        if !dominated {
            deduped = append(deduped, p)
            included[p.ID] = true
        }
    }

    // Pass 2 — budget enforcement
    if maxBytes <= 0 {
        return deduped
    }
    var result []*Pack
    used := 0
    for _, p := range deduped {
        size := len(p.ContextMD)
        if used+size > maxBytes {
            break
        }
        result = append(result, p)
        used += size
    }
    return result
}
```

**Algorithm decisions:**

- **Dedup iterates high→low weight.** Packs are pre-sorted by `LoadPacks`; higher-weight packs enter the `included` set first. A lower-weight pack whose `overlaps` lists an already-included ID is dropped.
- **Budget uses `break`, not `continue`.** Stops at the first pack that doesn't fit, preserving priority order. A lower-priority small pack does not sneak in ahead of a higher-priority large pack that was excluded.
- **Budget unit is bytes.** Stored as `max_tokens` in YAML for author readability; converted to bytes as `MaxTokens * 4` at render time (1 token ≈ 4 bytes approximation). `int` is 64-bit on all supported platforms (Linux/macOS/Windows amd64/arm64), so overflow is not a concern for any realistic token budget.
- **Dedup always runs**, even when unconstrained — it is cheap and semantically always correct.
- **Header overhead ignored.** The ~100-byte `RenderContext` header is well within any reasonable budget tolerance.
- **Empty `ContextMD`.** A pack with no context file has `len(p.ContextMD) == 0`, so it always fits within any budget and passes through both passes. This is intentional: `RenderContext` already skips empty-context packs, so including them is harmless.
- **Zero-pack result.** If budget enforcement drops every pack, `TrimPacks` returns `nil`. The engine must detect this and emit a warning to `opts.Out` (e.g. `"sap-devs: adapter %s: budget too small to include any pack content"`) rather than silently injecting a header-only block.

---

## Engine Refactor

### Before

`Engine` held a single pre-rendered `context string` used identically for all adapters:

```go
type Engine struct {
    adapters []Adapter
    context  string
    opts     Options
}
```

### After

`Engine` holds packs and profile, rendering per-adapter with its own budget:

**`internal/adapter/engine.go`:**

```go
type Engine struct {
    adapters []Adapter
    packs    []*content.Pack
    profile  *content.Profile
    opts     Options
}

func NewEngine(adapters []Adapter, packs []*content.Pack, profile *content.Profile, opts Options) *Engine {
    return &Engine{adapters: adapters, packs: packs, profile: profile, opts: opts}
}
```

**`Run()` — per-adapter render:**

```go
func (e *Engine) Run() error {
    for _, a := range e.adapters {
        if e.opts.ToolFilter != "" && a.ID != e.opts.ToolFilter {
            continue
        }
        maxBytes := a.MaxTokens * 4
        trimmed := content.TrimPacks(e.packs, maxBytes)
        if len(trimmed) == 0 && maxBytes > 0 {
            fmt.Fprintf(e.opts.Out, "sap-devs: adapter %s: budget too small to include any pack content\n", a.ID)
            continue
        }
        ctx := content.RenderContext(trimmed, e.profile)

        switch a.Type {
        case "file-inject":
            if err := e.runFileInject(a, ctx); err != nil {
                return fmt.Errorf("adapter %s: %w", a.ID, err)
            }
        case "clipboard-export":
            if e.opts.Scope == "project" {
                continue
            }
            if err := ExportToClipboard(ctx, a.Instructions, e.opts.DryRun); err != nil {
                return fmt.Errorf("adapter %s: %w", a.ID, err)
            }
        case "mcp-wire":
            // handled by mcp command
        }
    }
    return nil
}
```

`runFileInject` gains a `ctx string` parameter instead of reading `e.context`.

### `cmd` layer changes

**`cmd/inject.go`:** Remove the pre-render line and pass packs + profile to the engine:

```go
// Remove:
rendered := content.RenderContext(packs, activeProfile)

// Change:
eng, err := newAdapterEngine(packs, activeProfile, opts)
```

**`cmd/root.go` — `newAdapterEngine`:**

```go
func newAdapterEngine(packs []*content.Pack, profile *content.Profile, opts adapter.Options) (*adapter.Engine, error) {
    allAdapters, err := loadAdapters()
    if err != nil {
        return nil, err
    }
    return adapter.NewEngine(allAdapters, packs, profile, opts), nil
}
```

---

## Profiling Output (`--stats`)

New flag on `sap-devs inject`:

```
$ sap-devs inject --stats --dry-run

Adapter         Packs included          Tokens (approx)   Budget
claude-code     cap, btp-core, abap     ~750              unconstrained
cursor          cap, btp-core, abap     ~750              unconstrained
```

Once budgets are configured:

```
Adapter         Packs included   Tokens (approx)   Budget        Status
claude-code     cap, btp-core    ~500              2000 tokens   OK
cursor          cap              ~250              500 tokens    OK (1 pack trimmed)
```

**`Options` struct change** to thread the writer and flag:

```go
type Options struct {
    Scope      string
    ToolFilter string
    DryRun     bool
    Stats      bool
    Out        io.Writer // for stats output; nil → io.Discard
}
```

`NewEngine` normalises a nil `Out` to `io.Discard` so all write paths are safe without nil guards throughout `Run()`.

**`adapterStats` struct** (internal to `engine.go`):

```go
type adapterStats struct {
    AdapterID    string
    PackIDs      []string // IDs of packs included after TrimPacks
    ApproxTokens int      // len(rendered) / 4
    BudgetTokens int      // adapter.MaxTokens; 0 = unconstrained
    Trimmed      bool     // true if any packs were dropped by TrimPacks
}
```

**Stats collection and printing:**

- `Run()` builds `[]adapterStats` internally, appending one entry per processed adapter.
- After iterating all adapters, if `e.opts.Stats` is true, `Run()` prints the table to `e.opts.Out` using `text/tabwriter`.
- The table is printed whether or not `--dry-run` is set.

**Behaviour:**

- `--stats` can be combined with or without `--dry-run`. With `--dry-run` it previews and reports; without it, it injects and reports.
- Token approximation: `len(rendered) / 4`, displayed as `~N`.
- Stats are printed to `Out` (set to `cmd.OutOrStdout()` by the inject command).

---

## Files Changed

| File | Change |
|---|---|
| `internal/content/pack.go` | Add `Overlaps []string` to `Pack` and `packMeta` |
| `internal/content/render.go` | Add `TrimPacks(packs []*Pack, maxBytes int) []*Pack` |
| `internal/adapter/adapter.go` | Add `MaxTokens int` to `Adapter` |
| `internal/adapter/adapter_test.go` | Update all `NewEngine` call sites to new four-argument signature |
| `internal/adapter/engine.go` | Replace `context string` with `packs + profile`; render per-adapter; add stats collection |
| `cmd/inject.go` | Remove pre-render; wire `--stats` flag; pass packs+profile to engine |
| `cmd/inject_test.go` | Update `NewEngine` call sites and remove `RenderContext` pre-render calls |
| `cmd/root.go` | Update `newAdapterEngine` signature |
| `content/adapters/*.yaml` | Add `max_tokens` to adapters where budgets are known (follow-on) |
| `content/packs/*/pack.yaml` | Add `overlaps` where real overlap exists (follow-on) |

---

## Testing

- **`TrimPacks` unit tests** — dedup only (no budget), budget only (no overlaps), dedup + budget combined, zero budget (unconstrained passthrough), empty input.
- **Engine integration** — verify per-adapter context differs when adapters have different `MaxTokens`.
- **Stats output** — verify table format and token approximation via captured `io.Writer`.
- All existing inject tests continue to pass (zero-value `MaxTokens` preserves current behaviour).
