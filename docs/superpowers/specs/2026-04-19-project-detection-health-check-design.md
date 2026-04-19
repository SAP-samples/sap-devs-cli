# Project-Aware Context Detection & Health Check

**Date:** 2026-04-19
**Status:** Draft
**Feature:** Enhance `sap-devs doctor` with project health checks; inject project-specific context into AI tools

## Problem

A developer working on a CAP project with HANA Cloud on BTP gets the same injected context as one working on a standalone SQLite prototype. The agent can't give version-appropriate or environment-appropriate advice without knowing what it's working with. Additionally, there's no way to surface project misconfigurations or outdated dependencies — developers discover these problems late, during deployment or runtime.

## Goals

1. Detect project characteristics from files in the working directory (no network calls)
2. Inject detected facts + health warnings into AI tool context during `sap-devs inject`
3. Surface project issues via `sap-devs doctor` with actionable fix suggestions

## Non-Goals

- Pack-driven detection rules (checks.yaml per pack) — future enhancement
- ABAP project detection (pending ADT-in-VSCode availability — noted for future)
- Network-based version checking (latest versions come from pack metadata / sync cache)

## Architecture

### New Package: `internal/project`

A new package owns all detection and health checking. Two entry points:

- `Detect(cwd string) (*ProjectContext, error)` — scans project files, returns structured facts
- `Check(ctx *ProjectContext, packs []*content.Pack) []Finding` — runs health checks against detected context and pack knowledge

Both `cmd/inject.go` and `cmd/doctor.go` consume this package.

### Integration Points

- `internal/dynamic/gather.go`: Replace `detectProjectType()` with a call to `project.Detect()`. The `DynamicContext` struct gains a `Project *project.ProjectContext` field, replacing the current `ProjectType string`.
- `internal/content/render.go`: `renderDynamic()` renders detected facts and health warnings into the injected block.
- `cmd/doctor.go`: Calls `project.Detect()` then `project.Check()` after the existing tool-version table.

## Detection Engine

### Types

```go
package project

// ProjectContext holds everything detected about the current project.
type ProjectContext struct {
    Type          string          // "CAP (Node.js)", "CAP (Java)", "MTA", "Fiori", "Node.js", ""
    CAPVersion    string          // detected @sap/cds version from package.json
    LatestCAP     string          // latest known CAP version from pack metadata
    Database      string          // "hana", "sqlite", "postgres", ""
    Deployment    string          // "mta-cf", "helm-kyma", ""
    Auth          string          // "xsuaa", ""
    HasCDSRC      bool            // .cdsrc.json present
    HasDefaultEnv bool            // default-env.json present
    Facts         []Fact          // structured key-value facts for injection
    RawFiles      map[string]bool // which signal files were found
}

// Fact is a single detected property of the project.
type Fact struct {
    Key   string // e.g., "CAP version", "Database", "Deployment"
    Value string // e.g., "@sap/cds 9.6.2", "SAP HANA Cloud"
    Warn  string // optional warning (e.g., "update available: 9.8.0")
}
```

### Detection Signals

`Detect()` scans these files in the working directory:

| File | What it detects |
|---|---|
| `package.json` | CAP presence (`@sap/cds` dep), version, HANA require (`hana` in cds.requires), other SAP deps |
| `pom.xml` | CAP Java (`com.sap.cds` dependency), version |
| `.cdsrc.json` | Custom CDS configuration present |
| `xs-security.json` | XSUAA / OAuth2 in use |
| `.mta.yaml` / `mta.yaml` | MTA deployment to Cloud Foundry |
| `default-env.json` | Local CF environment simulation |
| `xs-app.json` | App Router / Fiori app |
| `chart/` or `helm/` directory | Kyma/Kubernetes deployment |

No network calls. Pure file system reads. `Detect()` populates both the typed fields (CAPVersion, Database, etc.) and the `Facts` slice (for flexible rendering). `RawFiles` records which signal files exist, used by health checks.

### Extensibility

The detector is structured as a series of independent check functions called sequentially:

```go
func Detect(cwd string) (*ProjectContext, error) {
    ctx := &ProjectContext{RawFiles: make(map[string]bool)}
    detectCAP(cwd, ctx)
    detectMTA(cwd, ctx)
    detectAuth(cwd, ctx)
    detectAppRouter(cwd, ctx)
    detectKyma(cwd, ctx)
    detectDefaultEnv(cwd, ctx)
    buildFacts(ctx)
    return ctx, nil
}
```

