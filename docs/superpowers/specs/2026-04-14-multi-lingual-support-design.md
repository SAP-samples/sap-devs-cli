# Multi-lingual Support Design

**Date:** 2026-04-14
**Status:** Approved

## Overview

Add a unified i18n infrastructure to `sap-devs` that covers both CLI output strings and content pack files. The system uses embedded JSON catalogs with no external dependencies. Language is resolved from config, then system locale, then falls back to English. German ships as a pilot alongside the infrastructure.

## Decisions

| Question | Decision |
|---|---|
| Scope | Both CLI output and content/packs, in one unified system |
| Language detection | Config `language` field → `LANG` then `LC_ALL` env vars → `en` |
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
// On Windows, LANG/LC_ALL are rarely set; callers should encourage
// users to set language via config. If neither env var is set and
// cfgLang is empty, returns "en".
// Parsing strips encoding and region suffixes from all inputs
// (cfgLang, LANG, LC_ALL): "de_AT.UTF-8" → "de", "de_AT" → "de".
// Catalog lookup is performed on the stripped base tag only.
// If the stripped tag has no embedded catalog, falls back to "en".
// The full locale tag (e.g. "de_AT") is never used as a catalog key.
func Resolve(cfgLang string) string
```

Env var priority: `LANG` is checked first, then `LC_ALL`. The first non-empty value wins. The same stripping rule applies to all three inputs (cfgLang, LANG, LC_ALL), so `config set language de_AT` resolves to `de` at runtime.

On Windows `LANG` and `LC_ALL` are typically unset. The `config set language` mechanism is the primary path for non-English on Windows. This is documented as a known limitation.

**Package-level active language:**
```go
var ActiveLang string  // set once in rootCmd.PersistentPreRunE
```

**String lookup — two functions:**
```go
// T looks up key in the catalog for lang.
// Falls back to en.json if key absent in lang catalog.
// Falls back to raw key string if absent in both (never panics).
func T(lang, key string) string

// Tf looks up key and executes it as a text/template with the provided data map.
// data may be nil (equivalent to calling T).
// Falls back using the same rules as T before template execution.
// If template parsing or execution fails (e.g. malformed template in catalog,
// missing data key), returns the raw un-executed template string rather than
// an error or empty string.
func Tf(lang, key string, data map[string]any) string
```

Separating `T` and `Tf` eliminates the ambiguous variadic signature. Call sites use `T` for static strings and `Tf` for strings with dynamic values.

**Key naming convention:** `<command>.<descriptor>`, e.g. `inject.short`, `inject.done`, `root.long`. Subcommands use the full dot-separated path built from the parent chain: `profile.list.short`, `config.show.long`. The authoritative key set is `en.json`. Convention enforcement tooling (e.g. CI lint for key naming) is deferred to a future iteration.

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

Empty string means auto-detect from system locale. Existing configs require no migration.

**Setting the language:**
```
sap-devs config set language de
```
Uses the existing `config set` key/value mechanism — no new command needed. No validation is performed at write time; an unsupported language value (e.g. `klingon`) is accepted silently and falls back to English at runtime via `Resolve`. This is intentional — runtime fallback is the sole response to an unsupported value.

**Initialisation in `rootCmd.PersistentPreRunE`:**
```go
i18n.ActiveLang = i18n.Resolve(cfg.Language)
localizeCommands(rootCmd, i18n.ActiveLang)
```
Runs before any command body, so all commands see the resolved language.

### CLI String Migration

Two categories of strings:

**Cobra metadata** (`Short`, `Long`, `Use`) — set at `init()` time before `PersistentPreRunE` runs. `localizeCommands` is called from `PersistentPreRunE` to patch them before any command body or help renderer runs.

`localizeCommands` lives in `cmd/` (package `cmd`). Signature:

```go
// localizeCommands walks root and all its descendants (recursively via Commands())
// and updates Short and Long from the i18n catalog.
// Use strings are not translated (they contain argument placeholders, not prose).
// Key derivation: the walker builds a dot-separated path by walking each command's
// parent chain up to (but not including) the root. For example:
//   root command "inject"       → "inject.short" / "inject.long"
//   subcommand "profile list"   → "profile.list.short" / "profile.list.long"
//   subcommand "config show"    → "config.show.short" / "config.show.long"
// en.json is the authoritative source for both .short and .long keys.
// If a key is absent in the target language catalog, falls back to en.json.
// If a key is absent in en.json, falls back to the cobra-registered English
// string unchanged (so a missing en.json key never blanks a description).
func localizeCommands(root *cobra.Command, lang string)
```

**Call site audit:** All callers of `LoadPacks` must be updated when the signature changes. Implementors must grep the full repo (not just `cmd/`) for `LoadPacks` call sites before making the change. The `go build ./...` compile check ensures no callers are missed.

**Known limitation — `--help` before `PersistentPreRunE`:** If cobra serves help without invoking `PersistentPreRunE` (e.g. `sap-devs --help` in some edge cases), `Short`/`Long` display in English. This is accepted as a known limitation; the cobra help path in normal usage does invoke `PersistentPreRunE`.

**Runtime output** — call sites use `T` or `Tf`:
```go
// static string
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "inject.success"))

