# Multi-lingual Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a unified i18n infrastructure covering both CLI output strings and content pack files, with German (`de`) as the pilot language.

**Architecture:** A new `internal/i18n` package embeds JSON catalogs and provides `Resolve`/`T`/`Lookup`/`Tf` functions. `config.Config` gains a `Language` field. `LoadPack`/`LoadPacks` gain a `lang` parameter for locale-suffixed file selection. `rootCmd.PersistentPreRunE` resolves the active language once and calls `localizeCommands` to patch cobra metadata.

**Tech Stack:** Go stdlib only — `encoding/json`, `text/template`, `embed`, `os`, `strings`; cobra for command walk; `github.com/stretchr/testify` (already used in tests).

> **Windows note:** `go test` always fails locally due to Windows Defender. Use `go build ./... && go vet ./...` for local verification. Tests are verified in CI (ubuntu-latest). Test steps include both commands — run whichever applies to your environment.

---

## File Map

**Create:**
- `internal/i18n/i18n.go` — Resolve, T, Lookup, Tf, ActiveLang, catalog loader
- `internal/i18n/i18n_test.go` — unit tests for all i18n functions
- `internal/i18n/catalogs/en.json` — authoritative English catalog (all keys)
- `internal/i18n/catalogs/de.json` — German pilot translations
- `content/packs/cap/context.de.md` — German CAP context
- `content/packs/cap/tips.de.md` — German CAP tips

**Modify:**
- `internal/config/config.go` — add `Language string` field
- `internal/config/config_test.go` — round-trip and omitempty tests
- `cmd/config.go` — add `language` case to `config set`; add `language` line to `config show`
- `internal/content/pack.go` — add `packMetaLocale` struct; add `lang string` param to `LoadPack`
- `internal/content/pack_test.go` — add locale file and metadata locale tests
- `internal/content/loader.go` — add `lang string` param to `LoadPacks`; pass to `LoadPack`
- `internal/content/loader_test.go` — update any `LoadPack`/`LoadPacks` calls
- `cmd/root.go` — add i18n init to `PersistentPreRunE`; add `localizeCommands` + `buildLocalizeKey`
- `cmd/inject.go` — update `LoadPacks` call; replace output strings with `i18n.T`/`Tf`
- `cmd/sync.go` — update `LoadPacks` call; replace output strings
- `cmd/tip.go` — update `LoadPacks` call; replace no-tips message
- `cmd/doctor.go` — update `LoadPacks` calls (×3); replace output strings
- `cmd/mcp.go` — update `LoadPacks` calls (×5); no output string changes
- `cmd/resources.go` — update `LoadPacks` calls (×3); no output string changes
- `content/packs/cap/pack.yaml` — add `locales.de` block

---

### Task 1: `internal/i18n` package

**Files:**
- Create: `internal/i18n/i18n.go`
- Create: `internal/i18n/i18n_test.go`
- Create: `internal/i18n/catalogs/en.json` (minimal — expanded in Task 6)
- Create: `internal/i18n/catalogs/de.json` (minimal — expanded in Task 7)

- [ ] **Step 1.1: Create minimal catalogs (required before package compiles)**

Create `internal/i18n/catalogs/en.json`:
```json
{
  "root.short": "AI-first SAP developer toolkit",
  "root.long": "sap-devs injects up-to-date SAP developer knowledge into your AI tools.",
  "inject.short": "Push SAP context to your AI tools",
  "inject.done": "SAP developer context injected ({{.Scope}} scope)."
}
```

Create `internal/i18n/catalogs/de.json`:
```json
{
  "root.short": "KI-gestütztes SAP Entwickler-Toolkit",
  "inject.short": "SAP-Kontext in deine KI-Tools einbinden"
}
```

- [ ] **Step 1.2: Write the failing tests**

Create `internal/i18n/i18n_test.go`:
```go
package i18n

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name    string
		cfgLang string
		lang    string
		lcAll   string
		want    string
	}{
		{"config wins over env", "de", "fr", "", "de"},
		{"config with unknown lang falls back to en", "klingon", "", "", "en"},
		{"LANG used when config empty", "", "de_AT.UTF-8", "", "de"},
		{"LC_ALL used when LANG empty", "", "", "de", "de"},
		{"LANG wins over LC_ALL", "", "de", "fr", "de"},
		{"LANG empty string treated as unset", "", "", "de", "de"},
		{"unknown LANG falls back to en", "", "fr_FR.UTF-8", "", "en"},
		{"no input returns en", "", "", "", "en"},
		{"config de_AT strips to de", "de_AT.UTF-8", "", "", "de"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LANG", tt.lang)
			t.Setenv("LC_ALL", tt.lcAll)
			assert.Equal(t, tt.want, Resolve(tt.cfgLang))
		})
	}
}

func TestT(t *testing.T) {
	// Key exists in de catalog
	assert.Equal(t, "KI-gestütztes SAP Entwickler-Toolkit", T("de", "root.short"))
	// Key absent in de, falls back to en
	assert.Equal(t, "SAP developer context injected ({{.Scope}} scope).", T("de", "inject.done"))
	// Key absent from both catalogs — returns raw key
	assert.Equal(t, "missing.key", T("de", "missing.key"))
}

func TestLookup(t *testing.T) {
	// Found in de catalog
	v, ok := Lookup("de", "root.short")
	assert.True(t, ok)
	assert.NotEmpty(t, v)

	// Not in de, found in en fallback
	v, ok = Lookup("de", "inject.done")
	assert.True(t, ok)
	assert.Contains(t, v, "{{.Scope}}")

	// Not in either
	_, ok = Lookup("de", "missing.key")
	assert.False(t, ok)
}

func TestTf(t *testing.T) {
	// Template substitution using en fallback
	got := Tf("de", "inject.done", map[string]any{"Scope": "global"})
	assert.Equal(t, "SAP developer context injected (global scope).", got)

	// Missing data key with missingkey=error → raw template string
	got = Tf("de", "inject.done", map[string]any{})
	assert.Equal(t, "SAP developer context injected ({{.Scope}} scope).", got)

	// nil data with template action → raw template string
	got = Tf("de", "inject.done", nil)
	assert.Equal(t, "SAP developer context injected ({{.Scope}} scope).", got)
}

func TestActiveLang(t *testing.T) {
	// ActiveLang is a package-level var that starts empty
	orig := ActiveLang
	t.Cleanup(func() { ActiveLang = orig })
	ActiveLang = "de"
	assert.Equal(t, "de", ActiveLang)
}

// Ensure catalogs panic on bad JSON at init — tested via panic recovery in a subprocess.
// Skip: malformed catalog panics at init time (programmer error caught by CI build).

func init() {
	// Override LANG/LC_ALL in case test environment has them set
	os.Unsetenv("LANG")
	os.Unsetenv("LC_ALL")
}
```