Adding a new project type (e.g., ABAP, UI5 standalone) means adding a new `detectX()` function and calling it from `Detect()`. Each detector is independent and reads only the files it needs.

## Health Check Engine

### Types

```go
// Finding represents a single health check result.
type Finding struct {
    Category string // "dependency", "version", "practice", "constraint"
    Severity string // "error", "warning", "info"
    Message  string // human-readable description
    Fix      string // suggested fix command or action (optional)
}
```

### Check Categories

`Check()` runs four categories of checks, each implemented as a Go function:

**1. Dependency checks** (`dependency`):
- CAP project missing `@sap/cds-dk` in devDependencies → warning
- `xs-security.json` missing but XSUAA-related deps detected → error
- `.mta.yaml` references modules not found in project → warning
- Missing `.gitignore` in a project with `default-env.json` → warning

**2. Version staleness** (`version`):
- Compare detected `@sap/cds` version against latest known from pack metadata
- >2 minor versions behind → warning; >1 major behind → error
- Same for `@sap/cds-dk`, Java CAP SDK version

**3. Best-practice / anti-pattern** (`practice`):
- `default-env.json` not in `.gitignore` → error (credential leak risk)
- No `lint` script in `package.json` for CAP projects → warning
- Hardcoded destinations in `xs-app.json` → warning

**4. Constraint compliance** (`constraint`):
- Curated checks that map pack constraints to detectable file states
- E.g., CAP constraints say "use cds lint" → check for lint script in package.json
- Initial set is small and manually curated; does not parse constraints.md programmatically

### How Checks Access Pack Knowledge

Each check function receives the `ProjectContext` (detected facts) and the loaded `[]*content.Pack`. Packs provide:
- Latest known versions (from pack metadata embedded at sync time)
- `tools.yaml` tool definitions (already used by doctor for tool checks)
- `constraints.md` content (for human reference; machine-checkable rules are manually curated in Go)

### Version Comparison

Latest known versions for staleness checks are stored as a new `versions` map in `pack.yaml` metadata:

```yaml
# content/packs/cap/pack.yaml (new field)
versions:
  "@sap/cds": "9.8.0"
  "@sap/cds-dk": "9.8.0"
```

**Loading path — requires changes to:**

1. `packMeta` struct in `internal/content/pack.go` — add `Versions map[string]string` field
2. `Pack` struct in `internal/content/pack.go` — add `Versions map[string]string` field
3. `LoadPack()` in `internal/content/pack.go` — propagate `packMeta.Versions` to `Pack.Versions`
4. `content/schemas/pack.schema.json` — add `versions` property (object with string values)

**Version resolution:** `Check()` collects `versions` maps from all loaded packs. When multiple packs declare the same key, the highest-weight pack wins. For CAP version staleness specifically, the CAP pack's `versions["@sap/cds"]` is the authoritative latest.

**Semver comparison:** Use manual major.minor.patch parsing (no external dependency). Compare numeric components left-to-right. Staleness thresholds: >2 minor versions behind → warning; >1 major → error.

### Gitignore Parsing

Several health checks (e.g., `default-env.json` not in `.gitignore`) require reading and pattern-matching against `.gitignore`. A small utility function `isGitignored(cwd, filename string) bool` reads `.gitignore` from the project root and checks for exact filename matches. No glob/negation support needed for the initial checks — just line-by-line string comparison.

## CLI Output

### Enhanced `sap-devs doctor`

Running `sap-devs doctor` in a project directory shows two sections:

```
Tool Versions
─────────────────────────────────────────────────────
Tool                 Required     Found        Status
─────────────────────────────────────────────────────
node                 >=20         22.14.0      ✓ OK
cds                  >=9          9.8.0        ✓ OK
cf                   >=8          8.12.1       ✓ OK

Project Health (CAP Node.js — MTA to Cloud Foundry)
─────────────────────────────────────────────────────
✗ ERROR    default-env.json is not in .gitignore (credential leak risk)
           Fix: Add 'default-env.json' to .gitignore
⚠ WARNING  @sap/cds 9.4.0 is 2 minor versions behind latest 9.8.0
           Fix: Run 'npm update @sap/cds'
⚠ WARNING  No 'lint' script in package.json
           Fix: Add '"lint": "npx cds lint"' to scripts
ℹ INFO     .cdsrc.json detected with custom configuration
```

When run outside a detectable project directory, the "Project Health" section is omitted silently.

### Flags

