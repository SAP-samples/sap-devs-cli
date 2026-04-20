# Config Location Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs config location` subcommand and `init` wizard step so users can store their city/country for future geo-aware features.

**Architecture:** Add `Location` field to `Config` struct; new `cmd/config_location.go` holds the subcommand and shared `detectLocation` helper (inline HTTP to ip-api.com); `cmd/init.go` gains a Step 4 calling the same helper; `configShowCmd` gains a location line. All strings go through the i18n catalog.

**Tech Stack:** Go, cobra, standard `net/http`, `encoding/json`, `gopkg.in/yaml.v3`, testify

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/config/config.go` | Modify | Add `Location` field to `Config` struct |
| `internal/config/config_test.go` | Modify | Round-trip + omitempty tests for `Location` |
| `internal/i18n/catalogs/en.json` | Modify | New `config.location.*` keys; rename/update `init.step4/5_*` keys; update `/5`→`/6` in step headers |
| `internal/i18n/catalogs/de.json` | Modify | Same set of changes in German |
| `cmd/config_location.go` | Create | `configLocationCmd` cobra command + `detectLocation(w, r)` helper |
| `cmd/config_location_test.go` | Create | Command-level tests: set, show-not-set, detect conflict |
| `cmd/config.go` | Modify | Register `configLocationCmd`; add location line to `configShowCmd` |
| `cmd/init.go` | Modify | Insert Step 4 (location); rename step4→5, step5→6 i18n key references |

---

## Task 1: Add `Location` field to Config — with tests first

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write two failing tests in `internal/config/config_test.go`**

Add after `TestConfigLanguageOmitempty`:

```go
func TestLocation_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Location = "Hamburg, Germany"
	require.NoError(t, cfg.Save(dir))

	loaded, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "Hamburg, Germany", loaded.Location)
}

func TestLocation_Omitempty(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default() // Location is ""
	require.NoError(t, cfg.Save(dir))

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "location",
		"empty Location should not appear in YAML output")
}
```

- [ ] **Step 2: Verify tests fail**

```bash
cd .worktrees/feat/config-location
go build ./internal/config/... 2>&1
```

Expected: compile error — `cfg.Location undefined`

- [ ] **Step 3: Add `Location` field to `Config` in `internal/config/config.go`**

Insert after the `Language` field:

```go
Location    string     `yaml:"location,omitempty"`
```

The struct should now read:

```go
type Config struct {
	CompanyRepo string     `yaml:"company_repo,omitempty"`
	Language    string     `yaml:"language,omitempty"`
	Location    string     `yaml:"location,omitempty"`
	Sync        SyncConfig `yaml:"sync"`
}
```

- [ ] **Step 4: Build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add Location field with omitempty"
```

---

## Task 2: i18n — add new keys and update init step numbering

