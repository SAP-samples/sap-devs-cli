# MCP CLI Wrappers (BTP & CF) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 11 new MCP tools (7 CF + 4 BTP) that wrap the `btp` and `cf` CLIs, using a config-first/CLI-fallback strategy, bringing the server from 15 to 26 tools.

**Architecture:** Two self-contained packages (`internal/cfcli`, `internal/btpcli`) provide typed Go methods wrapping CLI commands. Each uses a `Runner func(string)(string,error)` for subprocess execution and owns its own timeouts via `context.WithTimeout`. MCP tool handlers in `internal/mcpserver/tools_cf.go` and `tools_btp.go` call these packages and enrich errors with install hints.

**Tech Stack:** Go standard library (`os/exec`, `encoding/json`, `context`, `regexp`), existing `ResultEnvelope` from `internal/mcpserver/envelope.go`, `mark3labs/mcp-go` SDK for tool registration.

**Spec:** `docs/superpowers/specs/2026-04-21-mcp-cli-wrappers-design.md`

**Windows note:** `go test` fails locally due to Windows Defender. Use `go build ./...` + `go vet ./...` locally; CI is the authoritative test runner.

---

## File Map

### New files

| File | Responsibility |
|------|----------------|
| `internal/cfcli/client.go` | `Client` struct, `Runner` type, `NewClient()`, CF config reading, `cfConfig` struct |
| `internal/cfcli/auth.go` | `AuthError`, `NotInstalledError` types, auth pattern detection helpers |
| `internal/cfcli/commands.go` | `Target()`, `Apps()`, `Services()`, `Env()`, `Routes()`, `Domains()`, `Buildpacks()` methods |
| `internal/cfcli/parse.go` | Column-offset text parsers for CF tabular output (one function per command) |
| `internal/btpcli/client.go` | `Client` struct, `Runner` type, `NewClient()`, BTP config reading, `btpConfig` struct |
| `internal/btpcli/auth.go` | `AuthError`, `NotInstalledError` types, auth pattern detection helpers |
| `internal/btpcli/commands.go` | `Target()`, `Subaccounts()`, `ServiceInstances()`, `RoleCollections()` methods |
| `internal/mcpserver/tools_cf.go` | `registerCFTools()`, 7 MCP tool handlers, auth/install error enrichment |
| `internal/mcpserver/tools_btp.go` | `registerBTPTools()`, 4 MCP tool handlers, auth/install error enrichment |

### Modified files

| File | Change |
|------|--------|
| `internal/mcpserver/server.go` | Add `CFClient`/`BTPClient` to `Deps`, add `registerCFTools`/`registerBTPTools` calls, expand instructions string |
| `cmd/mcp_serve.go` | Add `cliRunner` closure, `exec.LookPath` detection, CF/BTP client construction, config path resolution |
| `docs/mcp-server.md` | Document 11 new tools, update tool count, add CF/BTP tool tables |

---

### Task 1: cfcli auth types

**Files:**
- Create: `internal/cfcli/auth.go`

- [ ] **Step 1: Create `internal/cfcli/auth.go` with error types and detection helpers**

```go
package cfcli

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type AuthError struct {
	CLI     string
	Message string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("%s: %s", e.CLI, e.Message)
}

type NotInstalledError struct {
	CLI     string
	Message string
}

func (e *NotInstalledError) Error() string {
	return fmt.Sprintf("%s: %s", e.CLI, e.Message)
}

func checkAuthError(output string) *AuthError {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "not logged in"):
		return &AuthError{CLI: "cf", Message: "Not logged in to Cloud Foundry."}
	case strings.Contains(lower, "no api endpoint set"):
		return &AuthError{CLI: "cf", Message: "No API endpoint set. Run 'cf api' first."}
	case strings.Contains(lower, "failed") && strings.Contains(lower, "not authenticated"):
		return &AuthError{CLI: "cf", Message: "Authentication expired. Run 'cf login' to re-authenticate."}
	}
	return nil
}

func checkNotInstalled(err error) *NotInstalledError {
	if err != nil && errors.Is(err, exec.ErrNotFound) {
		return &NotInstalledError{CLI: "cf", Message: "Cloud Foundry CLI is not installed."}
	}
	return nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/cfcli/...`
Expected: BUILD SUCCESS (no output)

- [ ] **Step 3: Commit**

```bash
git add internal/cfcli/auth.go
git commit -m "feat(cfcli): add auth and not-installed error types with detection helpers"
```

---

### Task 2: cfcli client and config

**Files:**
- Create: `internal/cfcli/client.go`

- [ ] **Step 1: Create `internal/cfcli/client.go` with Client, Runner, config structs, and config reading**

```go
package cfcli

import (
	"encoding/json"
	"os"
	"time"
)

type Runner func(command string) (string, error)

type Client struct {
	run        Runner
	configPath string
	timeout    time.Duration
}

func NewClient(run Runner, configPath string) *Client {
	return &Client{
		run:        run,
		configPath: configPath,
		timeout:    10 * time.Second,
	}
}

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

func (c *Client) readConfig() *cfConfig {
	if c.configPath == "" {
		return nil
	}
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return nil
	}
	var cfg cfConfig
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	return &cfg
}
```

Note: The `cfConfig` struct intentionally has `AccessToken` which the minimal `cfConfig` in `internal/project/detect.go` (line 222) does not — the detect.go struct only maps what project detection needs.

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/cfcli/...`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/cfcli/client.go
git commit -m "feat(cfcli): add Client struct, Runner type, config reading with AccessToken"
```

---

### Task 3: cfcli Target method, runWithContext, and parseCFTarget

This task creates both `commands.go` and `parse.go` together so the package compiles at every commit. The Target method uses a config-first strategy: read from `~/.cf/config.json` when possible, fall back to CLI execution for live data.

**Files:**
- Create: `internal/cfcli/commands.go`
- Create: `internal/cfcli/parse.go`
- Modify: `internal/cfcli/client.go` (add `runWithContext`)

