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

### Error cases
- `--detect` with a positional arg: returns an error (mirrors `config token` pattern).
- HTTP timeout or failure: warning message, exits with non-zero code.

## `config show` Update

Add a `location` line to `configShowCmd` output, aligned with existing fields:

```
location:        Hamburg, Germany
```

Empty string displays as blank (consistent with `company_repo` behaviour).

## `init` Wizard — Step 4

Insert a new Step 4 between profile selection (current Step 3) and inject (current Step 4). Renumber subsequent steps.

Prompt text:
```
=== Step 4: Set your location (optional) ===

Your location is used for event filtering and recommendations.
Enter city and country (e.g. "Hamburg, Germany"), type "detect" to
auto-detect from your IP address, or press Enter to skip:
> 
```

Logic:
- Empty input → skip, no save.
- Input `"detect"` (case-insensitive) → run the same detect flow as `--detect` (privacy notice → HTTP fetch → confirm → save).
- Any other input → save as-is.

The detect flow is extracted into a shared helper `detectLocation(w io.Writer, r io.Reader) (string, error)` so both the command handler and the init wizard call the same code path.

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
| `init.step4_header` | `=== Step 4: Set your location (optional) ===` |
| `init.step4_body` | `Your location is used for event filtering and recommendations.\nEnter city and country (e.g. "Hamburg, Germany"), type "detect" to auto-detect from your IP address, or press Enter to skip:` |
| `init.step4_prompt` | `> ` |

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