**Files:**
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json`

This is a data-only change. No tests needed (catalog loading is already covered by existing i18n tests).

- [ ] **Step 1: Update `en.json`**

**a) Update the five existing step header denominators from `/5` to `/6` (the sixth step is the new location step being added as a new key below):**

| Old key value | New value |
|---|---|
| `"Step 1/5: GitHub authentication (optional)"` | `"Step 1/6: GitHub authentication (optional)"` |
| `"\nStep 2/5: Downloading SAP developer content..."` | `"\nStep 2/6: Downloading SAP developer content..."` |
| `"\nStep 3/5: What kind of SAP developer are you?"` | `"\nStep 3/6: What kind of SAP developer are you?"` |
| `"\nStep 4/5: Inject SAP context into your AI tools?"` | `"\nStep 5/6: Inject SAP context into your AI tools?"` |
| `"\nStep 5/5: Add SAP tip to your terminal startup?"` | `"\nStep 6/6: Add SAP tip to your terminal startup?"` |

**b) Rename existing `init.step4_*` keys to `init.step5_*` and `init.step5_*` to `init.step6_*`:**

Rename these keys (update both key name and value where the value contains the step number):

```json
"init.step5_header": "\nStep 5/6: Inject SAP context into your AI tools?",
"init.step5_body": "  This writes SAP developer context to your AI tool configuration files.",
"init.step5_prompt": "  Inject now? [Y/n]: ",
"init.step5_warn_failed": "  Warning: inject failed ({{.Err}}). You can run 'sap-devs inject' manually.",
"init.step5_done": "  SAP context injected into your AI tools.",
"init.step6_header": "\nStep 6/6: Add SAP tip to your terminal startup?",
"init.step6_body": "  This adds 'sap-devs tip' to your shell profile so you see a tip each time you open a terminal.",
"init.step6_prompt": "  Add it? [y/N]: ",
"init.step6_no_profile": "  Could not add hook: no shell profile found.\n  Add 'sap-devs tip' to your shell profile manually.",
"init.step6_added": "  ✓ Added to {{.Path}}",
"init.step6_warn_partial": "  Warning: some profiles could not be updated ({{.Err}}).",
"init.step6_restart": "  Restart your terminal to see your first tip.",
"init.step6_already_present": "  Hook already present in your shell profile(s)."
```

**c) Add new `config.location.*` and `init.step4_location_*` keys.** Insert after the `config.token.*` block:

```json
"config.location.short": "Store your location for event and recommendation filtering",
"config.location.detect_with_value": "cannot use --detect with a location value",
"config.location.detect_notice": "Note: auto-detect uses IP geolocation (approximate). No GPS or OS location permissions are used.",
"config.location.detect_confirm": "Detected: {{.Value}} — save? [Y/n]: ",
"config.location.detect_failed": "Location detection failed: {{.Err}}",
"config.location.detect_cancelled": "Location not saved.",
"config.location.not_set": "(not set)",
"config.location.done": "Location set to: {{.Value}}",
"config.show.location": "location:        {{.Value}}",
"init.step4_location_header": "\nStep 4/6: Set your location (optional)",
"init.step4_location_body": "  Your location is used for event filtering and recommendations.\n  Enter city and country (e.g. \"Hamburg, Germany\"), type \"detect\" to\n  auto-detect from your IP address, or press Enter to skip:",
"init.step4_location_prompt": "> "
```

- [ ] **Step 2: Update `de.json` with equivalent German translations**

Apply the same structural changes (rename step4→5, step5→6 keys; update denominators; add new keys). German translations for new keys:

```json
"config.location.short": "Standort für Veranstaltungs- und Empfehlungsfilterung speichern",
"config.location.detect_with_value": "--detect kann nicht zusammen mit einem Standortwert verwendet werden",
"config.location.detect_notice": "Hinweis: Die automatische Erkennung verwendet IP-Geolokalisierung (ungefähr). Es werden keine GPS- oder Betriebssystem-Standortberechtigungen verwendet.",
"config.location.detect_confirm": "Erkannt: {{.Value}} — speichern? [Y/n]: ",
"config.location.detect_failed": "Standorterkennung fehlgeschlagen: {{.Err}}",
"config.location.detect_cancelled": "Standort nicht gespeichert.",
"config.location.not_set": "(nicht gesetzt)",
"config.location.done": "Standort gesetzt auf: {{.Value}}",
"config.show.location": "location:        {{.Value}}",
"init.step4_location_header": "\nSchritt 4/6: Standort festlegen (optional)",
"init.step4_location_body": "  Dein Standort wird für die Veranstaltungsfilterung und Empfehlungen verwendet.\n  Stadt und Land eingeben (z. B. \"Hamburg, Deutschland\"), \"detect\" eingeben\n  für automatische Erkennung per IP-Adresse, oder Enter zum Überspringen:",
"init.step4_location_prompt": "> "
```

Also update the German step headers to use `/6` denominators and rename step4→5, step5→6 exactly as in `en.json`.

- [ ] **Step 3: Build to verify JSON is valid**

```bash
go build ./...
```

Expected: no errors (the i18n `init()` panics on malformed JSON, so a clean build proves the catalogs parse)

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json
git commit -m "feat(i18n): add location keys; renumber init steps 4-5 to 5-6"
```

---

## Task 3: Implement `configLocationCmd` and `detectLocation` helper

**Files:**
- Create: `cmd/config_location.go`
- Create: `cmd/config_location_test.go`

- [ ] **Step 1: Write failing tests in `cmd/config_location_test.go`**