- [ ] **Step 1: Add `runWithContext` helper to `client.go`**

Add to the import block in `client.go`: `"context"` and `"fmt"`. Then append the method:

```go
func (c *Client) runWithContext(ctx context.Context, command string) (string, error) {
	type result struct {
		out string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := c.run(command)
		ch <- result{out, err}
	}()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("command timed out after %s — the CF API may be slow, try again", c.timeout)
	case r := <-ch:
		return r.out, r.err
	}
}
```

This wraps the synchronous `Runner` with context cancellation. The `Runner` itself has no timeout — the `Client` method owns the deadline via `context.WithTimeout`.

- [ ] **Step 2: Create `internal/cfcli/parse.go` with `parseCFTarget` and column-offset helpers**

```go
package cfcli

import "strings"

func parseCFTarget(output string) TargetInfo {
	var info TargetInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "org:"):
			info.Org = strings.TrimSpace(strings.TrimPrefix(line, "org:"))
		case strings.HasPrefix(line, "space:"):
			info.Space = strings.TrimSpace(strings.TrimPrefix(line, "space:"))
		case strings.HasPrefix(line, "API endpoint:"):
			info.API = strings.TrimSpace(strings.TrimPrefix(line, "API endpoint:"))
		}
	}
	if m := reCFRegion.FindStringSubmatch(info.API); len(m) >= 2 {
		info.Region = m[1]
	}
	info.LoggedIn = info.Org != ""
	return info
}

func findColumns(headerLine string, names []string) []int {
	lower := strings.ToLower(headerLine)
	positions := make([]int, len(names))
	for i, name := range names {
		positions[i] = strings.Index(lower, strings.ToLower(name))
	}
	return positions
}

func extractField(line string, start, end int) string {
	if start < 0 || start >= len(line) {
		return ""
	}
	if end < 0 || end > len(line) {
		end = len(line)
	}
	return strings.TrimSpace(line[start:end])
}
```

Note: `parseCFTarget` replicates the parsing logic from `internal/project/detect.go:251-263` (`parseCFTargetOutput`). The `findColumns`/`extractField` helpers are the foundation of the column-offset text parsing approach for all other CF commands.

- [ ] **Step 3: Create `internal/cfcli/commands.go` with Target method and TargetInfo struct**

```go
package cfcli

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var reCFRegion = regexp.MustCompile(`api\.cf\.([a-z0-9-]+)\.hana\.ondemand\.com`)

type TargetInfo struct {
	Org      string `json:"org"`
	Space    string `json:"space"`
	API      string `json:"api"`
	Region   string `json:"region"`
	LoggedIn bool   `json:"logged_in"`
}

func (c *Client) Target(ctx context.Context) (TargetInfo, error) {
	cfg := c.readConfig()
	if cfg != nil && cfg.OrganizationFields.Name != "" {
		region := ""
		if m := reCFRegion.FindStringSubmatch(cfg.Target); len(m) >= 2 {
			region = m[1]
		}
		return TargetInfo{
			Org:      cfg.OrganizationFields.Name,
			Space:    cfg.SpaceFields.Name,
			API:      cfg.Target,
			Region:   region,
			LoggedIn: cfg.AccessToken != "",
		}, nil
	}

	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf target")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return TargetInfo{}, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return TargetInfo{}, authErr
		}
		return TargetInfo{}, fmt.Errorf("cf target failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return TargetInfo{}, authErr
	}

	info := parseCFTarget(out)
	return info, nil
}
```

The region regex is the same pattern as `internal/project/detect.go:219` — replicated intentionally per the spec. The `fmt` import is included from the start.

- [ ] **Step 4: Verify compilation**

Run: `go build ./internal/cfcli/...`
Expected: BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add internal/cfcli/commands.go internal/cfcli/parse.go internal/cfcli/client.go
git commit -m "feat(cfcli): add Target method with config-first strategy, text parsers, and runWithContext"
```

---

### Task 4: cfcli remaining text parsers (Apps, Services, Routes, Domains, Buildpacks)

**Files:**
- Modify: `internal/cfcli/parse.go`

This task adds all remaining column-offset text parsers. CF CLI outputs column-aligned text. The column-offset approach: find header line by matching known column names, determine character offsets for each column, extract fields from data lines using those offsets. The last column captures everything from its start position to end-of-line (handles multi-word values like comma-separated routes).

Note: `findColumns` and `extractField` helpers were already created in Task 3 Step 2. This task appends only the remaining parser functions.

- [ ] **Step 1: Add `parseCFApps` parser**

The CF `apps` output looks like:
```
name            requested state   processes   routes
my-app          started           web:1/1     my-app.cfapps.us10.hana.ondemand.com
```

Note: CF CLI v8 outputs `processes` (not `memory`). The spec says `Memory` but the actual column is `processes`, so we map it to `Instances` in the struct. The JSON key is `"instances"` for consistency with what the CLI shows.

```go
type App struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	Instances string `json:"instances"`
	Routes    string `json:"routes"`
}

func parseCFApps(output string) []App {
	lines := strings.Split(output, "\n")
	headerIdx := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "name") && strings.Contains(lower, "requested state") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil
	}

	cols := findColumns(lines[headerIdx], []string{"name", "requested state", "processes", "routes"})
	var apps []App
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		apps = append(apps, App{
			Name:      extractField(line, cols[0], cols[1]),
			State:     extractField(line, cols[1], cols[2]),
			Instances: extractField(line, cols[2], cols[3]),
			Routes:    extractField(line, cols[3], -1),
		})
	}
	return apps
}
```

- [ ] **Step 2: Add `parseCFServices` parser**

The CF `services` output looks like:
```
name             offering    plan     bound apps   last operation     broker
my-hdi           hana        hdi-shared   my-app   create succeeded   sm-hana-broker
```

```go
type Service struct {
	Name      string `json:"name"`
	Service   string `json:"service"`
	Plan      string `json:"plan"`
	BoundApps string `json:"bound_apps"`
	Status    string `json:"status"`
}