- [ ] **Step 1.3: Run to verify tests fail**

```bash
go test ./internal/i18n/...
# Expected: FAIL — package does not exist yet
```
Local alternative: `go build ./internal/i18n/... 2>&1` — expected: package not found.

- [ ] **Step 1.4: Implement `internal/i18n/i18n.go`**

```go
package i18n

import (
	"bytes"
	"embed"
	"encoding/json"
	"os"
	"strings"
	"text/template"
)

//go:embed catalogs/*.json
var catalogFS embed.FS

// ActiveLang is the resolved active language tag.
// Set once in rootCmd.PersistentPreRunE before any command body runs.
var ActiveLang string

// catalogs holds all loaded catalogs: map[lang]map[key]value.
var catalogs map[string]map[string]string

func init() {
	catalogs = make(map[string]map[string]string)
	entries, err := catalogFS.ReadDir("catalogs")
	if err != nil {
		panic("i18n: failed to read catalogs directory: " + err.Error())
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		lang := strings.TrimSuffix(e.Name(), ".json")
		data, err := catalogFS.ReadFile("catalogs/" + e.Name())
		if err != nil {
			panic("i18n: failed to read catalog " + e.Name() + ": " + err.Error())
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			panic("i18n: malformed catalog " + e.Name() + ": " + err.Error())
		}
		catalogs[lang] = m
	}
}

// Resolve returns the active language tag.
// Priority: cfgLang (if non-empty, used exclusively) → LANG env var →
// LC_ALL env var → "en". Empty env var values are treated as unset.
// All inputs are stripped of region/encoding suffixes: "de_AT.UTF-8" → "de".
// Unknown tags (no catalog) resolve to "en".
func Resolve(cfgLang string) string {
	if cfgLang != "" {
		return resolveTag(cfgLang)
	}
	for _, env := range []string{"LANG", "LC_ALL"} {
		if v := os.Getenv(env); v != "" {
			return resolveTag(v)
		}
	}
	return "en"
}

func resolveTag(s string) string {
	tag := stripTag(s)
	if _, ok := catalogs[tag]; ok {
		return tag
	}
	return "en"
}

// stripTag removes region and encoding suffixes: "de_AT.UTF-8" → "de".
func stripTag(s string) string {
	s = strings.ToLower(s)
	if i := strings.IndexAny(s, "_."); i > 0 {
		s = s[:i]
	}
	return s
}

// Lookup returns the translation for key in lang (falling back to en).
// Returns the string and true if found in either catalog; "", false if absent from both.
func Lookup(lang, key string) (string, bool) {
	if lang != "" && lang != "en" {
		if cat, ok := catalogs[lang]; ok {
			if v, ok := cat[key]; ok {
				return v, true
			}
		}
	}
	if cat, ok := catalogs["en"]; ok {
		if v, ok := cat[key]; ok {
			return v, true
		}
	}
	return "", false
}

// T returns the translation for key, falling back to the raw key string if not found.
func T(lang, key string) string {
	if v, ok := Lookup(lang, key); ok {
		return v
	}
	return key
}

// Tf resolves key via T and executes it as a text/template with data.
// Uses missingkey=error: missing keys in data trigger a failure.
// On any parse or execute failure, returns the raw (un-executed) catalog string.
// If data is nil, template actions will fail (missingkey=error), returning the raw string.
func Tf(lang, key string, data map[string]any) string {
	raw := T(lang, key)
	tmpl, err := template.New("").Option("missingkey=error").Parse(raw)
	if err != nil {
		return raw
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return raw
	}
	return buf.String()
}
```

- [ ] **Step 1.5: Run tests and verify they pass**

```bash
go test ./internal/i18n/...
# Expected: PASS
```
Local: `go build ./internal/i18n/... && go vet ./internal/i18n/...`

- [ ] **Step 1.6: Commit**

```bash
git add internal/i18n/
git commit -m "feat: add internal/i18n package with Resolve, T, Lookup, Tf"
```

---

### Task 2: Add `Language` field to `config.Config`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/config.go`

- [ ] **Step 2.1: Write the failing tests**

