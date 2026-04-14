# Multi-lingual Support Design

**Date:** 2026-04-14
**Status:** Approved

## Overview

Add a unified i18n infrastructure to `sap-devs` that covers both CLI output strings and content pack files. The system uses embedded JSON catalogs with no external dependencies. Language is resolved from config, then system locale, then falls back to English. German ships as a pilot alongside the infrastructure.

## Decisions

| Question | Decision |
|---|---|
| Scope | Both CLI output and content/packs, in one unified system |
| Language detection | Config `language` field → first non-empty of `LANG` then `LC_ALL` env vars → `en`; stripping applied to all inputs |
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
// Priority: cfgLang (non-empty config.yaml language field) →
//   LANG env var → LC_ALL env var → "en".
// "Non-empty" means a string that is not "". An env var set to ""
// is treated as unset and the next source is checked.
// On Windows, LANG/LC_ALL are rarely set; callers should encourage
// users to set language via config.
// Parsing strips encoding and region suffixes from all inputs
// (cfgLang, LANG, LC_ALL): "de_AT.UTF-8" → "de", "de_AT" → "de".
// Catalog lookup is performed on the stripped base tag only.
// If the stripped tag has no embedded catalog, falls back to "en".
// The full locale tag (e.g. "de_AT") is never used as a catalog key.
func Resolve(cfgLang string) string
```

Env var priority: `LANG` is checked first, then `LC_ALL`. The first non-empty value (after stripping) wins. `LANG=""` is treated as unset — `LC_ALL` is then checked. This applies to all three inputs (cfgLang, LANG, LC_ALL), so `config set language de_AT` resolves to `de` at runtime.

On Windows `LANG` and `LC_ALL` are typically unset. The `config set language` mechanism is the primary path for non-English on Windows. This is documented as a known limitation.

**Package-level active language:**
```go
var ActiveLang string  // set once in rootCmd.PersistentPreRunE; read-only thereafter
```

`ActiveLang` is written once in `PersistentPreRunE` before any command body or goroutine is started by a command. Cobra runs commands sequentially; no concurrent access occurs in the current architecture.

**String lookup — two functions:**
```go
// T looks up key in the catalog for lang.
// Falls back to en.json if key absent in lang catalog.
// Falls back to raw key string if absent in both (never panics).
func T(lang, key string) string

// Tf looks up key and executes it as a text/template with the provided data map.
// data may be nil (equivalent to calling T).
// Falls back using the same rules as T before template execution.
// Template execution uses option("missingkey=error") so missing data keys
// produce an error rather than rendering as "<no value>".
// On template parse or execution failure (e.g. malformed template string,
// missing data key), returns the resolved raw template string (the catalog
// value after key fallback, before execution) rather than an error or empty string.
func Tf(lang, key string, data map[string]any) string
```

Separating `T` and `Tf` eliminates the ambiguous variadic signature. Call sites use `T` for static strings and `Tf` for strings with dynamic values.

**Key naming convention:** `<command>.<descriptor>`, e.g. `inject.short`, `inject.done`, `inject.long`. Subcommands use the full dot-separated path built from the parent chain: `profile.list.short`, `config.show.long`. The root command uses the hardcoded prefix `root` (not derived from `cmd.Use`, which is `sap-devs`): `root.short`, `root.long`. The authoritative key set is `en.json`, which must include both `.short` and `.long` keys for every command. Convention enforcement tooling (e.g. CI lint for key naming) is deferred to a future iteration.

**Embedded catalogs:**
```
internal/i18n/
  catalogs/
    en.json    ← authoritative source of all keys
    de.json    ← German pilot
  i18n.go
```

Catalog format (showing both `.short` and `.long` key forms):
```json
{
  "root.short": "AI-first SAP developer toolkit",
  "root.long": "sap-devs injects up-to-date SAP developer knowledge into your AI tools.",
  "inject.short": "Push SAP context into your AI tools",
  "inject.long": "Renders content from active packs and injects it into detected AI tool configs.",
  "inject.done": "Injected context into {{.Count}} tool(s)"
}
```

Catalogs are embedded via `//go:embed catalogs/*.json` and loaded into a `map[string]map[string]string` at package init. A malformed catalog file causes a `panic` at init time — this is a programmer error that must be caught in development and CI, not a runtime condition.

### Config Integration

`config.Config` gains a `Language` field:
```go
type Config struct {
    CompanyRepo string     `yaml:"company_repo,omitempty"`
    Language    string     `yaml:"language,omitempty"`  // e.g. "de" — empty means auto-detect
    Sync        SyncConfig `yaml:"sync"`
}
```