```go
package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/cmd"
)

func TestConfigLocation_SetValue(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	out, err := executeCommand(cmd.RootCmd(), "config", "location", "Berlin, Germany")
	require.NoError(t, err)
	assert.Contains(t, out, "Berlin, Germany")
}

func TestConfigLocation_ShowDisplaysValue(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := executeCommand(cmd.RootCmd(), "config", "location", "Berlin, Germany")
	require.NoError(t, err)

	out, err := executeCommand(cmd.RootCmd(), "config", "show")
	require.NoError(t, err)
	assert.Contains(t, out, "Berlin, Germany")
}

func TestConfigLocation_ShowNotSet(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	out, err := executeCommand(cmd.RootCmd(), "config", "location")
	require.NoError(t, err)
	assert.Contains(t, out, "(not set)")
}

func TestConfigLocation_DetectWithValueErrors(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := executeCommand(cmd.RootCmd(), "config", "location", "--detect", "Hamburg, Germany")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use --detect")
}

func TestConfigLocation_DetectFlagAloneAccepted(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// --detect alone should not error even when the HTTP call fails (soft failure)
	// In tests, ip-api.com is unreachable; command returns nil with a warning message.
	_, err := executeCommand(cmd.RootCmd(), "config", "location", "--detect")
	require.NoError(t, err)
}
```

- [ ] **Step 2: Verify tests fail to compile**

```bash
go build ./cmd/... 2>&1
```

Expected: compile error — `configLocationCmd` not yet registered

- [ ] **Step 3: Create `cmd/config_location.go`**

```go
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var locationDetectFlag bool

var configLocationCmd = &cobra.Command{
	Use:   "location [value]",
	Short: i18n.T("en", "config.location.short"),
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if locationDetectFlag && len(args) > 0 {
			return fmt.Errorf("%s", i18n.T(i18n.ActiveLang, "config.location.detect_with_value"))
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}

		if locationDetectFlag {
			loc, err := detectLocation(cmd.OutOrStdout(), strings.NewReader(""))
			if err != nil {
				return err
			}
			if loc == "" {
				return nil
			}
			cfg.Location = loc
			if err := cfg.Save(paths.ConfigDir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.location.done", map[string]any{"Value": loc}))
			return nil
		}

		if len(args) == 1 {
			cfg.Location = args[0]
			if err := cfg.Save(paths.ConfigDir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.location.done", map[string]any{"Value": args[0]}))
			return nil
		}

		// No args, no flag: show current value
		val := cfg.Location
		if val == "" {
			val = i18n.T(i18n.ActiveLang, "config.location.not_set")
		}
		fmt.Fprintln(cmd.OutOrStdout(), val)
		return nil
	},
}

// detectLocation fetches approximate location from ip-api.com.
// Prints the privacy notice and confirm prompt to w; reads the confirmation line from r.
// Returns the location string if confirmed, or ("", nil) if declined or on HTTP failure.
func detectLocation(w io.Writer, r io.Reader) (string, error) {
	fmt.Fprintln(w, i18n.T(i18n.ActiveLang, "config.location.detect_notice"))

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://ip-api.com/json")
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := "HTTP error"
		if err != nil {
			errMsg = err.Error()
		}
		fmt.Fprintln(w, i18n.Tf(i18n.ActiveLang, "config.location.detect_failed", map[string]any{"Err": errMsg}))
		return "", nil
	}
	defer resp.Body.Close()

	var result struct {
		City    string `json:"city"`
		Country string `json:"country"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.City == "" {
		fmt.Fprintln(w, i18n.Tf(i18n.ActiveLang, "config.location.detect_failed", map[string]any{"Err": "could not parse response"}))
		return "", nil
	}

	detected := result.City + ", " + result.Country
	fmt.Fprint(w, i18n.Tf(i18n.ActiveLang, "config.location.detect_confirm", map[string]any{"Value": detected}))

	scanner := bufio.NewScanner(r)
	scanner.Scan()
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if answer == "n" || answer == "no" {
		fmt.Fprintln(w, i18n.T(i18n.ActiveLang, "config.location.detect_cancelled"))
		return "", nil
	}
	return detected, nil
}