func parseCFServices(output string) []Service {
	lines := strings.Split(output, "\n")
	headerIdx := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "name") && (strings.Contains(lower, "offering") || strings.Contains(lower, "service")) && strings.Contains(lower, "plan") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil
	}

	header := lines[headerIdx]
	lowerHeader := strings.ToLower(header)

	nameCol := strings.Index(lowerHeader, "name")
	offeringCol := -1
	if idx := strings.Index(lowerHeader, "offering"); idx >= 0 {
		offeringCol = idx
	} else {
		offeringCol = strings.Index(lowerHeader, "service")
	}
	planCol := strings.Index(lowerHeader, "plan")
	boundCol := strings.Index(lowerHeader, "bound apps")
	lastOpCol := strings.Index(lowerHeader, "last operation")

	var services []Service
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		services = append(services, Service{
			Name:      extractField(line, nameCol, offeringCol),
			Service:   extractField(line, offeringCol, planCol),
			Plan:      extractField(line, planCol, boundCol),
			BoundApps: extractField(line, boundCol, lastOpCol),
			Status:    extractField(line, lastOpCol, -1),
		})
	}
	return services
}
```

- [ ] **Step 3: Add `parseCFRoutes` parser**

```go
type Route struct {
	Domain string `json:"domain"`
	Host   string `json:"host"`
	Path   string `json:"path"`
	Apps   string `json:"apps"`
}

func parseCFRoutes(output string) []Route {
	lines := strings.Split(output, "\n")
	headerIdx := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "space") && strings.Contains(lower, "host") && strings.Contains(lower, "domain") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil
	}

	header := lines[headerIdx]
	lowerHeader := strings.ToLower(header)

	hostCol := strings.Index(lowerHeader, "host")
	domainCol := strings.Index(lowerHeader, "domain")
	pathCol := strings.Index(lowerHeader, "path")
	appsCol := -1
	if idx := strings.Index(lowerHeader, "destination"); idx >= 0 {
		appsCol = idx
	} else if idx := strings.Index(lowerHeader, "apps"); idx >= 0 {
		appsCol = idx
	}

	var routes []Route
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		routes = append(routes, Route{
			Host:   extractField(line, hostCol, domainCol),
			Domain: extractField(line, domainCol, pathCol),
			Path:   extractField(line, pathCol, appsCol),
			Apps:   extractField(line, appsCol, -1),
		})
	}
	return routes
}
```

- [ ] **Step 4: Add `parseCFDomains` and `parseCFBuildpacks` parsers**

```go
type Domain struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

func parseCFDomains(output string) []Domain {
	lines := strings.Split(output, "\n")
	headerIdx := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "name") && (strings.Contains(lower, "type") || strings.Contains(lower, "status")) {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil
	}

	header := lines[headerIdx]
	lowerHeader := strings.ToLower(header)
	nameCol := strings.Index(lowerHeader, "name")
	typeCol := -1
	if idx := strings.Index(lowerHeader, "type"); idx >= 0 {
		typeCol = idx
	}
	statusCol := strings.Index(lowerHeader, "status")

	var domains []Domain
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		d := Domain{Name: extractField(line, nameCol, typeCol)}
		if typeCol >= 0 && statusCol >= 0 {
			d.Type = extractField(line, typeCol, statusCol)
			d.Status = extractField(line, statusCol, -1)
		} else if typeCol >= 0 {
			d.Type = extractField(line, typeCol, -1)
		}
		domains = append(domains, d)
	}
	return domains
}

type Buildpack struct {
	Name     string `json:"name"`
	Position string `json:"position"`
	Enabled  string `json:"enabled"`
	Locked   string `json:"locked"`
	Filename string `json:"filename"`
}

func parseCFBuildpacks(output string) []Buildpack {
	lines := strings.Split(output, "\n")
	headerIdx := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "position") && strings.Contains(lower, "name") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil
	}

	header := lines[headerIdx]
	lowerHeader := strings.ToLower(header)
	posCol := strings.Index(lowerHeader, "position")
	nameCol := strings.Index(lowerHeader, "name")
	enabledCol := strings.Index(lowerHeader, "enabled")
	lockedCol := strings.Index(lowerHeader, "locked")
	filenameCol := strings.Index(lowerHeader, "filename")

	var bps []Buildpack
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		bps = append(bps, Buildpack{
			Position: extractField(line, posCol, nameCol),
			Name:     extractField(line, nameCol, enabledCol),
			Enabled:  extractField(line, enabledCol, lockedCol),
			Locked:   extractField(line, lockedCol, filenameCol),
			Filename: extractField(line, filenameCol, -1),
		})
	}
	return bps
}
```

- [ ] **Step 5: Verify compilation**

Run: `go build ./internal/cfcli/...`
Expected: BUILD SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/cfcli/parse.go
git commit -m "feat(cfcli): add column-offset text parsers for all CF CLI commands"
```

---

### Task 5: cfcli remaining commands (Apps through Buildpacks)

**Files:**
- Modify: `internal/cfcli/commands.go`

- [ ] **Step 1: Add Apps method**

Append to `commands.go`:

```go
func (c *Client) Apps(ctx context.Context) ([]App, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf apps")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("cf apps failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}
	return parseCFApps(out), nil
}
```

- [ ] **Step 2: Add Services method**

```go
func (c *Client) Services(ctx context.Context) ([]Service, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf services")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("cf services failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}
	return parseCFServices(out), nil
}
```

- [ ] **Step 3: Add Env method with credential redaction**

Add `AppEnv` struct and `Env` method to `commands.go`:

```go
type AppEnv struct {
	SystemProvided any `json:"system_provided,omitempty"`
	UserProvided   any `json:"user_provided,omitempty"`
	Running        any `json:"running_env,omitempty"`
	Staging        any `json:"staging_env,omitempty"`
}

func (c *Client) Env(ctx context.Context, appName string) (AppEnv, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf env "+appName)
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return AppEnv{}, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return AppEnv{}, authErr
		}
		return AppEnv{}, fmt.Errorf("cf env failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return AppEnv{}, authErr
	}
	return parseCFEnv(out), nil
}
```