| Flag | Behavior |
|---|---|
| (none) | Tool checks + project health (both) |
| `--tools-only` | Tool version checks only (legacy behavior) |
| `--project-only` | Project health checks only |
| `--fix` | Existing flag; now also prints fix commands for project findings |
| `--profile` | Existing flag; unchanged |

### Exit Code

Non-zero if any `error`-severity finding exists (tool failures or project health errors). Unchanged from current behavior for tool checks.

## Inject Integration

### Rendered Output

The `## sap-devs Runtime Context` section gains a **Project Context** subsection:

```markdown
## sap-devs Runtime Context

**CLI:** sap-devs v1.5.0 | **Profile:** CAP Developer | **Packs:** base, cap, btp-core

**Project Context (detected):**
- CAP version: @sap/cds 9.6.2 (latest: 9.8.0 — update available)
- Database: SAP HANA Cloud
- Deployment: MTA to Cloud Foundry
- Auth: XSUAA (xs-security.json detected)
- ⚠ default-env.json not in .gitignore — credential leak risk
```

**Rendering rules:**
- Facts are always rendered (compact key-value lines)
- Only `error` and `warning` severity findings are injected (not `info`)
- If no project detected, the subsection is omitted entirely
- Warnings are prefixed with `⚠` for agent visibility

### Implementation in `render.go`

`renderDynamic()` already renders CLI version, profile, pack IDs, sync time, project type, MCP servers, and commands. The change:

1. Replace the single `ProjectType` line with the full `Project Context` block
2. Read `ProjectContext.Facts` for the key-value lines
3. Run `Check()` inline (lightweight, no I/O) and append error/warning findings
4. The `renderProjectContext(pc *project.ProjectContext, findings []Finding) string` helper builds the markdown

### DynamicContext Changes

```go
// internal/content/dynamic.go (changed field)
type DynamicContext struct {
    CLIVersion      string
    ActiveProfile   string
    LoadedPackIDs   []string
    LastSynced      *time.Time              // pointer — nil means "never synced"
    Project         *project.ProjectContext  // was: ProjectType string
    WiredMCPServers []WiredMCPEntry
    Commands        []CommandInfo
}
```

The `inject` command calls `project.Detect(cwd)` and `project.Check(ctx, packs)` before building the adapter engine. Results flow through `DynamicContext.Project` into rendering.

## Data Flow

```
cmd/inject.go
  ├─ content.LoadPacks()
  ├─ project.Detect(cwd)        ← NEW
  ├─ project.Check(ctx, packs)  ← NEW
  ├─ dynamic.GatherDynamic()    ← uses project.ProjectContext
  ├─ adapter.Engine.Run()
  │   └─ content.RenderContext()
  │       └─ renderDynamic()    ← renders project facts + warnings
  └─ print results

cmd/doctor.go
  ├─ content.LoadPacks()
  ├─ content.CheckTools()       ← existing tool checks
  ├─ project.Detect(cwd)        ← NEW
  ├─ project.Check(ctx, packs)  ← NEW
  └─ printProjectHealth()       ← NEW
```

## Testing Strategy

Since `go test` doesn't work on Windows (Defender blocks), tests target CI (ubuntu-latest):

- **Unit tests for `internal/project`**: Create temp directories with fixture files (package.json, .mta.yaml, etc.), run `Detect()` and `Check()`, assert expected results
- **Detection fixtures**: Test each signal file independently and in combination
- **Version comparison tests**: Edge cases for semver parsing and staleness thresholds
- **Rendering tests**: Verify `renderProjectContext()` output format
- **Integration test**: End-to-end `doctor` command with a fixture project directory

Local validation: `go build ./...` and `go vet ./...` per project conventions.

## i18n

All user-facing strings (column headers, severity labels, finding messages, fix suggestions) go through `internal/i18n` with keys under the `doctor.project.*` namespace. Initial implementation: English only, with German translations as a follow-up.

## Future Extensions

- **ABAP project detection**: Once ADT is available in VS Code, detect ABAP project markers (`.abapgit.xml`, `abappackage.json`, ADT project files)
- **UI5 standalone detection**: `ui5.yaml`, `@ui5/cli` in devDependencies
- **Pack-driven checks**: `checks.yaml` per pack defining custom detection rules (file presence, dependency patterns, version constraints) without code changes
- **Auto-fix mode**: `sap-devs doctor --auto-fix` applies safe fixes automatically (e.g., adding `.gitignore` entries)
- **JSON output**: `sap-devs doctor --json` for CI/CD pipeline integration