Add to `internal/config/config_test.go`:
```go
func TestConfigLanguageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	cfg.Language = "de"
	require.NoError(t, cfg.Save(dir))

	loaded, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "de", loaded.Language)
}

func TestConfigLanguageOmitempty(t *testing.T) {
	dir := t.TempDir()
	cfg := Default() // Language is ""
	require.NoError(t, cfg.Save(dir))

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "language",
		"empty Language should not appear in YAML output")
}
```

Ensure the test file imports `"os"` and `"path/filepath"` if not already present.

- [ ] **Step 2.2: Run to verify tests fail**

```bash
go test ./internal/config/...
# Expected: FAIL — Language field does not exist yet
```
Local: `go build ./internal/config/... && go vet ./internal/config/...`

- [ ] **Step 2.3: Add `Language` field to `config.Config`**

In `internal/config/config.go`, add the `Language` field after `CompanyRepo`:
```go
type Config struct {
	CompanyRepo string     `yaml:"company_repo,omitempty"`
	Language    string     `yaml:"language,omitempty"` // e.g. "de"; empty = auto-detect from locale
	Sync        SyncConfig `yaml:"sync"`
}
```

No change to `Default()` — `Language` defaults to `""` (auto-detect).

- [ ] **Step 2.4: Add `language` case to `config set` and line to `config show`**

In `cmd/config.go`, in the `configSetCmd.RunE` switch statement, add:
```go
case "language":
    cfg.Language = args[1]
```

In `configShowCmd.RunE`, after the `company_repo` line, add:
```go
fmt.Printf("language:        %s\n", cfg.Language)
```

- [ ] **Step 2.5: Run tests and verify they pass**

```bash
go test ./internal/config/...
# Expected: PASS
```
Local: `go build ./... && go vet ./...`

- [ ] **Step 2.6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go cmd/config.go
git commit -m "feat: add Language field to config.Config with config set/show support"
```

---

### Task 3: Add `lang` param to `LoadPack`

**Files:**
- Modify: `internal/content/pack.go`
- Modify: `internal/content/pack_test.go`

- [ ] **Step 3.1: Write the failing tests**

Add to `internal/content/pack_test.go`:
```go
func TestLoadPack_LocaleContextFile(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: test\nname: Test Pack\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.md"), []byte("English context"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.de.md"), []byte("German context"), 0644))

	// German: locale file used
	p, err := content.LoadPack(dir, "de")
	require.NoError(t, err)
	assert.Equal(t, "German context", p.ContextMD)

	// French: no locale file, falls back to base
	p, err = content.LoadPack(dir, "fr")
	require.NoError(t, err)
	assert.Equal(t, "English context", p.ContextMD)

	// Empty lang: base file used
	p, err = content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "English context", p.ContextMD)

	// lang="en": base file used (no context.en.md attempted)
	p, err = content.LoadPack(dir, "en")
	require.NoError(t, err)
	assert.Equal(t, "English context", p.ContextMD)
}

func TestLoadPack_LocaleMetadata(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: test
name: Test Pack
description: A test pack
tags: []
profiles: []
weight: 0
locales:
  de:
    name: Testpaket
    description: Ein Testpaket
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	// German locale
	p, err := content.LoadPack(dir, "de")
	require.NoError(t, err)
	assert.Equal(t, "Testpaket", p.Name)
	assert.Equal(t, "Ein Testpaket", p.Description)

	// English (no locale block)
	p, err = content.LoadPack(dir, "en")
	require.NoError(t, err)
	assert.Equal(t, "Test Pack", p.Name)
	assert.Equal(t, "A test pack", p.Description)
}

func TestLoadPack_MalformedLocales(t *testing.T) {
	dir := t.TempDir()
	// locales value is a string instead of a map — invalid YAML for the field
	yaml := "id: test\nname: Test\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\nlocales: not-a-map\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	_, err := content.LoadPack(dir, "de")
	assert.Error(t, err, "malformed locales block should return an error")
}
```

Update the existing `TestLoadPack_ParsesAllFiles` and `TestLoadPack_MissingOptionalFilesOK` calls to pass an empty `lang` argument — they will fail to compile before the signature change.

- [ ] **Step 3.2: Verify tests fail**

```bash
go build ./internal/content/...
# Expected: FAIL — LoadPack signature mismatch
```

- [ ] **Step 3.3: Update `pack.go`**

In `internal/content/pack.go`:

1. Add `packMetaLocale` struct and `Locales` field to `packMeta`:
```go
type packMetaLocale struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type packMeta struct {
	ID          string                       `yaml:"id"`
	Name        string                       `yaml:"name"`
	Description string                       `yaml:"description"`
	Tags        []string                     `yaml:"tags"`
	Profiles    []string                     `yaml:"profiles"`
	Weight      int                          `yaml:"weight"`
	Locales     map[string]packMetaLocale    `yaml:"locales,omitempty"`
}
```

2. Change `LoadPack(packDir string)` to `LoadPack(packDir string, lang string)`.

3. After populating `pack.Name` and `pack.Description` from `meta`, add locale override:
```go
if lang != "" && lang != "en" {
    if loc, ok := meta.Locales[lang]; ok {
        if loc.Name != "" {
            pack.Name = loc.Name
        }
        if loc.Description != "" {
            pack.Description = loc.Description
        }
    }
}
```

4. Replace the `context.md` read block:
```go
// was: if data, err := os.ReadFile(filepath.Join(packDir, "context.md")); err == nil {
contextFile := filepath.Join(packDir, "context.md")
if lang != "" && lang != "en" {
    if loc := filepath.Join(packDir, "context."+lang+".md"); fileExists(loc) {
        contextFile = loc
    }
}
if data, err := os.ReadFile(contextFile); err == nil {
    pack.ContextMD = string(data)
}
```

5. Replace the `tips.md` read block similarly:
```go
tipsFile := filepath.Join(packDir, "tips.md")
if lang != "" && lang != "en" {
    if loc := filepath.Join(packDir, "tips."+lang+".md"); fileExists(loc) {
        tipsFile = loc
    }
}
if data, err := os.ReadFile(tipsFile); err == nil {
    pack.Tips = parseTips(string(data))
}
```

6. Add the `fileExists` helper at the bottom of `pack.go`:
```go
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

