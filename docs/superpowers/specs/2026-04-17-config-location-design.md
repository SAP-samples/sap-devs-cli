# Design: `sap-devs config location`

**Date:** 2026-04-17
**Status:** Approved

## Overview

Add a `location` configuration field so users can store their geographic context (city/country). This enables future features (event filtering, learning recommendations) to apply geographic relevance. The value is a free-text string — no coordinates, no OS location APIs.

## Data Model

Add `Location string` to `Config` in `internal/config/config.go`:

```go
type Config struct {
    CompanyRepo string     `yaml:"company_repo,omitempty"`
    Language    string     `yaml:"language,omitempty"`
    Location    string     `yaml:"location,omitempty"`
    Sync        SyncConfig `yaml:"sync"`
}
```

`omitempty` keeps YAML clean when unset. No default value. No change to `Default()`.

## Command: `sap-devs config location`

New dedicated subcommand in `cmd/config.go`, registered alongside `configTokenCmd`. Three usage modes:

### Manual set
```
sap-devs config location "Hamburg, Germany"
```
Takes an optional positional argument. Saves to `~/.config/sap-devs/config.yaml` and prints confirmation.

### Auto-detect
```
sap-devs config location --detect
```
1. Prints privacy notice: `"Note: auto-detect uses IP geolocation (approximate). No GPS or OS location permissions are used."`
2. Makes a GET to `http://ip-api.com/json` with a 3-second timeout.
3. Parses `city` and `country` fields from the JSON response.
4. Prompts: `"Detected: <city>, <country> — save? [Y/n]: "`
5. On confirmation (default yes), saves the value and prints confirmation.
6. On HTTP error or non-2xx response, prints a warning and exits cleanly (no save).

### Show current value
```
sap-devs config location
```
No args, no flags: prints current value or `(not set)`.

### Note: `config set location` is intentionally unsupported

`configSetCmd`'s switch is **not** extended with `"location"`. The dedicated `config location` subcommand is the only setter. If a user runs `sap-devs config set location "..."`, the existing "unknown config key" error is the correct response — it will point them toward `config location`.

### Error cases
- `--detect` with a positional arg: returns an error (mirrors `config token` pattern).
- HTTP timeout or failure: prints a warning message and returns `nil` (soft failure, no save). Mirrors the project norm of reserving non-zero exits for hard failures.

## `config show` Update

Add a `location` line to `configShowCmd` output after the `language` line and before the `sync.*` lines (i.e., between `language` and `sync.tips`), aligned with existing fields:

```
location:        Hamburg, Germany
```

Empty string displays as blank (consistent with `company_repo` behaviour).

## `init` Wizard — Step 4 (location)

Insert a new location step between profile selection (current Step 3) and the inject step (current Step 4). The existing `init.step4_*` and `init.step5_*` i18n keys (inject and shell-hook) are **renamed** to `init.step5_*` and `init.step6_*` respectively as part of this change. All references in `cmd/init.go` are updated to match.

New i18n keys use the prefix `init.step4_location_` to avoid any collision risk:

Prompt text (matching existing step header style):
```
Step 4/6: Set your location (optional)

Your location is used for event filtering and recommendations.
Enter city and country (e.g. "Hamburg, Germany"), type "detect" to
auto-detect from your IP address, or press Enter to skip:
> 
```

Logic:
- Empty input → skip, no save.
- Input `"detect"` (case-insensitive) → run the same detect flow as `--detect` (privacy notice → HTTP fetch → confirm → save).
- Any other input → save as-is.

The detect flow calls the shared `detectLocation` helper. For the init wizard, `detectLocation` is called with `cmd.OutOrStdout()` as `w` and `os.Stdin` as `r` (bypassing `readLine()` for this sub-flow only, consistent with how `config token` reads from `os.Stdin` directly).

## i18n

New keys added to both `internal/i18n/catalogs/en.json` and `de.json`:

| Key | English value |
|-----|---------------|
| `config.location.short` | `Store your location for event and recommendation filtering` |
| `config.location.detect_with_value` | `cannot use --detect with a location value` |
| `config.location.detect_notice` | `Note: auto-detect uses IP geolocation (approximate). No GPS or OS location permissions are used.` |
| `config.location.detect_confirm` | `Detected: {{.Value}} — save? [Y/n]: ` |
| `config.location.detect_failed` | `Location detection failed: {{.Err}}` |
| `config.location.detect_cancelled` | `Location not saved.` |
| `config.location.not_set` | `(not set)` |
| `config.location.done` | `Location set to: {{.Value}}` |
| `config.show.location` | `location:        {{.Value}}` |
| `init.step4_location_header` | `\nStep 4/6: Set your location (optional)` |
| `init.step4_location_body` | `Your location is used for event filtering and recommendations.\nEnter city and country (e.g. "Hamburg, Germany"), type "detect" to auto-detect from your IP address, or press Enter to skip:` |
| `init.step4_location_prompt` | Bare string: prompt character followed by a space (`"> "` without the outer quotes) |

Existing `init.step4_*` keys (inject step) are renamed to `init.step5_*`; existing `init.step5_*` keys (shell-hook step) are renamed to `init.step6_*`. The `/5` denominator embedded in all `init.stepN_header` string values must also be updated to `/6` across all six steps (steps 1–3 already have correct numbers, only the denominator changes).

German translations provided for all keys.

## Architecture: Shared `detectLocation` helper

To avoid duplicating HTTP logic between `configLocationCmd` and `initCmd`, extract:

```go
// detectLocation fetches approximate location from ip-api.com.
// Writes the privacy notice and confirm prompt to w; reads confirmation from r.
// Returns the location string if confirmed, or ("", nil) if declined/cancelled.
func detectLocation(w io.Writer, r io.Reader) (string, error)
```

Lives in `cmd/config_location.go` (new file) alongside `configLocationCmd`. Both `configLocationCmd` and `initCmd` call it.

## Tests

### `internal/config`
- `TestLocation_RoundTrip`: set `cfg.Location`, save, reload, assert equal (mirrors `TestConfigLanguageRoundTrip`).
- `TestLocation_Omitempty`: empty `Location` must not appear in serialised YAML.

### `cmd`
- `TestConfigLocation_SetValue`: `config location "Berlin, Germany"` saves and `config show` displays it.
- `TestConfigLocation_ShowNotSet`: `config location` with no value stored prints `(not set)`.
- `TestConfigLocation_DetectFlag`: `config location --detect` flag is wired (unit test stubs HTTP; or just verifies the flag exists and the detect+value conflict errors correctly).
- Skip Windows per `skipOnWindows` convention.

## Files Changed

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `Location` field |
| `internal/config/config_test.go` | Add round-trip and omitempty tests |
| `cmd/config.go` | Add `configLocationCmd` registration; update `configShowCmd` |
| `cmd/config_location.go` | New file: `configLocationCmd`, `detectLocation` helper |
| `cmd/config_location_test.go` | New file: command-level tests |
| `cmd/init.go` | Insert Step 4 (location), renumber Steps 4→5, 5→6 |
| `internal/i18n/catalogs/en.json` | New keys |
| `internal/i18n/catalogs/de.json` | New keys (German) |