Then add `parseCFEnv`, `redactSensitive`, and `sensitiveKeyRe` to **`parse.go`** (not commands.go — keep the regex near its usage). Add `"encoding/json"` and `"regexp"` to parse.go's import block:

```go
var sensitiveKeyRe = regexp.MustCompile(`(?i)^(password|clientsecret|client_secret|token|access_token|refresh_token|key|secret|private_key|certificate)$`)

func parseCFEnv(output string) AppEnv {
	var env AppEnv

	sections := map[string]*any{
		"System-Provided:": &env.SystemProvided,
		"User-Provided:":   &env.UserProvided,
		"Running Environment Variable Groups:": &env.Running,
		"Staging Environment Variable Groups:":  &env.Staging,
	}

	lines := strings.Split(output, "\n")
	for sectionName, target := range sections {
		content := extractJSONSection(lines, sectionName)
		if content == "" {
			continue
		}
		var parsed any
		if json.Unmarshal([]byte(content), &parsed) == nil {
			parsed = redactSensitive(parsed)
			*target = parsed
		}
	}
	return env
}

func extractJSONSection(lines []string, marker string) string {
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == marker || strings.Contains(line, marker) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	var buf strings.Builder
	braceDepth := 0
	started := false
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if !started {
			if strings.HasPrefix(trimmed, "{") {
				started = true
			} else {
				continue
			}
		}
		buf.WriteString(line)
		buf.WriteString("\n")
		braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
		if started && braceDepth <= 0 {
			break
		}
	}
	return buf.String()
}

func redactSensitive(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v := range val {
			if sensitiveKeyRe.MatchString(k) {
				out[k] = "[REDACTED]"
			} else {
				out[k] = redactSensitive(v)
			}
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, v := range val {
			out[i] = redactSensitive(v)
		}
		return out
	default:
		return v
	}
}
```

- [ ] **Step 4: Add Routes, Domains, and Buildpacks methods**

Append to `commands.go`:

```go
func (c *Client) Routes(ctx context.Context) ([]Route, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf routes")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("cf routes failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}
	return parseCFRoutes(out), nil
}

func (c *Client) Domains(ctx context.Context) ([]Domain, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf domains")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("cf domains failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}
	return parseCFDomains(out), nil
}

func (c *Client) Buildpacks(ctx context.Context) ([]Buildpack, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf buildpacks")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("cf buildpacks failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}
	return parseCFBuildpacks(out), nil
}
```

- [ ] **Step 5: Verify compilation**

Run: `go build ./internal/cfcli/...`
Expected: BUILD SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/cfcli/commands.go internal/cfcli/parse.go
git commit -m "feat(cfcli): add Apps, Services, Env, Routes, Domains, Buildpacks methods with credential redaction"
```

---

### Task 6: btpcli auth types

**Files:**
- Create: `internal/btpcli/auth.go`

- [ ] **Step 1: Create `internal/btpcli/auth.go` with error types and detection helpers**

```go
package btpcli

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type AuthError struct {
	CLI     string
	Message string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("%s: %s", e.CLI, e.Message)
}

type NotInstalledError struct {
	CLI     string
	Message string
}

func (e *NotInstalledError) Error() string {
	return fmt.Sprintf("%s: %s", e.CLI, e.Message)
}

func checkAuthError(output string) *AuthError {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "login required"):
		return &AuthError{CLI: "btp", Message: "Login required. Run 'btp login' to authenticate."}
	case strings.Contains(lower, "you are not logged in"):
		return &AuthError{CLI: "btp", Message: "Not logged in. Run 'btp login' to authenticate."}
	case strings.Contains(lower, "session has expired"):
		return &AuthError{CLI: "btp", Message: "Session expired. Run 'btp login' to re-authenticate."}
	}
	return nil
}