- [ ] **Step 3.4: Fix existing tests and the internal `LoadPack` call in `loader.go`**

In `internal/content/pack_test.go`, update the two existing test calls:
```go
// TestLoadPack_ParsesAllFiles
pack, err := content.LoadPack(dir, "")

// TestLoadPack_MissingOptionalFilesOK
pack, err := content.LoadPack(dir, "")
```

Also update the `LoadPack` call inside `internal/content/loader.go` (line ~35) to compile now — pass `""` as a placeholder lang; the correct `lang` parameter is threaded in Task 4:

```go
pack, err := LoadPack(filepath.Join(packsDir, e.Name()), "")
```

- [ ] **Step 3.5: Verify the full module compiles and tests pass**

```bash
go build ./...
go vet ./...
go test ./internal/content/...
# Expected: all PASS
```

Local (Windows): `go build ./... && go vet ./...`

- [ ] **Step 3.6: Commit**

```bash
git add internal/content/pack.go internal/content/pack_test.go internal/content/loader.go
git commit -m "feat: add lang param to LoadPack with locale file and metadata support"
```

---

### Task 4: Add `lang` param to `LoadPacks` and update all 13 call sites

**Files:**
- Modify: `internal/content/loader.go`
- Modify: `internal/content/loader_test.go`
- Modify: `cmd/inject.go`, `cmd/tip.go`, `cmd/doctor.go` (×3), `cmd/mcp.go` (×5), `cmd/resources.go` (×3)

> Before making changes, verify the full list of call sites:
> ```bash
> grep -rn "\.LoadPacks(" . --include="*.go"
> ```
> Expected results match the 13 calls listed in the file map above.

- [ ] **Step 4.1: Update `loader.go`**

Change `LoadPacks(profile *Profile)` to `LoadPacks(profile *Profile, lang string)` in `internal/content/loader.go`:

```go
func (cl *ContentLoader) LoadPacks(profile *Profile, lang string) ([]*Pack, error) {
    // ... existing packMap logic unchanged ...
    // Change the LoadPack call:
    pack, err := LoadPack(filepath.Join(packsDir, e.Name()), lang)
    // ... rest unchanged ...
}
```

- [ ] **Step 4.2: Update `loader_test.go`**

Find all `LoadPacks(` calls in `internal/content/loader_test.go` and add `""` as the `lang` argument (`LoadPack` calls are only in `pack_test.go`, already addressed in Task 3):
```bash
grep -n "LoadPacks(" internal/content/loader_test.go
```
Update each call: `LoadPacks(profile)` → `LoadPacks(profile, "")`.

- [ ] **Step 4.3: Update all `cmd/` call sites**

For each file, change every `loader.LoadPacks(...)` call to pass `i18n.ActiveLang` as the second argument:

**`cmd/inject.go`** (1 call):
```go
packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
```
Add import: `"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"`

**`cmd/tip.go`** (1 call):
```go
packs, err := loader.LoadPacks(activeProfile, i18n.ActiveLang)
```
Add import: `"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"`

**`cmd/doctor.go`** (3 calls — all three `loader.LoadPacks(...)` calls):
```go
packs, err = loader.LoadPacks(nil, i18n.ActiveLang)        // line ~33
packs, err = loader.LoadPacks(active, i18n.ActiveLang)     // line ~56
packs, err = loader.LoadPacks(p, i18n.ActiveLang)          // line ~68
```
Add import: `"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"`

**`cmd/mcp.go`** (5 calls) — add `i18n.ActiveLang` as second arg to all 5 `loader.LoadPacks(...)` calls. Add import.

**`cmd/resources.go`** (3 calls) — add `i18n.ActiveLang` as second arg to all 3 calls. Add import.

- [ ] **Step 4.4: Verify everything compiles**

```bash
go build ./...
# Expected: SUCCESS — no compile errors
go vet ./...
# Expected: SUCCESS
```

- [ ] **Step 4.5: Run tests**

```bash
go test ./internal/content/...
# Expected: PASS
```

- [ ] **Step 4.6: Commit**

```bash
git add internal/content/loader.go internal/content/loader_test.go \
        cmd/inject.go cmd/tip.go cmd/doctor.go cmd/mcp.go cmd/resources.go
git commit -m "feat: add lang param to LoadPacks and update all call sites"
```

---

### Task 5: Language init and `localizeCommands` in `cmd/root.go`

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 5.1: Add i18n init and `localizeCommands` to `cmd/root.go`**

1. Add the import `"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"` to the import block.

2. In `PersistentPreRunE`, add language resolution at the top of the function. The i18n init runs for **all** commands (including `update`) so all commands see the resolved language. Replace the entire function opening up to the update check guard:

```go
PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    updateHintCh = nil // reset before every invocation

    // Resolve active language before any command body runs.
    // Runs for all commands so that Short/Long are always localized.
    if paths, err := xdg.New(); err == nil {
        if cfg, err := config.Load(paths.ConfigDir); err == nil {
            i18n.ActiveLang = i18n.Resolve(cfg.Language)
        }
    }
    if i18n.ActiveLang == "" {
        i18n.ActiveLang = "en"
    }
    localizeCommands(rootCmd, i18n.ActiveLang)

    // Skip background update check for "update" command and dev builds.
    if cmd.Name() == "update" || Version == "dev" {
        return nil
    }
    // ... rest of the existing update check code unchanged ...
```

3. Add `localizeCommands` and `buildLocalizeKey` at the bottom of `cmd/root.go`:
```go
// localizeCommands walks root and all its descendants and updates Short and Long
// from the i18n catalog. Uses i18n.Lookup so cobra-registered strings are never
// overwritten with bare key names when a key is absent from both catalogs.
// Key path segments are derived from cmd.Name() (cobra's first word of Use).
// The root command uses the hardcoded prefix "root".
func localizeCommands(root *cobra.Command, lang string) {
    var walk func(cmd *cobra.Command)
    walk = func(cmd *cobra.Command) {
        prefix := buildLocalizeKey(cmd)
        if s, ok := i18n.Lookup(lang, prefix+".short"); ok {
            cmd.Short = s
        }
        if s, ok := i18n.Lookup(lang, prefix+".long"); ok {
            cmd.Long = s
        }
        for _, sub := range cmd.Commands() {
            walk(sub)
        }
    }
    walk(root)
}

// buildLocalizeKey returns the dot-separated i18n key prefix for cmd.
// The root command (!cmd.HasParent()) returns "root".
// Other commands build the path by walking up the parent chain
// (excluding the root command itself).
func buildLocalizeKey(cmd *cobra.Command) string {
    if !cmd.HasParent() {
        return "root"
    }
    var parts []string
    for c := cmd; c.HasParent(); c = c.Parent() {
        parts = append([]string{c.Name()}, parts...)
    }
    return strings.Join(parts, ".")
}
```

Add `"strings"` to the import block if not already present.

- [ ] **Step 5.2: Verify it compiles**

```bash
go build ./...
go vet ./...
# Expected: SUCCESS
```

- [ ] **Step 5.3: Smoke test — default language (English) unchanged**

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run
# Expected: English output, no errors
```

- [ ] **Step 5.4: Commit**

```bash
git add cmd/root.go
git commit -m "feat: resolve active language and localize cobra commands in PersistentPreRunE"
```

---

### Task 6: Populate `en.json` and wire pilot command output strings

**Files:**
- Modify: `internal/i18n/catalogs/en.json` (expand to full key set)
- Modify: `cmd/inject.go`, `cmd/sync.go`, `cmd/tip.go`, `cmd/doctor.go`

- [ ] **Step 6.1: Replace `en.json` with the complete key set**

Overwrite `internal/i18n/catalogs/en.json`:
```json
{
  "root.short": "AI-first SAP developer toolkit",
  "root.long": "sap-devs injects up-to-date SAP developer knowledge into your AI tools.",

  "inject.short": "Push SAP context to your AI tools",
  "inject.long": "Inject up-to-date SAP developer context into all detected AI tools.\n\nInjects at global (user) scope by default into tools such as Claude Code,\nCursor, and GitHub Copilot. Use --project to opt into project scope and inject\ninto project-level files (CLAUDE.md, .cursorrules, etc.) in the current directory.",
  "inject.dry_run": "[dry-run] no files will be modified",
  "inject.done": "SAP developer context injected ({{.Scope}} scope).",
  "inject.hint": "Run 'sap-devs inject --dry-run' to preview changes before writing.",

  "sync.short": "Pull latest SAP developer content",
  "sync.long": "Syncs content from the official repo (and company repo if configured). Respects per-category TTLs unless --force is set.",
  "sync.disabled": "Sync is disabled in config.",
  "sync.up_to_date": "All content is up to date.",
  "sync.syncing": "Syncing SAP developer content...",
  "sync.updated": "Updated: {{.Categories}}",
  "sync.warn_https": "Warning: company_repo must be an HTTPS URL (got: {{.URL}}) — skipping sync.",
  "sync.syncing_company": "Syncing company repo...",
  "sync.warn_company_failed": "Warning: company repo sync failed: {{.Err}}",

  "tip.short": "Print a SAP developer tip (add to your shell profile)",
  "tip.no_tips": "No tips available. Run 'sap-devs sync' to download content.",

  "doctor.short": "Check local tool versions against pack requirements",
  "doctor.no_tools": "No tools defined for the current selection.",
  "doctor.col_tool": "TOOL",
  "doctor.col_required": "REQUIRED",
  "doctor.col_found": "FOUND",
  "doctor.col_status": "STATUS",
  "doctor.install_header": "\nInstall commands:",
  "doctor.status_ok": "ok",
  "doctor.status_ok_unverified": "ok (unverified)",
  "doctor.status_fail": "FAIL",
  "doctor.status_missing": "MISSING",

  "profile.short": "Manage your developer profile",
  "profile.list.short": "List available developer profiles",
  "profile.set.short": "Set your active developer profile",
  "profile.show.short": "Show your current profile and pack weights",

  "config.short": "Manage sap-devs configuration",
  "config.show.short": "Display current configuration",
  "config.set.short": "Set a configuration value",
  "config.company.short": "Configure the company content repo URL",

  "mcp.short": "Manage SAP MCP servers",
  "mcp.list.short": "List available SAP MCP servers",
  "mcp.status.short": "Show which SAP MCP servers are registered in your AI tool configs",
  "mcp.install.short": "Install and wire an SAP MCP server into your AI tools",

  "resources.short": "Browse curated SAP resources",
  "resources.list.short": "List curated resources for your active profile",
  "resources.search.short": "Search across all SAP resources",
  "resources.open.short": "Open a resource URL in the default browser",

  "version.short": "Print the sap-devs version",
  "update.short": "Update sap-devs to the latest release",
  "update.long": "Check for a newer release on GitHub and install it if found.",
  "init.short": "First-time setup wizard"
}
```

- [ ] **Step 6.2: Wire `cmd/inject.go` output strings**

Replace the three `fmt` output calls in `inject.go`:
```go
// Replace:   fmt.Println("[dry-run] no files will be modified")
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "inject.dry_run"))

