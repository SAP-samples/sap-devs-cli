# MCP CLI Wrappers for BTP and CF — Design Spec

## Goal

Wrap the `btp` and `cf` command-line tools as read-only MCP tool surfaces so AI agents can inspect Cloud Foundry apps/services/routes and BTP subaccounts/service instances conversationally — without the user context-switching to a terminal.

## Architecture

Two new abstraction packages (`internal/cfcli`, `internal/btpcli`) provide typed Go methods for each CLI command. Each method follows a **config-first, CLI-fallback** strategy: read from local config files when possible (fast, no subprocess), fall back to CLI execution for live data. MCP tool handlers call these packages and wrap results in the standard `ResultEnvelope`.

This adds **11 new MCP tools** (7 CF + 4 BTP), bringing the server from 15 to 26 tools. All tools are read-only.

## Tech Stack

- Go standard library (`os/exec`, `encoding/json`, `context`)
- Existing `ResultEnvelope` from `internal/mcpserver/envelope.go`
- `mark3labs/mcp-go` SDK for tool registration

---

## Shared Conventions

### Runner type

Both `cfcli` and `btpcli` define their own identical `Runner` type:

```go
type Runner func(command string) (string, error)
```

This is intentional duplication — each package is self-contained. Go's structural typing means the same concrete function satisfies both types without adapters. In `cmd/mcp_serve.go`, a single `func(string) (string, error)` closure (wrapping `exec.CommandContext`) is passed to both `cfcli.NewClient()` and `btpcli.NewClient()`.

The `Runner` itself is a simple subprocess executor with no timeout. **Timeouts are applied inside each `Client` method** via `context.WithTimeout`, not in the `Runner`. This lets each method control its own deadline (e.g., `Target()` could use a shorter timeout than `Apps()`). The default is 10 seconds per method call.

The runner construction in `mcp_serve.go`:

```go
cliRunner := func(command string) (string, error) {
    parts := strings.Fields(command)
    if len(parts) == 0 {
        return "", fmt.Errorf("empty command")
    }
    cmd := exec.Command(parts[0], parts[1:]...)
    out, err := cmd.CombinedOutput()
    return string(out), err
}
```

Note: no timeout in the runner itself — `Client` methods wrap each call with `context.WithTimeout`. This is different from the `execRunner` in `tools_doctor.go`, which bakes in a 5-second timeout. The CLI wrapper packages need longer, method-controlled timeouts.

### Install hints

The `cfcli` and `btpcli` packages return `*AuthError` (or `*NotInstalledError`) from their methods. These errors contain only the error message and CLI name — **not install commands**. The MCP tool handlers in `tools_cf.go` and `tools_btp.go` enrich these errors with install commands by calling `installForCurrentOS()` (which lives in `package mcpserver` and has access to `Deps.Packs` tool definitions). This keeps the CLI packages independent of the content layer.

### Pagination

CF and BTP CLIs return all results in a single call — there is no server-side pagination. Every CLI call returns the full result set. The `Client` methods always return the complete parsed output. The MCP tool handlers apply `limit` truncation in Go before returning, using the existing `clampLimit()` helper. The `total` field in `ResultEnvelope` is set to the count of all parsed results (before truncation); `count` is the number actually returned (after truncation).

---

## Package Design

### `internal/cfcli`

Wraps the Cloud Foundry CLI (`cf`).

#### Client

```go
type Runner func(command string) (string, error)

type Client struct {
    run        Runner
    configPath string        // resolved at construction, see below
    timeout    time.Duration // default 10s, applied per method call
}

func NewClient(run Runner, configPath string) *Client
```

#### Config path resolution

The CF CLI stores its config at `$CF_HOME/.cf/config.json` (if `CF_HOME` is set) or `~/.cf/config.json` (default). The caller (`mcp_serve.go`) resolves this path at construction time following the same logic as `internal/project/detect.go` lines 278-281. The resolved path is passed to `NewClient`.

#### Config struct

The `cfcli` package defines its own `cfConfig` struct that is **more complete** than the minimal one in `internal/project/detect.go`. The detect.go struct only maps `Target`, `OrganizationFields.Name`, and `SpaceFields.Name` because that's all project detection needs. The cfcli struct additionally maps `AccessToken` for login status detection:

```go
type cfConfig struct {
    Target             string `json:"Target"`
    OrganizationFields struct {
        Name string `json:"Name"`
    } `json:"OrganizationFields"`
    SpaceFields struct {
        Name string `json:"Name"`
    } `json:"SpaceFields"`
    AccessToken string `json:"AccessToken"`
}
```

**Login status:** If `AccessToken` is empty or the config file doesn't exist, the user is not logged in.

#### Config-first strategy