func init() {
	configLocationCmd.Flags().BoolVar(&locationDetectFlag, "detect", false, "Auto-detect location from IP address")
}
```

- [ ] **Step 4: Build to verify it compiles**

```bash
go build ./cmd/... 2>&1
```

Expected: build succeeds — `configLocationCmd` is a package-level var and compiles fine; it's just not yet reachable as a subcommand until registered

- [ ] **Step 5: Register `configLocationCmd` in `cmd/config.go`**

In the `init()` function at the bottom of `cmd/config.go`, add `configLocationCmd` to `configCmd.AddCommand`:

```go
configCmd.AddCommand(configShowCmd, configSetCmd, configCompanyCmd, configTokenCmd, configLocationCmd)
```

Also add the `location` line to `configShowCmd`'s `RunE`, **after** the `language` line and **before** the `sync.tips` line:

```go
fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf(i18n.ActiveLang, "config.show.location", map[string]any{"Value": cfg.Location}))
```

- [ ] **Step 6: Build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add cmd/config_location.go cmd/config_location_test.go cmd/config.go
git commit -m "feat(cmd): add config location subcommand with --detect flag"
```

---

## Task 4: Wire location step into `init` wizard

**Files:**
- Modify: `cmd/init.go`

- [ ] **Step 1: Update `cmd/init.go` — rename step4→5 and step5→6 key references**

In `initCmd.RunE`, update every `i18n.T`/`i18n.Tf` call that references `init.step4_*` or `init.step5_*`:

| Old key | New key |
|---------|---------|
| `init.step4_header` | `init.step5_header` |
| `init.step4_body` | `init.step5_body` |
| `init.step4_prompt` | `init.step5_prompt` |
| `init.step4_warn_failed` | `init.step5_warn_failed` |
| `init.step4_done` | `init.step5_done` |
| `init.step5_header` | `init.step6_header` |
| `init.step5_body` | `init.step6_body` |
| `init.step5_prompt` | `init.step6_prompt` |
| `init.step5_no_profile` | `init.step6_no_profile` |
| `init.step5_added` | `init.step6_added` |
| `init.step5_warn_partial` | `init.step6_warn_partial` |
| `init.step5_restart` | `init.step6_restart` |
| `init.step5_already_present` | `init.step6_already_present` |

- [ ] **Step 2: Insert Step 4 (location) block in `cmd/init.go`**

Insert the following block **after** the Step 3 profile block and **before** the Step 4 (now Step 5) inject block:

```go
// Step 4: Set location (optional)
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step4_location_header"))
fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step4_location_body"))
fmt.Fprint(cmd.OutOrStdout(), i18n.T(i18n.ActiveLang, "init.step4_location_prompt"))
locInput := strings.TrimSpace(readLine())
if locInput != "" {
    locationCfg, locErr := config.Load(paths.ConfigDir)
    if locErr == nil {
        if strings.ToLower(locInput) == "detect" {
            if detected, detectErr := detectLocation(cmd.OutOrStdout(), os.Stdin); detectErr == nil && detected != "" {
                locationCfg.Location = detected
                locationCfg.Save(paths.ConfigDir) //nolint:errcheck
            }
        } else {
            locationCfg.Location = locInput // preserve original casing
            locationCfg.Save(paths.ConfigDir) //nolint:errcheck
        }
    }
}
```

Also add `"os"` to the import if not already present (it already is — `os.Stdin` is already used for token input).

- [ ] **Step 3: Build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add cmd/init.go
git commit -m "feat(init): add Step 4 location collection to setup wizard"
```

---

## Task 5: Final verification

- [ ] **Step 1: Full build and vet**

```bash
cd .worktrees/feat/config-location
go build ./... && go vet ./...
```

Expected: no errors

- [ ] **Step 2: Manual smoke test — set location manually**

```bash
SAP_DEVS_DEV=1 go run . config location "Hamburg, Germany"
```

Expected output contains: `Location set to: Hamburg, Germany`

- [ ] **Step 3: Verify `config show` displays it**

```bash
SAP_DEVS_DEV=1 go run . config show
```

Expected: output contains `location:        Hamburg, Germany` between the `language` and `sync.tips` lines

- [ ] **Step 4: Verify `config location` (no args) shows value**

```bash
SAP_DEVS_DEV=1 go run . config location
```

Expected: `Hamburg, Germany`

- [ ] **Step 5: Verify `--detect` conflict error**

```bash
SAP_DEVS_DEV=1 go run . config location --detect "London, UK"
```

Expected: error containing `cannot use --detect`

- [ ] **Step 6: Final commit if any fixups needed, then confirm branch is ready**

```bash
git log --oneline feat/config-location ^main
```

Expected: 4 commits (config field, i18n, command, init wizard)