// Replace:   fmt.Printf("SAP developer context injected (%s scope).\n", scope)
fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "inject.done", map[string]any{"Scope": scope}))

// Replace:   fmt.Println("Run 'sap-devs inject --dry-run' to preview changes before writing.")
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "inject.hint"))
```

The `i18n` import was added in Task 4.

- [ ] **Step 6.3: Wire `cmd/sync.go` output strings**

Add import `"github.tools.sap/developer-relations/sap-devs-cli/internal/i18n"`.

Replace all `fmt.Println`/`fmt.Printf` output calls:
```go
// fmt.Println("Sync is disabled in config.")
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "sync.disabled"))

// fmt.Println("All content is up to date.")
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "sync.up_to_date"))

// fmt.Println("Syncing SAP developer content...")
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "sync.syncing"))

// fmt.Printf("Updated: %v\n", categories)
fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "sync.updated", map[string]any{"Categories": categories}))

// fmt.Printf("Warning: company_repo must be an HTTPS URL (got: %s) — skipping sync.\n", cfg.CompanyRepo)
fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "sync.warn_https", map[string]any{"URL": cfg.CompanyRepo}))

// fmt.Println("Syncing company repo...")
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "sync.syncing_company"))

// fmt.Printf("Warning: company repo sync failed: %v\n", err)
fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "sync.warn_company_failed", map[string]any{"Err": err}))
```

Note: `sync.go` uses bare `fmt.Println` (not `cmd.OutOrStdout()`). The `RunE` function receives `cmd *cobra.Command` as first arg — use `cmd.OutOrStdout()` consistently. For the warning inside the goroutine-free callback, `cmd` is in scope.

- [ ] **Step 6.4: Wire `cmd/tip.go` output string**

The `i18n` import was added in Task 4. Replace:
```go
// fmt.Println("No tips available. Run 'sap-devs sync' to download content.")
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "tip.no_tips"))
```

- [ ] **Step 6.5: Wire `cmd/doctor.go` output strings**

The `i18n` import was added in Task 4.

Replace in `doctorCmd.RunE`:
```go
// fmt.Println("No tools defined for the current selection.")
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "doctor.no_tools"))
```

Replace in `printDoctorTable` (note: this function doesn't receive `cmd`; use `fmt.Printf` directly since table formatting is structural):
```go
func printDoctorTable(results []content.ToolResult, lang string) {
    fmt.Printf("%-20s %-12s %-12s %s\n",
        i18n.T(lang, "doctor.col_tool"),
        i18n.T(lang, "doctor.col_required"),
        i18n.T(lang, "doctor.col_found"),
        i18n.T(lang, "doctor.col_status"))
    fmt.Println(strings.Repeat("-", 62))
    for _, r := range results {
        found := r.Found
        if found == "" {
            found = "-"
        }
        fmt.Printf("%-20s %-12s %-12s %s\n", r.Tool.ID, r.Tool.Required, found, statusLabel(r.Status, lang))
    }
}
```

Update the call site: `printDoctorTable(results)` → `printDoctorTable(results, i18n.ActiveLang)`.

Update `statusLabel` to accept lang:
```go
func statusLabel(s content.CheckStatus, lang string) string {
    switch s {
    case content.StatusOK:
        return i18n.T(lang, "doctor.status_ok")
    case content.StatusUnknown:
        return i18n.T(lang, "doctor.status_ok_unverified")
    case content.StatusFail:
        return i18n.T(lang, "doctor.status_fail")
    case content.StatusMissing:
        return i18n.T(lang, "doctor.status_missing")
    }
    return string(s)
}
```

Replace in `printInstallCommands`:
```go
// fmt.Println("\nInstall commands:")
fmt.Println(i18n.T(i18n.ActiveLang, "doctor.install_header"))
```

Update `printInstallCommands` and `statusLabel` call sites to pass `i18n.ActiveLang` where needed.

- [ ] **Step 6.6: Verify everything compiles and runs in English**

```bash
go build ./...
go vet ./...
SAP_DEVS_DEV=1 go run . inject --dry-run
# Expected: English output unchanged
SAP_DEVS_DEV=1 go run . doctor
# Expected: English table headers, English status labels
```

- [ ] **Step 6.7: Commit**

```bash
git add internal/i18n/catalogs/en.json cmd/inject.go cmd/sync.go cmd/tip.go cmd/doctor.go
git commit -m "feat: wire pilot command output through i18n, populate en.json"
```

---

### Task 7: German translations in `de.json`

**Files:**
- Modify: `internal/i18n/catalogs/de.json`

- [ ] **Step 7.1: Replace `de.json` with German translations**

Overwrite `internal/i18n/catalogs/de.json`:
```json
{
  "root.short": "KI-gestütztes SAP Entwickler-Toolkit",
  "root.long": "sap-devs integriert aktuelles SAP-Entwicklerwissen in deine KI-Tools.",

  "inject.short": "SAP-Kontext in deine KI-Tools einbinden",
  "inject.long": "Aktuellen SAP-Entwicklerkontext in alle erkannten KI-Tools einbinden.\n\nStandardmäßig globaler (Benutzer-)Bereich für Tools wie Claude Code, Cursor und GitHub Copilot. Mit --project auf Projektbereich umschalten und in Projektdateien (CLAUDE.md, .cursorrules, etc.) im aktuellen Verzeichnis einbinden.",
  "inject.dry_run": "[Testlauf] Es werden keine Dateien geändert",
  "inject.done": "SAP-Entwicklerkontext eingefügt ({{.Scope}}-Bereich).",
  "inject.hint": "Führe 'sap-devs inject --dry-run' aus, um Änderungen vorab anzuzeigen.",

  "sync.short": "Aktuellen SAP-Entwicklerinhalt herunterladen",
  "sync.long": "Synchronisiert Inhalte aus dem offiziellen Repo (und dem Unternehmens-Repo, falls konfiguriert). Berücksichtigt kategoriespezifische TTLs, außer --force ist gesetzt.",
  "sync.disabled": "Synchronisierung ist in der Konfiguration deaktiviert.",
  "sync.up_to_date": "Alle Inhalte sind aktuell.",
  "sync.syncing": "SAP-Entwicklerinhalt wird synchronisiert...",
  "sync.updated": "Aktualisiert: {{.Categories}}",
  "sync.warn_https": "Warnung: company_repo muss eine HTTPS-URL sein (erhalten: {{.URL}}) — Synchronisierung übersprungen.",
  "sync.syncing_company": "Unternehmens-Repo wird synchronisiert...",
  "sync.warn_company_failed": "Warnung: Synchronisierung des Unternehmens-Repos fehlgeschlagen: {{.Err}}",

  "tip.short": "Einen SAP-Entwicklertipp anzeigen (in dein Shell-Profil einfügen)",
  "tip.no_tips": "Keine Tipps verfügbar. Führe 'sap-devs sync' aus, um Inhalte herunterzuladen.",

  "doctor.short": "Lokale Tool-Versionen gegen Pack-Anforderungen prüfen",
  "doctor.no_tools": "Keine Tools für die aktuelle Auswahl definiert.",
  "doctor.col_tool": "TOOL",
  "doctor.col_required": "ERFORDERLICH",
  "doctor.col_found": "GEFUNDEN",
  "doctor.col_status": "STATUS",
  "doctor.install_header": "\nInstallationsbefehle:",
  "doctor.status_ok": "ok",
  "doctor.status_ok_unverified": "ok (nicht verifiziert)",
  "doctor.status_fail": "FEHLER",
  "doctor.status_missing": "FEHLT",

  "profile.short": "Entwicklerprofil verwalten",
  "profile.list.short": "Verfügbare Entwicklerprofile auflisten",
  "profile.set.short": "Aktives Entwicklerprofil festlegen",
  "profile.show.short": "Aktuelles Profil und Pack-Gewichtungen anzeigen",

  "config.short": "sap-devs-Konfiguration verwalten",
  "config.show.short": "Aktuelle Konfiguration anzeigen",
  "config.set.short": "Konfigurationswert setzen",
  "config.company.short": "URL des Unternehmens-Inhalts-Repos konfigurieren",

  "mcp.short": "SAP MCP-Server verwalten",
  "mcp.list.short": "Verfügbare SAP MCP-Server auflisten",
  "mcp.status.short": "Registrierte SAP MCP-Server in KI-Tool-Konfigurationen anzeigen",
  "mcp.install.short": "Einen SAP MCP-Server installieren und in KI-Tools einbinden",

  "resources.short": "Kuratierte SAP-Ressourcen durchsuchen",
  "resources.list.short": "Kuratierte Ressourcen für aktives Profil auflisten",
  "resources.search.short": "Über alle SAP-Ressourcen suchen",
  "resources.open.short": "Ressourcen-URL im Standardbrowser öffnen",

  "version.short": "sap-devs-Version ausgeben",
  "update.short": "sap-devs auf die neueste Version aktualisieren",
  "update.long": "Sucht auf GitHub nach einer neueren Version und installiert diese, falls gefunden.",
  "init.short": "Ersteinrichtungsassistent"
}
```

- [ ] **Step 7.2: Smoke test with German locale**

> **Note:** `--help` and `help` bypass `PersistentPreRunE`, so cobra `Short`/`Long` strings are always displayed in English for help invocations. This is a known limitation documented in the spec. Smoke test using commands that complete normally instead.

```bash
SAP_DEVS_DEV=1 LANG=de go run . inject --dry-run
# Expected: "[Testlauf] Es werden keine Dateien geändert"
SAP_DEVS_DEV=1 LANG=de go run . sync
# Expected: German sync output (e.g. "Alle Inhalte sind aktuell." if content already synced)
SAP_DEVS_DEV=1 LANG=de go run . doctor
# Expected: German table headers (TOOL, ERFORDERLICH, GEFUNDEN, STATUS)
```

- [ ] **Step 7.3: Verify English still works**

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run
# Expected: "[dry-run] no files will be modified"
SAP_DEVS_DEV=1 go run . doctor
# Expected: English table headers (TOOL, REQUIRED, FOUND, STATUS)
```