CF stores target metadata in `~/.cf/config.json`:

```json
{
  "Target": "https://api.cf.us10.hana.ondemand.com",
  "OrganizationFields": { "Name": "my-org", "GUID": "..." },
  "SpaceFields": { "Name": "dev", "GUID": "..." },
  "AccessToken": "bearer eyJ..."
}
```

- **Available from config:** org name, space name, API endpoint, region (extracted from endpoint URL), login status (presence of `AccessToken`).
- **Requires CLI execution:** apps, services, routes, domains, buildpacks, env vars.

The `Target()` method reads config first; all other methods execute the CLI.

#### Methods

| Method | CLI Command | Returns |
|--------|-------------|---------|
| `Target(ctx)` | config-first, fallback `cf target` | `TargetInfo{Org, Space, API, Region, LoggedIn}` |
| `Apps(ctx)` | `cf apps` | `[]App{Name, State, Instances, Memory, Routes}` |
| `Services(ctx)` | `cf services` | `[]Service{Name, Service, Plan, BoundApps, Status}` |
| `Env(ctx, app)` | `cf env <app>` | `AppEnv{SystemProvided, UserProvided, Running, Staging}` |
| `Routes(ctx)` | `cf routes` | `[]Route{Domain, Host, Path, Apps}` |
| `Domains(ctx)` | `cf domains` | `[]Domain{Name, Type, Status}` |
| `Buildpacks(ctx)` | `cf buildpacks` | `[]Buildpack{Name, Position, Enabled, Locked, Filename}` |

All methods take `ctx context.Context` as their first parameter. Each method derives a child context via `context.WithTimeout(ctx, c.timeout)` for the CLI subprocess. MCP tool handlers pass the request context from `mcp.CallToolRequest`, enabling cancellation propagation.

#### Text parsing

CF CLI outputs column-aligned text. Each parser:
1. Splits output into lines
2. Finds the header line by matching known column names (e.g., `"name"`, `"requested state"`, `"instances"`)
3. Uses the header line to determine **column start positions** (character offsets where each header word begins)
4. For each data line, extracts fields using those column positions — the last column captures everything from its start position to end of line (handles multi-word values like comma-separated routes)
5. Trims whitespace and maps to typed structs

This column-offset approach handles the known edge case where the last field can contain spaces or commas (e.g., `cf apps` routes column, `cf routes` apps column).

CF output formats are stable across v8.x.

#### Credential redaction in `cf env`

`cf env` exposes bound service credentials. The `Env()` method redacts values for known sensitive keys before returning:

**Redacted keys:** `password`, `clientsecret`, `client_secret`, `token`, `access_token`, `refresh_token`, `key`, `secret`, `private_key`, `certificate`.

**Redaction:** Value replaced with `"[REDACTED]"`. The agent sees the structure (which services are bound, what env vars exist) without leaking secrets.

### `internal/btpcli`

Wraps the BTP CLI (`btp`).

#### Client

```go
type Runner func(command string) (string, error)

type Client struct {
    run        Runner
    configPath string        // resolved at construction, see below
    timeout    time.Duration // default 10s, applied per method call
}

func NewClient(run Runner, configPath string) *Client
```

#### Config path resolution

The BTP CLI config path resolution must replicate the logic from `internal/project/detect.go` `defaultBTPConfigPath()` (lines 371-388), including the fallback for older BTP CLI versions:

- Check `$BTP_CLIENTCONFIG` env var first
- Windows: `%APPDATA%/SAP/btp/config.json`
- Linux/macOS: `~/.config/btp/config.json` (primary), falling back to `~/.config/.btp/config.json` (older BTP CLI v1.x layout)

The caller (`mcp_serve.go`) resolves this path at construction time. The `.btp` fallback is load-bearing — users with older BTP CLI installations will get silent config-read failures without it.

#### Config struct

Replicate the `btpConfig` struct from `internal/project/detect.go` (lines 301-307) with **identical field names and JSON tags**:

```go
type btpConfig struct {
    TargetHierarchy struct {
        GlobalAccountSubdomain string `json:"GlobalAccountSubdomain"`
        SubaccountSubdomain    string `json:"SubaccountSubdomain"`
    } `json:"TargetHierarchy"`
    CLIServerURL string `json:"CLIServerURL"`
}
```

The capital-initial JSON keys match the actual BTP config file format. Using different casing will cause silent unmarshal failures.

**Login status:** If `SubaccountSubdomain` is empty or the config file doesn't exist, the user hasn't targeted a subaccount.

#### Config-first strategy

BTP stores target metadata in its config file:

```json
{
  "TargetHierarchy": {
    "GlobalAccountSubdomain": "my-ga",
    "SubaccountSubdomain": "eu10-myapp-dev"
  },
  "CLIServerURL": "https://cli.btp.cloud.sap"
}
```

