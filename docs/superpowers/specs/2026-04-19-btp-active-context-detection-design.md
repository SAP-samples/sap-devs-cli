# BTP Active Context Detection

**Date:** 2026-04-19
**Status:** Draft
**Feature:** Detect active BTP subaccount/region and Cloud Foundry org/space at inject time

## Problem

When an AI agent assists a developer, knowing the BTP environment (trial vs productive, region, CF space) significantly changes advice. Trial accounts have HANA Cloud limitations, restricted entitlements, and a 90-day lifecycle. Region affects API endpoints and data residency. CF space (dev vs prod) changes deployment advice. Currently the agent has no idea what BTP environment the developer is targeting.

## Solution

Detect BTP CLI and CF CLI context at inject time. Include the results in the rendered output as a `**BTP Environment (detected):**` section, separate from the project facts. Silently skip if neither tool is configured.

## Architecture Decision

**Approach A (chosen):** Integrate into `internal/project/detect.go` alongside existing detectors. BTP/CF fields go on `ProjectContext`, facts flow through the existing pipeline.

Alternatives considered:
- **B:** New `internal/btp/` package — more separation but excessive plumbing for ~50 lines of detection logic.
- **C:** Detection in `internal/dynamic/gather.go` — mixes detection into the gather phase which currently only enriches/mirrors data.

## Detection Logic

### New Fields on `ProjectContext`

```go
BTPSubaccount string // display name or subdomain, e.g., "my-trial-account"
BTPRegion     string // e.g., "us10", "eu10" — extracted from API endpoint URL
BTPIsTrial    bool   // heuristic: subdomain/name contains "trial" (case-insensitive)
CFOrg         string // CF organization name
CFSpace       string // CF space name, e.g., "dev"
CFRegion      string // e.g., "us10" — extracted from CF API Target URL
```

### `detectBTP(ctx *ProjectContext)`

No `cwd` parameter — BTP config is global.

**Primary (config file):**
1. Check `BTP_CLIENTCONFIG` env var for custom config path.
2. Fall back to default path:
   - Linux/macOS: `~/.config/.btp/config.json`
   - Windows: `%APPDATA%\.btp\config.json`
3. Parse JSON. Extract `TargetHierarchy.SubaccountSubdomain` (or `SubaccountDisplayName` if available) and region from `CLIURL` or `TargetHierarchy` fields.
4. Silently return if file doesn't exist or JSON structure is unexpected.

**Fallback (CLI):**
1. Exec `btp --format json target` with a 3-second timeout.
2. Parse JSON output for subaccount name and region.
3. Silently return if `btp` binary not found or command fails.

**Trial detection:**
- Case-insensitive check for "trial" in the subdomain or display name string.
- Sets `BTPIsTrial = true`.

### `detectCF(ctx *ProjectContext)`

No `cwd` parameter — CF config is global.

**Primary (config file):**
1. Read `~/.cf/config.json` (standard location across all platforms).
2. Parse with a **minimal struct** — only deserialize the fields we need:
   ```go
   type cfConfig struct {
       Target             string `json:"Target"`
       OrganizationFields struct {
           Name string `json:"Name"`
       } `json:"OrganizationFields"`
       SpaceFields struct {
           Name string `json:"Name"`
       } `json:"SpaceFields"`
   }
   ```
3. **Privacy:** `AccessToken`, `RefreshToken`, and all other credential fields are never deserialized into memory. The minimal struct ensures only names and the API endpoint URL are read.
4. Extract region from `Target` URL: `https://api.cf.us10.hana.ondemand.com` -> `us10` via regex `api\.cf\.([a-z0-9]+)\.`.
5. Silently return if file doesn't exist or JSON structure is unexpected.

**Fallback (CLI):**
1. Exec `cf target` with a 3-second timeout.
2. Parse text output for lines containing `org:`, `space:`, and `API endpoint:`.
3. Silently return if `cf` binary not found or command fails.

### Region Extraction

Region is extracted from API endpoint URLs using regex patterns:
- CF: `https://api.cf.<region>.hana.ondemand.com` -> capture `<region>`
- BTP: depends on the URL pattern in the config; extract from subdomain or path

Pattern: `(?:api\.cf\.|cli\.btp\.)([a-z0-9-]+)\.`

### Fact Rendering in `buildFacts()`

New facts appended at the end of `buildFacts()`:

```go
if ctx.BTPSubaccount != "" {
    val := ctx.BTPSubaccount
    if ctx.BTPRegion != "" {
        val += " (" + ctx.BTPRegion
        if ctx.BTPIsTrial {
            val += ", trial"
        }
        val += ")"
    } else if ctx.BTPIsTrial {
        val += " (trial)"
    }
    ctx.Facts = append(ctx.Facts, Fact{Key: "BTP subaccount", Value: val})
}
if ctx.CFOrg != "" {
    val := ctx.CFOrg + "/" + ctx.CFSpace
    if ctx.CFRegion != "" {
        val += " (" + ctx.CFRegion + ")"
    }
    ctx.Facts = append(ctx.Facts, Fact{Key: "Cloud Foundry", Value: val})
}
```

Example rendered output:
```
- BTP subaccount: my-trial-account (eu10, trial)
- Cloud Foundry: MyOrg/dev (us10)
```

## Pipeline Integration

### Call Site in `Detect()`

```go
func Detect(cwd string) (*ProjectContext, error) {
    // ...existing detectors...
    detectDefaultEnv(cwd, ctx)
    detectBTP(ctx)   // NEW — no cwd needed, reads global config
    detectCF(ctx)    // NEW — no cwd needed, reads global config
    buildFacts(ctx)
    return ctx, nil
}
```

### Rendering in Dynamic Context

Currently, `gather.go` only mirrors project info when `pc.Type != ""`. BTP/CF context should appear even when no project files are detected.

**Change condition:**
```go
// Before:
if pc != nil && pc.Type != "" {
// After:
if pc != nil && (pc.Type != "" || pc.HasBTPContext()) {
```

Where `HasBTPContext()` is a method on `ProjectContext`:
```go
func (ctx *ProjectContext) HasBTPContext() bool {
    return ctx.BTPSubaccount != "" || ctx.CFOrg != ""
}
```

### Separate Rendering Section

In `renderDynamic()`, BTP/CF facts render under a separate heading:

```
**BTP Environment (detected):**
- BTP subaccount: my-trial-account (eu10, trial)
- Cloud Foundry: MyOrg/dev (us10)
```

This requires adding a `BTPFacts` field to `ProjectInfo` (or a separate `BTPInfo` on `DynamicContext`) and a new block in `renderDynamic()`.

**Decision:** Add `BTPFacts []ProjectFact` to `ProjectInfo` and split facts at gather time: project-file facts go to `Facts`, BTP/CF facts go to `BTPFacts`. `renderDynamic()` renders `BTPFacts` under its own heading.

## Scope

Both global-scope and project-scope inject include BTP/CF detection. The BTP environment is developer-global context, not project-specific, so it appears in all inject runs.

## Privacy

- Subaccount name/subdomain and CF org/space names are included — same info visible when running `btp target` or `cf target`.
- No credentials, tokens, GUIDs, or account IDs are read or injected.
- CF config.json `AccessToken`/`RefreshToken` are excluded by using a minimal parse struct.
- No network calls are made — only local file reads and optional CLI exec.

## Testing

- Unit tests with JSON fixture files for BTP and CF config parsing.
- Unit tests for region extraction regex.
- Unit tests for trial detection heuristic.
- Unit tests for `HasBTPContext()` and `buildFacts()` with BTP/CF fields.
- `go build ./...` and `go vet ./...` locally (Windows Defender blocks `go test`).
- CI (ubuntu-latest) runs the full test suite.
- Manual: `SAP_DEVS_DEV=1 go run . inject --dry-run` to confirm BTP/CF facts appear.

## Files to Modify

| File | Change |
|------|--------|
| `internal/project/detect.go` | Add `BTP*`/`CF*` fields to `ProjectContext`, `detectBTP()`, `detectCF()`, `HasBTPContext()`, update `buildFacts()` |
| `internal/project/detect_test.go` | New tests for BTP/CF detection with fixture files |
| `internal/content/dynamic.go` | Add `BTPFacts []ProjectFact` to `ProjectInfo` |
| `internal/content/render.go` | Add `**BTP Environment (detected):**` block in `renderDynamic()` |
| `internal/dynamic/gather.go` | Update condition to include BTP context; split facts into `Facts` and `BTPFacts` |
| `CLAUDE.md` | Update Architecture section with BTP detection |

## Non-Goals

- No new health checks — BTP detection is informational, not prescriptive.
- No BTP service entitlement detection — out of scope.
- No BTP login/authentication — we only read existing config.
- No Kyma/K8s context detection — future work.