- [ ] **Step 7.4: Commit**

```bash
git add internal/i18n/catalogs/de.json
git commit -m "feat: add German (de) pilot translations"
```

---

### Task 8: German CAP pack content

**Files:**
- Create: `content/packs/cap/context.de.md`
- Create: `content/packs/cap/tips.de.md`
- Modify: `content/packs/cap/pack.yaml`

- [ ] **Step 8.1: Add `locales.de` block to `cap/pack.yaml`**

Read the current `content/packs/cap/pack.yaml` first, then add `locales` at the end:
```yaml
locales:
  de:
    name: SAP Cloud Application Programming Model
    description: Node.js- und Java-Framework für Cloud-native Business-Anwendungen auf BTP
```

(The name stays in English as it is an SAP product name; only the description is translated.)

- [ ] **Step 8.2: Create `content/packs/cap/context.de.md`**

~~~markdown
## SAP CAP (Cloud Application Programming Model)

CAP ist SAPs primäres Framework für Cloud-native Business-Anwendungen auf SAP BTP.
Es verwendet CDS (Core Data Services) für Daten- und Servicedefinitionen sowie Node.js oder Java für die Service-Logik.

### Wichtige Tools
- `@sap/cds-dk` — CAP Development Kit (CLI: `cds`)
- `cds watch` — lokaler Entwicklungsserver mit Live-Reload
- `cds deploy` — Deployment in Datenbank / Cloud