- **Available from config:** subaccount subdomain, global account, region (extracted from subdomain via regex `^([a-z]{2}\d{2})`), trial flag (heuristic substring match on `"trial"`), login status (presence of `SubaccountSubdomain`).
- **Requires CLI execution:** subaccount listing, service instances, role collections.

#### Methods

| Method | CLI Command | Returns |
|--------|-------------|---------|
| `Target(ctx)` | config-first, fallback `btp --format json target` | `TargetInfo{Subaccount, GlobalAccount, Region, Trial, LoggedIn}` |
| `Subaccounts(ctx)` | `btp --format json list accounts/subaccount` | `[]Subaccount{Name, Subdomain, Region, State, Parent}` |
| `ServiceInstances(ctx)` | `btp --format json list services/instance` | `[]ServiceInstance{Name, Service, Plan, Status, Created}` |
| `RoleCollections(ctx)` | `btp --format json list security/role-collection` | `[]RoleCollection{Name, Description, RoleCount}` |

All methods take `ctx context.Context` as their first parameter, same as the CF side.

#### JSON parsing

BTP CLI's `--format json` flag returns structured JSON natively. No text parsing needed — just `json.Unmarshal` into typed structs and reshape for the MCP response.

---

## Auth Error Handling

### Error types in CLI packages

Both packages define two error types:

```go
type AuthError struct {
    CLI     string // "cf" or "btp"
    Message string
}

type NotInstalledError struct {
    CLI     string // "cf" or "btp"
    Message string
}
```

These are returned from `Client` methods. They carry only the error message — **not install commands or fix suggestions**. The MCP tool handlers enrich them (see below).

### Error detection patterns

Both packages detect errors by pattern-matching CLI output:

**CF patterns:**
- `"Not logged in"`
- `"No API endpoint set"`
- `"FAILED"` combined with `"not authenticated"`

**BTP patterns:**
- `"Login required"`
- `"You are not logged in"`
- `"session has expired"`

**CLI not found:** If `Runner` returns an error wrapping `exec.ErrNotFound`, the method returns `*NotInstalledError`.

### MCP handler error enrichment

The MCP tool handlers in `tools_cf.go` and `tools_btp.go` check for `*AuthError` and `*NotInstalledError` via type assertions. They format structured JSON as a **successful MCP tool result** (`mcp.NewToolResultText`), not as `mcp.NewToolResultError`.

**Rationale:** `mcp.NewToolResultError` returns a plain string that agents display as-is. A structured JSON body gives the agent parseable fields (`error`, `cli`, `fix`, `hint`) so it can reason about the failure, suggest the exact login/install command, and retry. This is intentional — the existing `tools_discovery.go` uses the same pattern (network errors surfaced via `wrapResultsWithHint` rather than `NewToolResultError`).

Auth error response:

```json
{
  "error": "not_authenticated",
  "cli": "cf",
  "message": "Not logged in to Cloud Foundry.",
  "fix": "Run: cf login -a https://api.cf.us10.hana.ondemand.com",
  "hint": "The cf CLI requires an active login session. After logging in, retry the command."
}
```

The `fix` field includes the API endpoint from config when available. The handler reads the CF config to get the endpoint even when the CLI call fails.

Not-installed response:

```json
{
  "error": "cli_not_installed",
  "cli": "cf",
  "message": "Cloud Foundry CLI is not installed.",
  "fix": "Install: brew install cloudfoundry/tap/cf-cli@8",
  "hint": "The cf CLI is required for Cloud Foundry operations. Install it and run 'cf login' to authenticate."
}
```

Install commands are resolved by the MCP handler using `installForCurrentOS()` from `package mcpserver`, which reads `tools.yaml` definitions from `Deps.Packs`. The `cfcli`/`btpcli` packages never import from `mcpserver`.

---

## MCP Tool Registration

### New files

```
internal/mcpserver/
  tools_cf.go     — registerCFTools(), 7 tool handlers
  tools_btp.go    — registerBTPTools(), 4 tool handlers
```

### CF Tools

| MCP Tool | Description | Parameters |
|----------|-------------|------------|
| `cf_target` | Get current CF target (org, space, API endpoint, region, login status) | — |
| `cf_apps` | List deployed apps with state, instances, memory, and routes | `limit` (optional) |
| `cf_services` | List service instances with plan, bound apps, and status | `limit` (optional) |
| `cf_env` | Get environment variables for an app (credentials redacted) | `app` (required) |
| `cf_routes` | List routes with domain, host, path, and bound apps | `limit` (optional) |
| `cf_domains` | List domains with type (shared/private) and status | `limit` (optional) |
| `cf_buildpacks` | List buildpacks with position, enabled status, and filename | `limit` (optional) |