Empty string means auto-detect from system locale. Existing configs require no migration. `config set language ""` stores an empty string; because `Language` uses `yaml:"language,omitempty"`, it marshals as absent from YAML (same as not set). The existing `config set` mechanism reads, updates, and re-marshals the full `Config` struct, so `omitempty` handles the empty-value case automatically — no special write logic is needed. Unsupported values (e.g. `klingon`) are accepted silently and fall back to `en` via `Resolve` at runtime. This is intentional — runtime fallback is the sole response.

**Setting the language:**
```
sap-devs config set language de
```
Uses the existing `config set` key/value mechanism — no new command needed.

**Initialisation in `rootCmd.PersistentPreRunE`:**
```go
i18n.ActiveLang = i18n.Resolve(cfg.Language)
localizeCommands(rootCmd, i18n.ActiveLang)
```
Runs before any command body, so all commands see the resolved language.

### CLI String Migration

Two categories of strings:

**Cobra metadata** (`Short`, `Long`, `Use`) — set at `init()` time before `PersistentPreRunE` runs. `localizeCommands` is called from `PersistentPreRunE` to patch them before any command body runs.

`localizeCommands` lives in `cmd/root.go` alongside `rootCmd`. Signature:

```go
// localizeCommands walks root and all its descendants (recursively via Commands())
// and updates Short and Long from the i18n catalog.
// Use strings are not translated (they contain argument placeholders, not prose).
// Root identification: a command is the root if !cmd.HasParent(). The root command
// uses the hardcoded key prefix "root". All other commands build a dot-separated
// path by walking their parent chain upward until a command with !cmd.HasParent()
// is reached (that ancestor is excluded from the path). For example:
//   root command (sap-devs)     → "root.short" / "root.long"
//   direct child "inject"       → "inject.short" / "inject.long"
//   subcommand "profile list"   → "profile.list.short" / "profile.list.long"
//   subcommand "config show"    → "config.show.short" / "config.show.long"
// localizeCommands calls i18n.T for each key, which provides the full fallback
// chain automatically (target lang → en.json → cobra-registered string).
func localizeCommands(root *cobra.Command, lang string)
```

**Known limitation — `--help` and `help` subcommand:** Cobra intercepts `--help` / `-h` flags and the `help` subcommand without invoking `PersistentPreRunE`. This means ALL `--help` invocations (`sap-devs --help`, `sap-devs inject --help`, `sap-devs help inject`) display un-localized English `Short`/`Long` strings. This is accepted as a known limitation.

**Runtime output** — call sites use `T` or `Tf`:
```go
// static string
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "inject.success"))

// string with dynamic value
fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "inject.done", map[string]any{"Count": count}))
```

**German pilot scope for CLI strings:**

- All `Short`/`Long` command descriptions (highest visibility)
- Runtime output for `inject`, `sync`, `tip`, and `doctor` commands (all four are in scope)

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

**`LoadPack` new signature:**
```go
func LoadPack(packDir string, lang string) (*Pack, error)
```
`lang` is expected to be a stripped base language tag (e.g. `"de"`, `"en"`) as produced by `i18n.Resolve` — never a full locale tag like `"de_AT"`. An empty `lang` (`""`) is treated as `"en"`: no locale suffix is attempted and base files are used. When loading `context.md` and `tips.md`, tries `<file>.<lang>.md` first (when lang is non-empty and not `"en"`); falls back to the base file silently if absent.

**`ContentLoader.LoadPacks` new signature:**
```go
func (cl *ContentLoader) LoadPacks(profile *Profile, lang string) ([]*Pack, error)
```
Passes `lang` down to each `LoadPack` call. All callers must be updated — implementors must grep the full repo (not just `cmd/`) for `LoadPacks` call sites. In `cmd/`, pass `i18n.ActiveLang`. In tests, pass `"en"` unless the test is specifically exercising locale behaviour. The `go build ./...` compile check ensures no callers are silently missed.

**`pack.yaml` metadata** — locale values are stored under a `locales` sub-map for standard `gopkg.in/yaml.v3` compatibility. A malformed `locales` block causes `yaml.Unmarshal` to return an error, which `LoadPack` propagates as a pack load error (same behaviour as any other malformed `pack.yaml`):

```yaml
id: cap
name: SAP Cloud Application Programming Model
description: Node.js and Java framework for cloud-native BTP apps
locales:
  de:
    name: SAP Cloud Application Programming Modell
    description: Node.js- und Java-Framework für Cloud-native BTP-Anwendungen
```