// string with dynamic value
fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "inject.done", map[string]any{"Count": count}))
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

**`LoadPack` new signature:**
```go
func LoadPack(packDir string, lang string) (*Pack, error)
```
When loading `context.md` and `tips.md`, tries `<file>.<lang>.md` first; falls back to the base file silently if absent. No change to the `Pack` struct or any consumer.

**`ContentLoader.LoadPacks` new signature:**
```go
func (cl *ContentLoader) LoadPacks(profile *Profile, lang string) ([]*Pack, error)
```
Passes `lang` down to each `LoadPack` call. All call sites in `cmd/` that currently call `LoadPacks(profile)` must be updated to `LoadPacks(profile, i18n.ActiveLang)`.

**`pack.yaml` metadata** — locale values are stored under a `locales` sub-map to remain compatible with standard `gopkg.in/yaml.v3` unmarshalling:

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
  → i18n.Resolve(cfg.Language)             sets i18n.ActiveLang
  → localizeCommands(rootCmd, i18n.ActiveLang)  patches cobra Short/Long

command body
  → i18n.T(i18n.ActiveLang, "key")         for static runtime output
  → i18n.Tf(i18n.ActiveLang, "key", data)  for dynamic runtime output
  → ContentLoader.LoadPacks(profile, i18n.ActiveLang)
      → LoadPack(packDir, i18n.ActiveLang)
          → tries context.<lang>.md, falls back to context.md
          → tries tips.<lang>.md, falls back to tips.md
          → selects locales.<lang>.name / .description from pack.yaml
```

## Error Handling

- Missing translation key → silent fallback to `en.json` → fallback to raw key string (never panics, never errors)
- Unknown/unsupported language (no catalog) → silent fallback to `en` at resolution time
- Unsupported value in `config.yaml language` field → silent fallback to `en` at runtime; no error at write time
- Missing locale content file → silent fallback to base English file
- No `language` in config + no `LANG`/`LC_ALL` env var → `en`
- Windows with no locale env vars → `en`; user must use `config set language` for non-English

## Testing

**`internal/i18n` unit tests:**

- `Resolve`: config value wins over env vars; `LANG` wins over `LC_ALL`; `de_AT.UTF-8` parses to `de`; unknown language falls back to `en`; empty config + no env vars returns `en`
- `T`: known key returns translation; missing key falls back to `en` catalog value; missing in both returns raw key
- `Tf`: template substitution works; nil data behaves like `T`; missing key falls back before template execution

**`LoadPack` tests:**

- Locale file present → locale content loaded for `context.md` and `tips.md`
- Locale file absent → base file loaded silently
- `pack.yaml` `locales.<lang>` present → localised name/description returned
- `pack.yaml` `locales.<lang>` absent → base name/description returned

**`config.Config` tests:**

- `Language` field round-trips through marshal/unmarshal
- Empty `Language` marshals as omitempty (no key in YAML output)

**`cmd/` call site tests:**

- `LoadPacks` callers updated to pass `lang` — compile-time verification via `go build ./...`

## Known Limitations

- `--help` displayed before `PersistentPreRunE` runs shows English `Short`/`Long` strings. Accepted as minor edge case.
- On Windows, `LANG`/`LC_ALL` are typically unset. Users wanting non-English output on Windows must run `sap-devs config set language <lang>`.
- No validation of the `language` config value at write time; unsupported values fall back to English silently.

## Out of Scope (this iteration)

- Plural rules, gender inflection, RTL layout
- Translations for any language other than German
- Content pack translations beyond the `cap` pack
- CLI output for commands other than inject/sync/tip/doctor
- Contributor tooling (translation workflow, string extraction scripts)
