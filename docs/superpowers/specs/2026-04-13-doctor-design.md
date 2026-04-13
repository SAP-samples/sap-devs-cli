# sap-devs doctor — Design Specification

## Goal

Add `sap-devs doctor` so developers can verify their local environment meets the tool version requirements defined in their SAP content packs. Optimised for CI use: exits 1 if any tool fails.

## Commands

```
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
    Found  string // detected version string, empty if missing
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
5. If `tool.Required == "latest"` → `ToolResult{Status: StatusUnknown, Found: version}`
6. Prepend `"v"` to version if not already present (semver requires it)
7. Parse constraint with `golang.org/x/mod/semver`; if constraint is satisfied → `StatusOK`, else → `StatusFail`

#### CheckTools deduplication

Tools with the same `ID` may appear in multiple packs. `CheckTools` checks each unique ID only once (first occurrence in the slice wins).

### New code: `cmd/doctor.go`

Thin presentation layer only.

#### Profile resolution

| Flag value | Behaviour |
|-----------|-----------|
| `""` (omitted) | `loader.LoadPacks(nil)` — all packs |
| `"@active"` | `config.LoadProfile` → `loader.FindProfile(id)` — error if no profile set or not found |
| any other string | `loader.FindProfile(value)` — error if not found |

#### Flow

1. Resolve packs via profile flag
2. Collect all `ToolDef` values from all packs into a flat slice
3. Call `content.CheckTools(tools, execRunner)` where `execRunner` uses `exec.Command`
4. Print aligned table
5. If `--fix`, print install commands for `StatusFail` and `StatusMissing` results
6. If any result is `StatusFail` or `StatusMissing` → exit 1

## Output Format

### Table (always printed)

```
TOOL             REQUIRED   FOUND      STATUS
node             >=18.0.0   v20.11.0   ok
@sap/cds-dk      >=7.0.0    6.8.2      FAIL
btp-cli          latest     3.65.0     ok (unverified)
cf-cli           >=8.0.0    -          MISSING
```

- `StatusOK` → `ok`
- `StatusUnknown` → `ok (unverified)`
- `StatusFail` → `FAIL`
- `StatusMissing` → `MISSING`

### Install commands (--fix only, printed after table)

```
Install commands:
  @sap/cds-dk   npm install -g @sap/cds-dk
  cf-cli        apt-get install cf8-cli
```

Platform is selected from `tool.Install` using `runtime.GOOS`:
- `"windows"` → windows key
- `"darwin"` → macos key
- `"linux"` → linux key
- Falls back to `"all"` key if OS-specific key is absent
- If no install command available → prints `"see: <tool.Docs>"`

## Exit Codes

| Condition | Exit code |
|-----------|-----------|
| All tools `ok` or `ok (unverified)` | 0 |
| Any tool `FAIL` or `MISSING` | 1 |
| Profile not found / config error | 1 |

## Dependencies

No new dependencies. Version comparison uses `golang.org/x/mod/semver`, which is already an indirect dependency.

## Error Handling

- Missing `tools.yaml` in a pack: already silently skipped by `LoadPack`
- Tool not installed: `StatusMissing` — not a Go error
- Version string doesn't match pattern: treated as `StatusMissing`
- `required: "latest"`: reported as `StatusUnknown`, does not count as a failure

## Testing

Tests in `internal/content/doctor_test.go` using a fake `Runner` — no real processes spawned:

- `TestCheckTool_OK` — runner returns version matching `>=18.0.0`, expect `StatusOK`
- `TestCheckTool_Fail` — runner returns version below required, expect `StatusFail`
- `TestCheckTool_Missing` — runner returns an error (tool not found), expect `StatusMissing`
- `TestCheckTool_Latest` — `required: "latest"`, any version → `StatusUnknown`
- `TestCheckTool_PatternNoMatch` — output doesn't match pattern → `StatusMissing`
- `TestCheckTools_Dedup` — same tool ID in two tools → checked once

## Files

- **Create:** `internal/content/doctor.go` — `CheckStatus`, `ToolResult`, `Runner`, `CheckTool`, `CheckTools`
- **Create:** `internal/content/doctor_test.go`
- **Create:** `cmd/doctor.go` — `doctor` Cobra command with `--profile` and `--fix` flags