`packMeta` gains a `Locales` map:

```go
type packMeta struct {
    ID          string                       `yaml:"id"`
    Name        string                       `yaml:"name"`
    Description string                       `yaml:"description"`
    Tags        []string                     `yaml:"tags"`
    Profiles    []string                     `yaml:"profiles"`
    Weight      int                          `yaml:"weight"`
    Locales     map[string]packMetaLocale    `yaml:"locales,omitempty"`
}

type packMetaLocale struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
}
```

`LoadPack` selects the locale entry matching `lang`; if absent, uses the base `Name`/`Description`.

**German pilot scope for content:** `context.de.md` and `tips.de.md` for the `cap` pack only, plus German `locales.de` block in `cap/pack.yaml`.

## Data Flow

```
rootCmd.PersistentPreRunE
  → config.Load()
  → i18n.Resolve(cfg.Language)                  sets i18n.ActiveLang
  → localizeCommands(rootCmd, i18n.ActiveLang)   patches cobra Short/Long

command body
  → i18n.T(i18n.ActiveLang, "key")              for static runtime output
  → i18n.Tf(i18n.ActiveLang, "key", data)       for dynamic runtime output
  → ContentLoader.LoadPacks(profile, i18n.ActiveLang)
      → LoadPack(packDir, i18n.ActiveLang)
          → tries context.<lang>.md, falls back to context.md
          → tries tips.<lang>.md, falls back to tips.md
          → selects locales.<lang>.name / .description from pack.yaml
```

## Error Handling

- Missing translation key → silent fallback to `en.json` → fallback to raw key string (never panics, never errors)
- Unknown/unsupported language (no catalog) → silent fallback to `en` at resolution time
- Unsupported or empty value in `config.yaml language` field → silent fallback to `en` at runtime; no error at write time
- `Tf` template parse/execute failure → returns raw (un-executed) catalog template string
- Missing locale content file → silent fallback to base English file
- Malformed `locales` block in `pack.yaml` → `LoadPack` returns error (propagated from `yaml.Unmarshal`)
- No `language` in config + no `LANG`/`LC_ALL` env var → `en`
- Windows with no locale env vars → `en`; user must use `config set language` for non-English

## Testing

**`internal/i18n` unit tests:**

- `Resolve`: config value wins over env vars; `LANG` wins over `LC_ALL`; `LANG=""` treated as unset (falls through to `LC_ALL`); `de_AT.UTF-8` parses to `de`; unknown language falls back to `en`; empty config + no env vars returns `en`
- `T`: known key returns translation; missing key falls back to `en` catalog value; missing in both returns raw key
- `Tf`: template substitution works; nil data behaves like `T`; missing key falls back before template execution; template execution failure returns raw template string; missing data key returns raw template string

**`LoadPack` tests:**

- Locale file present → locale content loaded for `context.md` and `tips.md`
- Locale file absent → base file loaded silently
- `pack.yaml` `locales.<lang>` present → localised name/description returned
- `pack.yaml` `locales.<lang>` absent → base name/description returned
- Malformed `locales` block → error returned

**`config.Config` tests:**

- `Language` field round-trips through marshal/unmarshal
- Empty `Language` marshals as omitempty (no key in YAML output)

**`cmd/` call site tests:**

- `LoadPacks` callers updated to pass `lang` — compile-time verification via `go build ./...`

## Known Limitations

- ALL `--help` / `-h` / `sap-devs help <cmd>` invocations display English `Short`/`Long` strings. Cobra intercepts help before `PersistentPreRunE` runs. Accepted as a known limitation.
- On Windows, `LANG`/`LC_ALL` are typically unset. Users wanting non-English output on Windows must run `sap-devs config set language <lang>`.
- No validation of the `language` config value at write time; unsupported values fall back to English silently.
- Running `sap-devs tip` in German with a profile that includes non-`cap` packs returns tips in English (only the `cap` pack has German content in this iteration). Mixed-language output is expected and accepted.
- Duplicate keys in a catalog JSON file are silently resolved by Go's `encoding/json` (last value wins). This is a translator error that the infrastructure does not detect.

## Out of Scope (this iteration)

- Plural rules, gender inflection, RTL layout
- Translations for any language other than German
- Content pack translations beyond the `cap` pack
- CLI output for `init`, `mcp`, `update`, `resources`, `version`, `config`, and `profile` (beyond German pilot commands: inject/sync/tip/doctor)
- Contributor tooling (translation workflow, string extraction scripts, key linting)