### CDS-Datenmodellierung
```cds
entity Books : managed {
  key ID     : Integer;
  title      : localized String(111);
  author     : Association to Authors;
}
```

### Service-Definition
```cds
service CatalogService @(path:'/browse') {
  @readonly entity Books as SELECT from my.Books;
}
```

### Best Practices
- Entities in `db/schema.cds` definieren, Services in `srv/*.cds`
- `cds.ql` für typsichere CQL-Abfragen verwenden
- Eingebaute Authentifizierung via `@requires`-Annotationen nutzen
~~~

- [ ] **Step 8.3: Create `content/packs/cap/tips.de.md`**

```markdown
## cds watch für lokale Entwicklung nutzen
Tags: cap,nodejs
`cds watch` statt `node server.js` ausführen — es lädt bei jeder Dateiänderung neu und protokolliert alle Anfragen.

## managed-Entities für Audit-Felder definieren
Tags: cap,cds
`: managed` zu Entities hinzufügen, um `createdAt`, `createdBy`, `modifiedAt`, `modifiedBy` automatisch zu erhalten.

## @readonly in der Service-Schicht verwenden
Tags: cap,odata,security
`@readonly` in der Service-Schicht statt auf DB-Ebene einschränken — hält das Schema flexibel.

## CAP-Versionskompatibilität prüfen
Tags: cap,versions
`cds version` ausführen, um alle CAP-Stack-Versionen anzuzeigen. Nicht übereinstimmende `@sap/cds`- und `@sap/cds-dk`-Versionen verursachen schwer zu findende Fehler.
```

- [ ] **Step 8.4: Verify German CAP content loads**

```bash
SAP_DEVS_DEV=1 LANG=de go run . tip
# Expected: German tip text
SAP_DEVS_DEV=1 go run . tip
# Expected: English tip text (unchanged)
```

- [ ] **Step 8.5: Verify English CAP content is unaffected**

```bash
SAP_DEVS_DEV=1 go run . inject --dry-run
# Expected: no errors, English context
```

- [ ] **Step 8.6: Commit**

```bash
git add content/packs/cap/context.de.md content/packs/cap/tips.de.md content/packs/cap/pack.yaml
git commit -m "feat: add German (de) CAP pack content and pack.yaml locale metadata"
```

---

## Final Verification

- [ ] **Full build and vet**

```bash
go build ./...
go vet ./...
```

- [ ] **End-to-end German smoke test**

```bash
SAP_DEVS_DEV=1 LANG=de go run . --help
# Expected: English output (--help bypasses PersistentPreRunE — known limitation; Short/Long are not localized for help invocations)
SAP_DEVS_DEV=1 LANG=de go run . inject --dry-run
# Expected: "[Testlauf] Es werden keine Dateien geändert"
SAP_DEVS_DEV=1 LANG=de go run . sync --force
# Expected: German sync output (e.g. "SAP-Entwicklerinhalt wird synchronisiert...")
SAP_DEVS_DEV=1 LANG=de go run . tip
# Expected: German tip output (if cap pack German tips are present)
SAP_DEVS_DEV=1 LANG=de go run . doctor
# Expected: German table headers (TOOL, ERFORDERLICH, GEFUNDEN, STATUS)
```

- [ ] **End-to-end English smoke test (regression check)**

```bash
SAP_DEVS_DEV=1 go run . --help
# Expected: English descriptions unchanged
SAP_DEVS_DEV=1 go run . inject --dry-run
# Expected: "[dry-run] no files will be modified"
SAP_DEVS_DEV=1 go run . tip
SAP_DEVS_DEV=1 go run . doctor
# Expected: English table headers (TOOL, REQUIRED, FOUND, STATUS)
```

- [ ] **Config language override test**

```bash
SAP_DEVS_DEV=1 go run . config set language de
SAP_DEVS_DEV=1 go run . inject --dry-run
# Expected: German output regardless of LANG env var
SAP_DEVS_DEV=1 go run . config set language ""
```
