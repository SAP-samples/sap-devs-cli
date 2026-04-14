# Multi-lingual Support Design

**Date:** 2026-04-14
**Status:** Approved

## Overview

Add a unified i18n infrastructure to `sap-devs` that covers both CLI output strings and content pack files. The system uses embedded JSON catalogs with no external dependencies. Language is resolved from config, then system locale, then falls back to English. German ships as a pilot alongside the infrastructure.

## Decisions

| Question | Decision |
|---|---|
| Scope | Both CLI output and content/packs, in one unified system |
| Language detection | Config `language` field → `LANG`/`LC_ALL` env var → `en` |
| Missing translations | Silent fallback to English |
| Content pack translations | Official repo ships translations; user/company layers can add more |
| Launch languages | Infrastructure + German (`de`) pilot |
| i18n approach | Option B — minimal embedded JSON catalogs, zero new dependencies |

## Architecture

### `internal/i18n` Package

New package with three responsibilities:

**Language resolution:**
```go
// Resolve returns the active language tag.
// Priority: cfg (config.yaml language field) → LANG env var → "en".
// If the resolved language has no catalog, falls back to "en".
func Resolve(cfgLang string) string
```

Parsing strips encoding and region suffixes: `de_AT.UTF-8` → `de`.

**Package-level active language:**
```go
var ActiveLang string  // set once in rootCmd.PersistentPreRunE
```

**String lookup:**
```go
// T looks up key in the catalog for lang.
// Supports {{.Var}} substitution via text/template when data is provided.
// Falls back to en.json if key absent in lang catalog.
// Falls back to raw key string if absent in both (never panics).
func T(lang, key string, data ...any) string
```

**Embedded catalogs:**
```
internal/i18n/
  catalogs/
    en.json    ← authoritative source of all keys
    de.json    ← German pilot
  i18n.go
```

Catalog format:
```json
{
  "root.short": "AI-first SAP developer toolkit",
  "inject.short": "Push SAP context into your AI tools",
  "inject.done": "Injected context into {{.Count}} tool(s)"
}
```

Catalogs are embedded via `//go:embed catalogs/*.json` and loaded into a `map[string]map[string]string` at package init.

### Config Integration

`config.Config` gains a `Language` field:
```go
type Config struct {
    CompanyRepo string     `yaml:"company_repo,omitempty"`
    Language    string     `yaml:"language,omitempty"`  // e.g. "de" — empty means auto-detect
    Sync        SyncConfig `yaml:"sync"`
}
```

Empty string means auto-detect from system locale. Existing configs require no migration.

**Setting the language:**
```
sap-devs config set language de
```
Uses the existing `config set` key/value mechanism — no new command needed.

**Initialisation in `rootCmd.PersistentPreRunE`:**
```go
i18n.ActiveLang = i18n.Resolve(cfg.Language)
```
Runs before any command body, so all commands see the resolved language.

### CLI String Migration

Two categories of strings:

**Cobra metadata** (`Short`, `Long`, `Use`) — set at `init()` time before `PersistentPreRunE` runs. A `localizeCommands(lang string)` helper is called from `PersistentPreRunE`; it walks all registered commands and mutates their display strings. This is safe because cobra reads these fields at help-render time, not at registration time.

**Runtime output** — replaced with `i18n.T` at each call site:
```go
// before
fmt.Fprintf(cmd.OutOrStdout(), "Injected context into %d tool(s)\n", count)

// after
fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "inject.done", map[string]any{"Count": count}))
```

**German pilot scope for CLI strings:**
- All `Short`/`Long` command descriptions (highest visibility)
- Runtime output for `inject`, `sync`, `tip`, and `doctor` commands

All other commands fall back to English silently. The English catalog (`en.json`) is the complete authoritative key set.

### Content Pack Translation

**File naming convention** — locale-suffixed files alongside base English files:
```
content/packs/cap/
  pack.yaml
  context.md        ← English base
  context.de.md     ← German translation
  tips.md           ← English base
  tips.de.md        ← German translation
  resources.yaml    ← not translated
  tools.yaml        ← not translated
  mcp.yaml          ← not translated
```

**`LoadPack` change** — accepts an optional `lang string` parameter. When loading `context.md` and `tips.md`, tries `<file>.<lang>.md` first; falls back to base file silently if absent. No change to the `Pack` struct or any consumer.

**`pack.yaml` metadata** — inline locale keys for `name` and `description`:
```yaml
id: cap
name: SAP Cloud Application Programming Model
name.de: SAP Cloud Application Programming Modell
description: Node.js and Java framework for cloud-native BTP apps
description.de: Node.js- und Java-Framework für Cloud-native BTP-Anwendungen
```

`packMeta` gains optional locale maps; `LoadPack` selects the right value after unmarshalling.

**`ContentLoader.LoadPacks`** — passes `i18n.ActiveLang` down to `LoadPack`.

**German pilot scope for content:** `context.de.md` and `tips.de.md` for the `cap` pack only.

## Data Flow

```
rootCmd.PersistentPreRunE
  → config.Load()
  → i18n.Resolve(cfg.Language)      sets i18n.ActiveLang
  → localizeCommands(i18n.ActiveLang)  patches cobra Short/Long

command body
  → i18n.T(i18n.ActiveLang, "key")  for runtime output
  → ContentLoader.LoadPacks(profile, i18n.ActiveLang)
      → LoadPack(packDir, i18n.ActiveLang)
          → tries context.<lang>.md, falls back to context.md
          → tries tips.<lang>.md, falls back to tips.md
          → selects name.<lang> / description.<lang> from pack.yaml
```

## Error Handling

- Missing translation key → silent fallback to `en.json` → fallback to raw key string (never panics, never errors)
- Unknown language (no catalog) → silent fallback to `en`
- Missing locale content file → silent fallback to base English file
- No `language` in config + no `LANG` env var → `en`

## Testing

**`internal/i18n` unit tests:**
- `Resolve`: config value wins over env var; env var used when config empty; `de_AT.UTF-8` parses to `de`; unknown language falls back to `en`
- `T`: known key returns translation; missing key falls back to `en`; missing in both returns raw key; template substitution works

**`LoadPack` tests:**
- Locale file present → locale content loaded
- Locale file absent → base file loaded silently
- `pack.yaml` locale key present → localised name/description returned
- `pack.yaml` locale key absent → base name/description returned

**`config.Config` tests:**
- `Language` field round-trips through marshal/unmarshal

## Out of Scope (this iteration)

- Plural rules, gender inflection, RTL layout
- Translations for any language other than German
- Content pack translations beyond the `cap` pack
- CLI output for commands other than inject/sync/tip/doctor
- Contributor tooling (translation workflow, string extraction scripts)
