# sap-devs doctor — Design Specification

## Goal

Add `sap-devs doctor` so developers can verify their local environment meets the tool version requirements defined in their SAP content packs. Optimised for CI use: exits 1 if any tool fails.

## Commands

```sh
sap-devs doctor                        # check all packs
sap-devs doctor --profile cap-dev      # check a specific profile's packs
sap-devs doctor --profile @active      # check the currently configured profile's packs
sap-devs doctor --fix                  # same as above + print install commands for failures
```

## Content Schema

Each pack may contain a `tools.yaml` file. The `ToolDef` struct already exists in `internal/content/pack.go`:

```go
type ToolDef struct {
    ID       string            `yaml:"id"`
    Name     string            `yaml:"name"`
    Required string            `yaml:"required"`    // semver constraint e.g. ">=18.0.0", or "latest"
    Detect   ToolDetect        `yaml:"detect"`
    Install  map[string]string `yaml:"install"`     // keys: "windows", "macos", "linux", "all"
    Docs     string            `yaml:"docs"`
}

type ToolDetect struct {
    Command string `yaml:"command"`   // e.g. "node --version"
    Pattern string `yaml:"pattern"`   // regex with one capture group for the version
}
```

`tools.yaml` is already parsed by `LoadPack` into `pack.Tools`.

## Architecture

### What already exists (do not re-implement)

- `ToolDef` and `ToolDetect` structs — in `internal/content/pack.go`
- `tools.yaml` loading — in `LoadPack` (populates `pack.Tools`)
- `LoadPacks(nil)` — loads all packs unfiltered
- `LoadPacks(profile)` — loads profile-weighted packs
- Profile resolution — `config.LoadProfile` + `loader.FindProfile` (same as `inject` and `resources`)

### New code: `internal/content/doctor.go`

```go
type CheckStatus string

const (
    StatusOK      CheckStatus = "ok"
    StatusFail    CheckStatus = "fail"
    StatusMissing CheckStatus = "missing"
    StatusUnknown CheckStatus = "unknown" // required is "latest", or version parse failed
)

type ToolResult struct {
    Tool   ToolDef
    Status CheckStatus
    Found  string // raw captured version string as returned by the detect command, empty if missing
}

// Runner abstracts exec.Command for testability.
type Runner func(command string) (string, error)

// CheckTool runs the tool's detect command, extracts version via regex, and
// compares against the required constraint.
func CheckTool(tool ToolDef, run Runner) ToolResult

// CheckTools runs CheckTool for each tool, deduplicating by ID (first seen wins).
func CheckTools(tools []ToolDef, run Runner) []ToolResult
```

#### CheckTool logic

1. Run `tool.Detect.Command` via `run`
2. If error → `ToolResult{Status: StatusMissing}`
3. Apply `tool.Detect.Pattern` regex to output; capture group 1 is the version
4. If no match → `ToolResult{Status: StatusMissing}`
5. If `tool.Required == "latest"` → `ToolResult{Status: StatusUnknown, Found: captured string}`
6. Call `parseConstraint(tool.Required, captured)` → if satisfied → `StatusOK`, else → `StatusFail`

#### parseConstraint

No external semver library is used. Implement a small helper:

```go
// parseConstraint parses a required string of the form ">=1.2.3", ">1.2.3",
// "=1.2.3", "<=1.2.3", or "<1.2.3" and compares it against found.
// Both version strings are normalised before comparison: a leading "v" is stripped,
// then each is zero-padded to exactly three components (major.minor.patch) by the
// caller before being passed to compareVersions. Returns false if either version
// cannot be parsed.
func parseConstraint(required, found string) bool

// compareVersions compares two version strings of exactly three dot-separated
// integer segments and returns -1, 0, or 1. Each segment has any trailing
// non-digit characters stripped before parsing (e.g. "0-alpine3.19" → "0",
// "7 (release)" → "7"), so real-world version strings with suffixes compare
// correctly. Always iterates exactly three positions.
func compareVersions(a, b string) int
```

The operator is extracted by scanning the prefix for `>=`, `>`, `<=`, `<`, `=` (in that order to avoid
ambiguity). The remainder is the required version. `parseConstraint` zero-pads both the required version
and `found` to exactly three `.`-separated components, strips any leading `"v"`, then calls
`compareVersions`. `parseConstraint` dispatches on the operator against the return value of
`compareVersions`. If no recognised operator prefix is found (e.g. a bare `"18.0.0"` with no operator),
`parseConstraint` returns false.

#### CheckTools deduplication

Tools with the same `ID` may appear in multiple packs. `CheckTools` checks each unique ID only once
(first occurrence in the slice wins).

### New code: `cmd/doctor.go`

Thin presentation layer only.

#### Profile resolution

The `--profile` flag uses the sentinel string `"@active"` to mean "use the configured profile". Define
it as a package-level constant `const profileActive = "@active"` in `cmd/doctor.go` so it can be reused
if other commands adopt `--profile` later.