func checkNotInstalled(err error) *NotInstalledError {
	if err != nil && errors.Is(err, exec.ErrNotFound) {
		return &NotInstalledError{CLI: "btp", Message: "BTP CLI is not installed."}
	}
	return nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/btpcli/...`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/btpcli/auth.go
git commit -m "feat(btpcli): add auth and not-installed error types with detection helpers"
```

---

### Task 7: btpcli client and config

**Files:**
- Create: `internal/btpcli/client.go`

- [ ] **Step 1: Create `internal/btpcli/client.go` with Client, Runner, config structs, and config reading**

```go
package btpcli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Runner func(command string) (string, error)

type Client struct {
	run        Runner
	configPath string
	timeout    time.Duration
}

func NewClient(run Runner, configPath string) *Client {
	return &Client{
		run:        run,
		configPath: configPath,
		timeout:    10 * time.Second,
	}
}

type btpConfig struct {
	TargetHierarchy struct {
		GlobalAccountSubdomain string `json:"GlobalAccountSubdomain"`
		SubaccountSubdomain    string `json:"SubaccountSubdomain"`
	} `json:"TargetHierarchy"`
	CLIServerURL string `json:"CLIServerURL"`
}

func (c *Client) readConfig() *btpConfig {
	if c.configPath == "" {
		return nil
	}
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return nil
	}
	var cfg btpConfig
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	return &cfg
}

func (c *Client) runWithContext(ctx context.Context, command string) (string, error) {
	type result struct {
		out string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := c.run(command)
		ch <- result{out, err}
	}()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("command timed out after %s — the BTP API may be slow, try again", c.timeout)
	case r := <-ch:
		return r.out, r.err
	}
}
```

Note: The `btpConfig` struct has **identical field names and JSON tags** as `internal/project/detect.go:301-307`. Capital-initial JSON keys match the actual BTP config file format.

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/btpcli/...`
Expected: BUILD SUCCESS

- [ ] **Step 3: Commit**

```bash
git add internal/btpcli/client.go
git commit -m "feat(btpcli): add Client struct, Runner type, config reading with BTP config struct"
```

---

### Task 8: btpcli commands (Target, Subaccounts, ServiceInstances, RoleCollections)

**Files:**
- Create: `internal/btpcli/commands.go`

- [ ] **Step 1: Create `internal/btpcli/commands.go` with Target method**

```go
package btpcli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var reBTPRegion = regexp.MustCompile(`^([a-z]{2}\d{2})`)

type TargetInfo struct {
	Subaccount    string `json:"subaccount"`
	GlobalAccount string `json:"global_account"`
	Region        string `json:"region"`
	Trial         bool   `json:"trial"`
	LoggedIn      bool   `json:"logged_in"`
}

func (c *Client) Target(ctx context.Context) (TargetInfo, error) {
	cfg := c.readConfig()
	if cfg != nil && cfg.TargetHierarchy.SubaccountSubdomain != "" {
		subdomain := cfg.TargetHierarchy.SubaccountSubdomain
		region := ""
		if m := reBTPRegion.FindStringSubmatch(subdomain); len(m) >= 2 {
			region = m[1]
		}
		return TargetInfo{
			Subaccount:    subdomain,
			GlobalAccount: cfg.TargetHierarchy.GlobalAccountSubdomain,
			Region:        region,
			Trial:         strings.Contains(strings.ToLower(subdomain), "trial"),
			LoggedIn:      true,
		}, nil
	}

	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "btp --format json target")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return TargetInfo{}, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return TargetInfo{}, authErr
		}
		return TargetInfo{}, fmt.Errorf("btp target failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return TargetInfo{}, authErr
	}

	var result struct {
		SubAccount struct {
			Subdomain string `json:"subdomain"`
		} `json:"subAccount"`
		GlobalAccount struct {
			Subdomain string `json:"subdomain"`
		} `json:"globalAccount"`
	}
	if json.Unmarshal([]byte(out), &result) != nil {
		return TargetInfo{}, fmt.Errorf("failed to parse btp target output")
	}

	subdomain := result.SubAccount.Subdomain
	region := ""
	if m := reBTPRegion.FindStringSubmatch(subdomain); len(m) >= 2 {
		region = m[1]
	}
	return TargetInfo{
		Subaccount:    subdomain,
		GlobalAccount: result.GlobalAccount.Subdomain,
		Region:        region,
		Trial:         strings.Contains(strings.ToLower(subdomain), "trial"),
		LoggedIn:      subdomain != "",
	}, nil
}
```

Note: Region regex `^([a-z]{2}\d{2})` matches `internal/project/detect.go:220`.

- [ ] **Step 2: Add Subaccounts method**

```go
type Subaccount struct {
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	Region    string `json:"region"`
	State     string `json:"state"`
	Parent    string `json:"parent"`
}

func (c *Client) Subaccounts(ctx context.Context) ([]Subaccount, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "btp --format json list accounts/subaccount")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("btp list subaccounts failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}

	var raw struct {
		Value []struct {
			DisplayName       string `json:"displayName"`
			Subdomain         string `json:"subdomain"`
			Region            string `json:"region"`
			State             string `json:"state"`
			ParentDisplayName string `json:"parentDisplayName"`
		} `json:"value"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse btp subaccounts output: %w", err)
	}

	subs := make([]Subaccount, 0, len(raw.Value))
	for _, v := range raw.Value {
		subs = append(subs, Subaccount{
			Name:      v.DisplayName,
			Subdomain: v.Subdomain,
			Region:    v.Region,
			State:     v.State,
			Parent:    v.ParentDisplayName,
		})
	}
	return subs, nil
}
```

- [ ] **Step 3: Add ServiceInstances method**

```go
type ServiceInstance struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	Plan    string `json:"plan"`
	Status  string `json:"status"`
	Created string `json:"created"`
}

func (c *Client) ServiceInstances(ctx context.Context) ([]ServiceInstance, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "btp --format json list services/instance")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("btp list service instances failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}

	var raw []struct {
		Name            string `json:"name"`
		ServicePlanName string `json:"service_plan_name"`
		ServiceName     string `json:"service_name"`
		Ready           bool   `json:"ready"`
		CreatedAt       string `json:"created_at"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse btp service instances output: %w", err)
	}

	instances := make([]ServiceInstance, 0, len(raw))
	for _, v := range raw {
		status := "not ready"
		if v.Ready {
			status = "ready"
		}
		instances = append(instances, ServiceInstance{
			Name:    v.Name,
			Service: v.ServiceName,
			Plan:    v.ServicePlanName,
			Status:  status,
			Created: v.CreatedAt,
		})
	}
	return instances, nil
}
```

- [ ] **Step 4: Add RoleCollections method**

```go
type RoleCollection struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RoleCount   int    `json:"role_count"`
}

func (c *Client) RoleCollections(ctx context.Context) ([]RoleCollection, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "btp --format json list security/role-collection")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("btp list role collections failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}

	var raw []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		RoleRefs    []any  `json:"roleReferences"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse btp role collections output: %w", err)
	}

	rcs := make([]RoleCollection, 0, len(raw))
	for _, v := range raw {
		rcs = append(rcs, RoleCollection{
			Name:        v.Name,
			Description: v.Description,
			RoleCount:   len(v.RoleRefs),
		})
	}
	return rcs, nil
}
```

- [ ] **Step 5: Verify compilation**

Run: `go build ./internal/btpcli/...`
Expected: BUILD SUCCESS

- [ ] **Step 6: Commit**

```bash
git add internal/btpcli/commands.go
git commit -m "feat(btpcli): add Target, Subaccounts, ServiceInstances, RoleCollections methods"
```

---

### Task 9: MCP tool handlers — tools_cf.go

**Files:**
- Create: `internal/mcpserver/tools_cf.go`

- [ ] **Step 1: Create `internal/mcpserver/tools_cf.go` with `registerCFTools` and error enrichment helpers**

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/cfcli"
)

func registerCFTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("cf_target",
			mcp.WithDescription("Get current CF target (org, space, API endpoint, region, login status). Use to check where the user's CF CLI is pointed before running other cf_ tools."),
		),
		cfTargetHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_apps",
			mcp.WithDescription("List deployed Cloud Foundry apps with state, instances, memory, and routes."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		cfAppsHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_services",
			mcp.WithDescription("List Cloud Foundry service instances with plan, bound apps, and status."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		cfServicesHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_env",
			mcp.WithDescription("Get environment variables for a Cloud Foundry app (credentials redacted). Shows system-provided services, user-provided variables, and running/staging env groups."),
			mcp.WithString("app",
				mcp.Required(),
				mcp.Description("Name of the Cloud Foundry application"),
			),
		),
		cfEnvHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_routes",
			mcp.WithDescription("List Cloud Foundry routes with domain, host, path, and bound apps."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		cfRoutesHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_domains",
			mcp.WithDescription("List Cloud Foundry domains with type (shared/private) and status."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		cfDomainsHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_buildpacks",
			mcp.WithDescription("List Cloud Foundry buildpacks with position, enabled status, and filename."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		cfBuildpacksHandler(deps),
	)
}
```

- [ ] **Step 2: Add CF error enrichment helpers**

Append to `tools_cf.go`:

```go
func handleCFError(err error, deps Deps) *mcp.CallToolResult {
	switch e := err.(type) {
	case *cfcli.AuthError:
		return cfAuthErrorResult(e, deps)
	case *cfcli.NotInstalledError:
		return cfNotInstalledResult(deps)
	default:
		return mcp.NewToolResultError(err.Error())
	}
}

func cfAuthErrorResult(err *cfcli.AuthError, deps Deps) *mcp.CallToolResult {
	fix := "Run: cf login"
	// Read API endpoint directly from config file — do NOT call Target() here,
	// as we may already be in a Target() error path (would be recursive and wasteful).
	if deps.CFConfigPath != "" {
		data, readErr := os.ReadFile(deps.CFConfigPath)
		if readErr == nil {
			var cfg struct {
				Target string `json:"Target"`
			}
			if json.Unmarshal(data, &cfg) == nil && cfg.Target != "" {
				fix = fmt.Sprintf("Run: cf login -a %s", cfg.Target)
			}
		}
	}
	result := map[string]string{
		"error":   "not_authenticated",
		"cli":     "cf",
		"message": err.Message,
		"fix":     fix,
		"hint":    "The cf CLI requires an active login session. After logging in, retry the command.",
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b))
}

func cfNotInstalledResult(deps Deps) *mcp.CallToolResult {
	install := ""
	for _, p := range deps.Packs {
		for _, t := range p.Tools {
			if t.ID == "cf-cli" {
				install = installForCurrentOS(t.Install)
				break
			}
		}
		if install != "" {
			break
		}
	}
	result := map[string]string{
		"error":   "cli_not_installed",
		"cli":     "cf",
		"message": "Cloud Foundry CLI is not installed.",
		"fix":     fmt.Sprintf("Install: %s", install),
		"hint":    "The cf CLI is required for Cloud Foundry operations. Install it and run 'cf login' to authenticate.",
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b))
}
```

Note: `installForCurrentOS` is already defined in `tools_doctor.go` (line 63) in the same `mcpserver` package.

- [ ] **Step 3: Add handler functions for all 7 CF tools**

Append to `tools_cf.go`:

```go
func cfTargetHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		info, err := deps.CFClient.Target(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		b, _ := json.Marshal(info)
		return mcp.NewToolResultText(string(b)), nil
	}
}

func cfAppsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		apps, err := deps.CFClient.Apps(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(apps)
		if limit < total {
			apps = apps[:limit]
		}
		return wrapResults(apps, total, len(apps), "apps", ""), nil
	}
}

func cfServicesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		services, err := deps.CFClient.Services(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(services)
		if limit < total {
			services = services[:limit]
		}
		return wrapResults(services, total, len(services), "services", ""), nil
	}
}

func cfEnvHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		appName, err := req.RequireString("app")
		if err != nil {
			return mcp.NewToolResultError("app parameter is required"), nil
		}
		env, err := deps.CFClient.Env(ctx, appName)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		b, _ := json.Marshal(env)
		return mcp.NewToolResultText(string(b)), nil
	}
}

func cfRoutesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		routes, err := deps.CFClient.Routes(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(routes)
		if limit < total {
			routes = routes[:limit]
		}
		return wrapResults(routes, total, len(routes), "routes", ""), nil
	}
}

func cfDomainsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		domains, err := deps.CFClient.Domains(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(domains)
		if limit < total {
			domains = domains[:limit]
		}
		return wrapResults(domains, total, len(domains), "domains", ""), nil
	}
}

func cfBuildpacksHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		bps, err := deps.CFClient.Buildpacks(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(bps)
		if limit < total {
			bps = bps[:limit]
		}
		return wrapResults(bps, total, len(bps), "buildpacks", ""), nil
	}
}
```

- [ ] **Step 4: Verify syntax**

This file references `Deps.CFClient` and `Deps.CFConfigPath` which are added in Task 11. It won't compile standalone — verify syntax visually. Full compilation is verified after Task 11.

- [ ] **Step 5: Commit**

```bash
git add internal/mcpserver/tools_cf.go
git commit -m "feat(mcp): add 7 CF tool handlers with auth/install error enrichment"
```

---

### Task 10: MCP tool handlers — tools_btp.go

**Files:**
- Create: `internal/mcpserver/tools_btp.go`

- [ ] **Step 1: Create `internal/mcpserver/tools_btp.go` with `registerBTPTools` and error enrichment**

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/btpcli"
)

func registerBTPTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("btp_target",
			mcp.WithDescription("Get current BTP target (subaccount, region, global account, trial flag, login status). Use to check the user's BTP CLI state before running other btp_ tools."),
		),
		btpTargetHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("btp_subaccounts",
			mcp.WithDescription("List BTP subaccounts with name, subdomain, region, state, and parent directory."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		btpSubaccountsHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("btp_service_instances",
			mcp.WithDescription("List BTP service instances with name, service, plan, status, and creation date."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		btpServiceInstancesHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("btp_role_collections",
			mcp.WithDescription("List BTP role collections with name, description, and role count."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		btpRoleCollectionsHandler(deps),
	)
}

func handleBTPError(err error, deps Deps) *mcp.CallToolResult {
	switch e := err.(type) {
	case *btpcli.AuthError:
		return btpAuthErrorResult(e)
	case *btpcli.NotInstalledError:
		return btpNotInstalledResult(deps)
	default:
		return mcp.NewToolResultError(err.Error())
	}
}

func btpAuthErrorResult(err *btpcli.AuthError) *mcp.CallToolResult {
	result := map[string]string{
		"error":   "not_authenticated",
		"cli":     "btp",
		"message": err.Message,
		"fix":     "Run: btp login",
		"hint":    "The btp CLI requires an active login session. After logging in, retry the command.",
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b))
}

func btpNotInstalledResult(deps Deps) *mcp.CallToolResult {
	install := ""
	for _, p := range deps.Packs {
		for _, t := range p.Tools {
			if t.ID == "btp-cli" {
				install = installForCurrentOS(t.Install)
				break
			}
		}
		if install != "" {
			break
		}
	}
	result := map[string]string{
		"error":   "cli_not_installed",
		"cli":     "btp",
		"message": "BTP CLI is not installed.",
		"fix":     fmt.Sprintf("Install: %s", install),
		"hint":    "The btp CLI is required for BTP operations. Install it and run 'btp login' to authenticate.",
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b))
}

func btpTargetHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.BTPClient == nil {
			return btpNotInstalledResult(deps), nil
		}
		info, err := deps.BTPClient.Target(ctx)
		if err != nil {
			return handleBTPError(err, deps), nil
		}
		b, _ := json.Marshal(info)
		return mcp.NewToolResultText(string(b)), nil
	}
}

func btpSubaccountsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.BTPClient == nil {
			return btpNotInstalledResult(deps), nil
		}
		subs, err := deps.BTPClient.Subaccounts(ctx)
		if err != nil {
			return handleBTPError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(subs)
		if limit < total {
			subs = subs[:limit]
		}
		return wrapResults(subs, total, len(subs), "subaccounts", ""), nil
	}
}

func btpServiceInstancesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.BTPClient == nil {
			return btpNotInstalledResult(deps), nil
		}
		instances, err := deps.BTPClient.ServiceInstances(ctx)
		if err != nil {
			return handleBTPError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(instances)
		if limit < total {
			instances = instances[:limit]
		}
		return wrapResults(instances, total, len(instances), "service instances", ""), nil
	}
}

func btpRoleCollectionsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.BTPClient == nil {
			return btpNotInstalledResult(deps), nil
		}
		rcs, err := deps.BTPClient.RoleCollections(ctx)
		if err != nil {
			return handleBTPError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(rcs)
		if limit < total {
			rcs = rcs[:limit]
		}
		return wrapResults(rcs, total, len(rcs), "role collections", ""), nil
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/mcpserver/tools_btp.go
git commit -m "feat(mcp): add 4 BTP tool handlers with auth/install error enrichment"
```

---

### Task 11: Wire Deps, server.go, and mcp_serve.go

Tasks 11 and 12 from the original plan are merged into one task so every commit compiles. This task wires: Deps struct changes, register calls, server instructions, cliRunner closure, exec.LookPath detection, config path resolution, and client construction.

**Files:**
- Modify: `internal/mcpserver/server.go`
- Modify: `cmd/mcp_serve.go`

- [ ] **Step 1: Add CFClient, BTPClient, and CFConfigPath to Deps struct in server.go**

In `internal/mcpserver/server.go`, add to the `Deps` struct (after `Cwd string`):

```go
CFClient     *cfcli.Client
BTPClient    *btpcli.Client
CFConfigPath string
```

Add imports:

```go
"github.com/SAP-samples/sap-devs-cli/internal/cfcli"
"github.com/SAP-samples/sap-devs-cli/internal/btpcli"
```

Both client fields are pointer types — `nil` when the CLI is not detected at startup. `CFConfigPath` is passed through so MCP handlers can read the config file directly for error enrichment without calling Target() recursively.

- [ ] **Step 2: Add register calls in NewServer**

After the existing `registerDiscoveryTools(s, deps)` line, add:

```go
registerCFTools(s, deps)
registerBTPTools(s, deps)
```

- [ ] **Step 3: Expand server instructions string**

Append to the existing `server.WithInstructions(...)` string (before the closing `"`):

```
 Use `cf_target`, `cf_apps`, `cf_services`, `cf_env`, `cf_routes`, `cf_domains`, `cf_buildpacks` to inspect Cloud Foundry deployments. Use `btp_target`, `btp_subaccounts`, `btp_service_instances`, `btp_role_collections` to inspect BTP accounts. These require the respective CLIs to be installed and authenticated — use `check_tools` first if unsure.
```

- [ ] **Step 4: Add imports to mcp_serve.go**

Add to import block in `cmd/mcp_serve.go`:

```go
"os/exec"
"path/filepath"
"runtime"
"strings"

"github.com/SAP-samples/sap-devs-cli/internal/cfcli"
"github.com/SAP-samples/sap-devs-cli/internal/btpcli"
```

- [ ] **Step 5: Add config path resolution functions to mcp_serve.go**

Add as file-level functions:

```go
func resolveCFConfigPath() string {
	cfHome := os.Getenv("CF_HOME")
	if cfHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		cfHome = home
	}
	return filepath.Join(cfHome, ".cf", "config.json")
}

func resolveBTPConfigPath() string {
	path := os.Getenv("BTP_CLIENTCONFIG")
	if path != "" {
		return path
	}
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			return filepath.Join(appdata, "SAP", "btp", "config.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	primary := filepath.Join(home, ".config", "btp", "config.json")
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	return filepath.Join(home, ".config", ".btp", "config.json")
}
```

Note: `resolveBTPConfigPath` replicates the fallback logic from `internal/project/detect.go:371-388`, including the `.btp` fallback for older BTP CLI v1.x installations.

- [ ] **Step 6: Add cliRunner and client construction to RunE body**

After `learningIndex, _ := learning.LoadIndex(...)` and before `deps := mcpserver.Deps{`, add:

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

cfConfigPath := resolveCFConfigPath()
var cfClient *cfcli.Client
if _, err := exec.LookPath("cf"); err == nil {
	cfClient = cfcli.NewClient(cliRunner, cfConfigPath)
}

var btpClient *btpcli.Client
if _, err := exec.LookPath("btp"); err == nil {
	btpClient = btpcli.NewClient(cliRunner, resolveBTPConfigPath())
}
```

Note: `exec.LookPath` only checks PATH — no subprocess execution, no startup latency. The `cliRunner` intentionally has no baked-in timeout (unlike `execRunner` in `tools_doctor.go:51` which uses a 5s timeout). Here, each Client method owns its own deadline via `context.WithTimeout`.

- [ ] **Step 7: Add CFClient/BTPClient/CFConfigPath to Deps initialization**

Update the `deps := mcpserver.Deps{...}` to include:

```go
CFClient:     cfClient,
BTPClient:    btpClient,
CFConfigPath: cfConfigPath,
```

- [ ] **Step 8: Verify full project compilation**

Run: `go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 9: Commit**

```bash
git add internal/mcpserver/server.go cmd/mcp_serve.go
git commit -m "feat(mcp): wire Deps, register CF/BTP tools, add cliRunner and exec.LookPath detection"
```

---

### Task 12: Verify full build

**Files:** None (verification only)

- [ ] **Step 1: Clean build**

Run: `go build ./...`
Expected: BUILD SUCCESS with no errors

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 3: Verify binary starts**

Run: `go build -o sap-devs.exe . && echo "OK"`
Expected: Binary built successfully

---

### Task 13: Update documentation

**Files:**
- Modify: `docs/mcp-server.md`

- [ ] **Step 1: Update tool count in mcp-server.md**

Change "fifteen tools" to "twenty-six tools" in the introduction paragraph (line 3).

Update the line: `The server loads the same content layer...and serves it through fifteen tools.`
To: `The server loads the same content layer...and serves it through twenty-six tools.`

- [ ] **Step 2: Add CF tools table**

After the Doctor tools section and before Discovery tools, add a new section:

```markdown
### Cloud Foundry tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `cf_target` | Get current CF target (org, space, API endpoint, region, login status) | — |
| `cf_apps` | List deployed apps with state, instances, memory, and routes | `limit` (optional, default 20, max 100) |
| `cf_services` | List service instances with plan, bound apps, and status | `limit` (optional, default 20, max 100) |
| `cf_env` | Get environment variables for an app (credentials redacted) | `app` (required) |
| `cf_routes` | List routes with domain, host, path, and bound apps | `limit` (optional, default 20, max 100) |
| `cf_domains` | List domains with type (shared/private) and status | `limit` (optional, default 20, max 100) |
| `cf_buildpacks` | List buildpacks with position, enabled status, and filename | `limit` (optional, default 20, max 100) |
```

- [ ] **Step 3: Add BTP tools table**

After CF tools section:

```markdown
### BTP tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `btp_target` | Get current BTP target (subaccount, region, global account, trial flag, login status) | — |
| `btp_subaccounts` | List subaccounts with name, region, state, and parent directory | `limit` (optional, default 20, max 100) |
| `btp_service_instances` | List BTP service instances with name, plan, and status | `limit` (optional, default 20, max 100) |
| `btp_role_collections` | List role collections with name, description, and role count | `limit` (optional, default 20, max 100) |
```

- [ ] **Step 4: Update server instructions quote**

Update the quoted instructions block to include the CF/BTP tools clause.

- [ ] **Step 5: Update the Architecture table**

Add `tools_cf.go` and `tools_btp.go` to the file table.

- [ ] **Step 6: Update triggers table**

Add rows for CF/BTP interactions:
```markdown
| Asks about Cloud Foundry apps or services | `cf_target`, `cf_apps`, `cf_services` | Inspects live CF deployment state via CLI |
| Asks about BTP subaccounts or services | `btp_target`, `btp_subaccounts`, `btp_service_instances` | Inspects live BTP account state via CLI |
```

- [ ] **Step 7: Commit**

```bash
git add docs/mcp-server.md
git commit -m "docs: update mcp-server.md with 11 new CF/BTP tools (26 total)"
```

---

### Task 14: Update CLAUDE.md and TODO.md

**Files:**
- Modify: `CLAUDE.md`
- Modify: `TODO.md`

- [ ] **Step 1: Update CLAUDE.md MCP server section**

In the CLI Commands table, update the `mcp` row description to mention 26 tools and include the CF/BTP tool names.

Update: `(15 tools: list_packs, get_context, ...)`
To: `(26 tools: list_packs, get_context, get_tip, search_resources, get_known_errors, get_recent_news, get_news_detail, search_tutorials, search_learning_journeys, get_samples, check_tools, check_project, search_events, search_videos, search_discovery, cf_target, cf_apps, cf_services, cf_env, cf_routes, cf_domains, cf_buildpacks, btp_target, btp_subaccounts, btp_service_instances, btp_role_collections)`

- [ ] **Step 2: Update TODO.md**

Mark the MCP CLI wrappers item as done.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md TODO.md
git commit -m "docs: update CLAUDE.md with 26 MCP tools, mark CLI wrappers done in TODO"
```