### BTP Tools

| MCP Tool | Description | Parameters |
|----------|-------------|------------|
| `btp_target` | Get current BTP target (subaccount, region, global account, trial flag, login status) | — |
| `btp_subaccounts` | List subaccounts with name, region, state, and parent directory | `limit` (optional) |
| `btp_service_instances` | List BTP service instances with name, plan, and status | `limit` (optional) |
| `btp_role_collections` | List role collections with name, description, and role count | `limit` (optional) |

### Response format

All tools return `ResultEnvelope` with `count`, `total`, `results`, `hint` — identical pattern to the existing 15 tools. The CLI is always called for the full result set; `limit` is applied in Go after parsing. `total` reflects all parsed results; `count` reflects the truncated set returned.

### Deps integration

```go
type Deps struct {
    // ... existing fields ...
    CFClient  *cfcli.Client  // nil if cf CLI not detected at startup
    BTPClient *btpcli.Client // nil if btp CLI not detected at startup
}
```

Constructed in `cmd/mcp_serve.go`:

1. **CLI detection at startup:** Use `exec.LookPath("cf")` and `exec.LookPath("btp")` — no subprocess execution, instant. This avoids adding latency to MCP server startup (LookPath checks PATH only; running `cf --version` would add ~300ms).
2. If found, create client with the shared `cliRunner` closure and the platform-resolved config path.
3. If not found, set to `nil` — the MCP tools still register but return "CLI not installed" with install instructions when called.

### Server instructions

Add to the existing instructions string:

> "Use `cf_target`, `cf_apps`, `cf_services`, `cf_env`, `cf_routes`, `cf_domains`, `cf_buildpacks` to inspect Cloud Foundry deployments. Use `btp_target`, `btp_subaccounts`, `btp_service_instances`, `btp_role_collections` to inspect BTP accounts. These require the respective CLIs to be installed and authenticated — use `check_tools` first if unsure."

---

## Timeouts

All CLI wrapper commands use a **10-second timeout** via `context.WithTimeout`, applied inside each `Client` method (not in the `Runner`). This is longer than the 5-second timeout used by `check_tools` because:

- `cf apps` and `btp list` make HTTP API calls to remote services
- First call after login can be slow (token refresh)
- Network latency varies

The timeout is configurable per-client at construction time but 10s is the default. If a command times out, the client returns an error with message: `"Command timed out after 10s. The CF/BTP API may be slow. Try again."` — the MCP handler wraps this in a hint.

---

## File Structure

```
internal/cfcli/
  client.go       — Client struct, Runner type, NewClient(), config reading, cfConfig struct
  commands.go     — Target(), Apps(), Services(), Env(), Routes(), Domains(), Buildpacks()
  parse.go        — column-offset text parsers for CF tabular output (one function per command)
  auth.go         — AuthError, NotInstalledError types, auth pattern detection

internal/btpcli/
  client.go       — Client struct, Runner type, NewClient(), config reading, btpConfig struct
  commands.go     — Target(), Subaccounts(), ServiceInstances(), RoleCollections()
  auth.go         — AuthError, NotInstalledError types, auth pattern detection

internal/mcpserver/
  tools_cf.go     — registerCFTools(), 7 MCP tool handlers, error enrichment with install hints
  tools_btp.go    — registerBTPTools(), 4 MCP tool handlers, error enrichment with install hints
```

---

## Not in Scope

- **Write operations** — no `cf push`, `cf bind-service`, `btp create`. Future iteration with confirmation gating.
- **`cf logs`** — requires streaming/long-lived connection, fundamentally different pattern.
- **Service key creation/viewing** — write-adjacent; `cf env` already shows bound credentials.
- **BTP entitlements** — `btp list accounts/entitlement` is useful but lower priority. Can add later without design changes.
- **Multi-target switching** — no switching between orgs/spaces/subaccounts. Tools read whatever the CLI is currently targeting.
- **OAuth/token management** — we use the CLI's existing auth. No token refresh, no credential storage.
- **Caching** — CLI output is live state. Unlike tutorials or learning journeys, caching `cf apps` would serve stale data. Every call hits the CLI.

---

## Dependencies

- Existing `internal/mcpserver/envelope.go` — `ResultEnvelope`, `wrapResults()`, `clampLimit()`
- Existing `internal/mcpserver/tools_doctor.go` — `installForCurrentOS()` for enriching install hints
- Existing `content/packs/btp-core/tools.yaml` — install commands for CF and BTP CLIs
- Existing `internal/project/detect.go` — reference for config path resolution logic and struct field names (not imported; logic replicated)