| Flag value | Behaviour |
| ---------- | --------- |
| `""` (omitted) | `loader.LoadPacks(nil)` — all packs |
| `"@active"` | `config.LoadProfile` → if ID empty: error "no profile set"; `loader.FindProfile(id)` → if nil: error "profile not found" |
| any other string | `loader.FindProfile(value)` → if nil: error "profile not found" |

#### execRunner

`execRunner` is the default `Runner` used in production. It splits the command string on spaces:
`parts[0]` is the executable, `parts[1:]` are arguments passed to `exec.Command(parts[0], parts[1:]...)`.
It uses `cmd.CombinedOutput()` so that tools writing version output to stderr (e.g. some SAP CLI tools)
are captured correctly.

#### Flow

1. Resolve packs via profile flag
2. Collect all `ToolDef` values from all packs into a flat slice
3. Call `content.CheckTools(tools, execRunner)`
4. Print aligned table
5. If `--fix`, print install commands for `StatusFail` and `StatusMissing` results
6. If any result is `StatusFail` or `StatusMissing` → exit 1

## Output Format

### Table (always printed)

```text
TOOL             REQUIRED   FOUND      STATUS
node             >=18.0.0   v20.11.0   ok
@sap/cds-dk      >=7.0.0    6.8.2      FAIL
btp-cli          latest     3.65.0     ok (unverified)
cf-cli           >=8.0.0    -          MISSING
```

- `Found` column displays the raw captured version string (as returned by detect command), or `-` if missing
- `StatusOK` → `ok`
- `StatusUnknown` → `ok (unverified)`
- `StatusFail` → `FAIL`
- `StatusMissing` → `MISSING`

### Install commands (--fix only, printed after table)

```text
Install commands:
  @sap/cds-dk   npm install -g @sap/cds-dk
  cf-cli        apt-get install cf8-cli
```

Install commands are printed for `StatusFail` and `StatusMissing` results only. `StatusUnknown` (tools
with `required: latest`) does not trigger install output even when `--fix` is set, because the tool is
present.

**Note:** A tool with `required: latest` whose detect command errors or whose output doesn't match the
pattern will yield `StatusMissing` (not `StatusUnknown`) — it still counts as a failure and triggers
the install hint.

Platform is selected from `tool.Install` using `runtime.GOOS`:

- `"windows"` → `windows` key
- `"darwin"` → `macos` key
- `"linux"` → `linux` key
- Falls back to `"all"` key if OS-specific key is absent
- If `tool.Install` is nil/empty, or no matching key exists, prints `"see: <tool.Docs>"`

## Exit Codes

| Condition | Exit code |
| --------- | --------- |
| All tools `ok` or `ok (unverified)` | 0 |
| Any tool `FAIL` or `MISSING` | 1 |
| Profile not found / config error | 1 |

## Dependencies

No new dependencies. Version comparison is implemented with a small `parseConstraint` helper using only
the standard library (`strings`, `strconv`).

## Error Handling

- Missing `tools.yaml` in a pack: already silently skipped by `LoadPack`
- Tool not installed or detect command fails: `StatusMissing` — not a Go error
- Version string doesn't match pattern: treated as `StatusMissing`
- `required: "latest"` with tool present: `StatusUnknown` — not a failure
- `required: "latest"` with tool missing: `StatusMissing` — is a failure
- `tool.Install` nil or empty map: safe in Go (nil map read returns `""`); falls through to docs fallback

## Testing

Tests in `internal/content/doctor_test.go` using a fake `Runner` — no real processes spawned:

- `TestCheckTool_OK` — runner returns `"v20.11.0"`, required `">=18.0.0"` → `StatusOK`
- `TestCheckTool_Fail` — runner returns `"6.8.2"`, required `">=7.0.0"` → `StatusFail`
- `TestCheckTool_Missing` — runner returns an error → `StatusMissing`
- `TestCheckTool_PatternNoMatch` — runner returns output with no regex match → `StatusMissing`
- `TestCheckTool_Latest` — `required: "latest"`, runner returns version → `StatusUnknown`
- `TestCheckTool_LatestMissing` — `required: "latest"`, runner returns error → `StatusMissing`
- `TestCheckTools_Dedup` — two tools with same ID → runner called once, not twice
- `TestParseConstraint_GTE` — `">=18.0.0"` with `"18.0.0"` → true; with `"17.9.9"` → false
- `TestParseConstraint_GT` — `">18.0.0"` with `"18.0.1"` → true; with `"18.0.0"` → false
- `TestParseConstraint_PartialVersion` — `">=8"` with `"8.0.0"` → true; `"7.9.9"` → false

## Files

- **Create:** `internal/content/doctor.go` — `CheckStatus`, `ToolResult`, `Runner`, `CheckTool`, `CheckTools`, `parseConstraint`, `compareVersions`
- **Create:** `internal/content/doctor_test.go`
- **Create:** `cmd/doctor.go` — `doctor` Cobra command with `--profile` and `--fix` flags; defines `profileActive = "@active"`
