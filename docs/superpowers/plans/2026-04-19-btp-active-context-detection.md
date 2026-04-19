# BTP Active Context Detection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Detect the developer's active BTP subaccount/region and Cloud Foundry org/space at inject time and render them as a `**BTP Environment (detected):**` section in the injected AI context.

**Architecture:** Add `detectBTP()` and `detectCF()` alongside existing detectors in `internal/project/detect.go`. Each reads a local config file first (fast, no subprocess) and falls back to CLI exec with a 3-second timeout. BTP/CF facts flow through the existing `ProjectContext` → `DynamicContext` → `renderDynamic()` pipeline, rendered under a separate heading from project facts. Trial accounts are heuristically flagged.

**Tech Stack:** Go standard library (`os`, `encoding/json`, `os/exec`, `regexp`, `runtime`, `context`), existing `internal/project`, `internal/content`, `internal/dynamic` packages. Tests use `testing` and `github.com/stretchr/testify`.

**Spec:** `docs/superpowers/specs/2026-04-19-btp-active-context-detection-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/project/detect.go` | Add 6 new fields to `ProjectContext`, `detectBTP()`, `detectCF()`, `HasBTPContext()`, region extraction helpers, update `buildFacts()` |
| `internal/project/detect_test.go` | Unit tests for BTP/CF config parsing, region extraction, trial detection, `HasBTPContext()`, `buildFacts()` with BTP/CF fields |
| `internal/content/dynamic.go` | Add `BTPFacts []ProjectFact` to `ProjectInfo` |
| `internal/content/render.go` | Add `**BTP Environment (detected):**` block in `renderDynamic()` |
| `internal/content/render_dynamic_test.go` | Tests for BTP rendering in dynamic context |
| `internal/dynamic/gather.go` | Update condition to create `ProjectInfo` when BTP context exists; split facts into `Facts` and `BTPFacts` |
| `internal/dynamic/gather_test.go` | Tests for BTP/CF context flowing through gather |

---

### Task 1: Add CF Config Parsing and Region Extraction

**Files:**
- Modify: `internal/project/detect.go` (add `CFOrg`, `CFSpace`, `CFRegion` fields, `detectCF()`, `extractCFRegion()`)
- Modify: `internal/project/detect_test.go` (add tests)

CF is implemented first because we have an actual config.json sample and known structure. BTP comes next.

- [ ] **Step 1: Write failing tests for CF config parsing**

Add to `internal/project/detect_test.go`:

```go
func TestDetectCF_ParsesConfigJSON(t *testing.T) {
	// CF_HOME is the PARENT of .cf/ — the cf CLI reads $CF_HOME/.cf/config.json
	dir := t.TempDir()
	cfDir := filepath.Join(dir, ".cf")
	os.Mkdir(cfDir, 0755)
	writeFile(t, cfDir, "config.json", `{
		"Target": "https://api.cf.us10.hana.ondemand.com",
		"OrganizationFields": {"Name": "MyOrg", "GUID": "xxx"},
		"SpaceFields": {"Name": "dev", "GUID": "yyy", "AllowSSH": true}
	}`)

	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("CF_HOME", dir) // parent of .cf/, not the .cf/ dir itself
	detectCF(ctx)

	if ctx.CFOrg != "MyOrg" {
		t.Errorf("CFOrg = %q, want %q", ctx.CFOrg, "MyOrg")
	}
	if ctx.CFSpace != "dev" {
		t.Errorf("CFSpace = %q, want %q", ctx.CFSpace, "dev")
	}
	if ctx.CFRegion != "us10" {
		t.Errorf("CFRegion = %q, want %q", ctx.CFRegion, "us10")
	}
}

func TestDetectCF_SilentOnMissingConfig(t *testing.T) {
	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("CF_HOME", "/nonexistent/path")
	detectCF(ctx)

	if ctx.CFOrg != "" {
		t.Errorf("CFOrg should be empty, got %q", ctx.CFOrg)
	}
}

func TestExtractCFRegion(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://api.cf.us10.hana.ondemand.com", "us10"},
		{"https://api.cf.eu10.hana.ondemand.com", "eu10"},
		{"https://api.cf.us10-001.hana.ondemand.com", "us10-001"},
		{"https://api.cf.ap21.hana.ondemand.com", "ap21"},
		{"https://some.other.url.com", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractCFRegion(tt.url)
		if got != tt.want {
			t.Errorf("extractCFRegion(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/project/...`
Expected: compile error — `detectCF` and `extractCFRegion` not defined

- [ ] **Step 3: Implement CF detection**

Add to `internal/project/detect.go`:

1. Add fields to `ProjectContext`:
```go
CFOrg    string
CFSpace  string
CFRegion string
```

2. Add the regex (package-level):
```go
var reCFRegion = regexp.MustCompile(`api\.cf\.([a-z0-9-]+)\.hana\.ondemand\.com`)
```

3. Add the config struct:
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

4. Add `extractCFRegion`:
```go
func extractCFRegion(target string) string {
	m := reCFRegion.FindStringSubmatch(target)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
```

5. Add `detectCF`:
```go
func detectCF(ctx *ProjectContext) {
	cfg := readCFConfig()
	if cfg == nil {
		return
	}
	ctx.CFOrg = cfg.OrganizationFields.Name
	ctx.CFSpace = cfg.SpaceFields.Name
	ctx.CFRegion = extractCFRegion(cfg.Target)
}

func readCFConfig() *cfConfig {
	cfHome := os.Getenv("CF_HOME")
	if cfHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		cfHome = home
	}
	data, err := os.ReadFile(filepath.Join(cfHome, ".cf", "config.json"))
	if err != nil {
		return nil
	}
	var cfg cfConfig
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	if cfg.OrganizationFields.Name == "" {
		return nil
	}
	return &cfg
}
```

6. Add `regexp` to the import list.

7. Call `detectCF(ctx)` in `Detect()` after `detectDefaultEnv(cwd, ctx)` and before `buildFacts(ctx)`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./internal/project/... && go vet ./internal/project/...`
Expected: builds and vets clean

- [ ] **Step 5: Commit**

```bash
git add internal/project/detect.go internal/project/detect_test.go
git commit -m "feat: add Cloud Foundry context detection via config.json parsing"
```

---

### Task 2: Add BTP Config Parsing and Trial Detection

**Files:**
- Modify: `internal/project/detect.go` (add `BTPSubaccount`, `BTPRegion`, `BTPIsTrial` fields, `detectBTP()`, `extractBTPRegion()`)
- Modify: `internal/project/detect_test.go` (add tests)

- [ ] **Step 1: Write failing tests for BTP config parsing and trial detection**

Add to `internal/project/detect_test.go`:

```go
func TestDetectBTP_ParsesConfigJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.json", `{
		"TargetHierarchy": {
			"GlobalAccountSubdomain": "ga-sub",
			"SubaccountSubdomain": "my-subaccount"
		},
		"CLIServerURL": "https://cli.btp.cloud.sap"
	}`)

	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("BTP_CLIENTCONFIG", filepath.Join(dir, "config.json"))
	detectBTP(ctx)

	if ctx.BTPSubaccount != "my-subaccount" {
		t.Errorf("BTPSubaccount = %q, want %q", ctx.BTPSubaccount, "my-subaccount")
	}
	if ctx.BTPIsTrial {
		t.Error("BTPIsTrial should be false for non-trial subaccount")
	}
}

func TestDetectBTP_DetectsTrialFromSubdomain(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.json", `{
		"TargetHierarchy": {
			"SubaccountSubdomain": "eu10-trial-abc123"
		}
	}`)

	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("BTP_CLIENTCONFIG", filepath.Join(dir, "config.json"))
	detectBTP(ctx)

	if !ctx.BTPIsTrial {
		t.Error("BTPIsTrial should be true when subdomain contains 'trial'")
	}
}

func TestDetectBTP_ExtractsRegionFromSubdomain(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.json", `{
		"TargetHierarchy": {
			"SubaccountSubdomain": "eu10-trial-abc123"
		}
	}`)

	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("BTP_CLIENTCONFIG", filepath.Join(dir, "config.json"))
	detectBTP(ctx)

	if ctx.BTPRegion != "eu10" {
		t.Errorf("BTPRegion = %q, want %q", ctx.BTPRegion, "eu10")
	}
}

func TestDetectBTP_SilentOnMissingConfig(t *testing.T) {
	ctx := &ProjectContext{RawFiles: make(map[string]bool)}
	t.Setenv("BTP_CLIENTCONFIG", "/nonexistent/config.json")
	detectBTP(ctx)

	if ctx.BTPSubaccount != "" {
		t.Errorf("BTPSubaccount should be empty, got %q", ctx.BTPSubaccount)
	}
}

func TestExtractBTPRegion(t *testing.T) {
	tests := []struct {
		subdomain string
		want      string
	}{
		{"eu10-trial-abc123", "eu10"},
		{"us10-mysubaccount", "us10"},
		{"ap21-prod-xyz", "ap21"},
		{"my-custom-subdomain", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractBTPRegion(tt.subdomain)
		if got != tt.want {
			t.Errorf("extractBTPRegion(%q) = %q, want %q", tt.subdomain, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/project/...`
Expected: compile error — `detectBTP` and `extractBTPRegion` not defined

- [ ] **Step 3: Implement BTP detection**

Add to `internal/project/detect.go`:

1. Add fields to `ProjectContext` (insert after existing fields, before `CFOrg`):
```go
BTPSubaccount string
BTPRegion     string
BTPIsTrial    bool
```

2. Add the regex (package-level):
```go
var reBTPRegion = regexp.MustCompile(`^([a-z]{2}\d{2})`)
```

3. Add the config struct:
```go
type btpConfig struct {
	TargetHierarchy struct {
		GlobalAccountSubdomain string `json:"GlobalAccountSubdomain"`
		SubaccountSubdomain    string `json:"SubaccountSubdomain"`
	} `json:"TargetHierarchy"`
	CLIServerURL string `json:"CLIServerURL"`
}
```

4. Add `extractBTPRegion`:
```go
func extractBTPRegion(subdomain string) string {
	m := reBTPRegion.FindStringSubmatch(subdomain)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
```

5. Add `detectBTP`:
```go
func detectBTP(ctx *ProjectContext) {
	cfg := readBTPConfig()
	if cfg == nil {
		return
	}
	ctx.BTPSubaccount = cfg.TargetHierarchy.SubaccountSubdomain
	if ctx.BTPSubaccount == "" {
		return
	}
	ctx.BTPRegion = extractBTPRegion(ctx.BTPSubaccount)
	ctx.BTPIsTrial = strings.Contains(strings.ToLower(ctx.BTPSubaccount), "trial")
}

func readBTPConfig() *btpConfig {
	path := os.Getenv("BTP_CLIENTCONFIG")
	if path == "" {
		path = defaultBTPConfigPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg btpConfig
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	return &cfg
}

func defaultBTPConfigPath() string {
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
	// BTP CLI v2.x uses ~/.config/btp/; older versions used ~/.config/.btp/
	primary := filepath.Join(home, ".config", "btp", "config.json")
	if fileExists(primary) {
		return primary
	}
	return filepath.Join(home, ".config", ".btp", "config.json")
}
```

6. Add `runtime` to the import list.

7. Call `detectBTP(ctx)` in `Detect()` after `detectDefaultEnv(cwd, ctx)` and before `detectCF(ctx)`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./internal/project/... && go vet ./internal/project/...`
Expected: builds and vets clean

- [ ] **Step 5: Commit**

```bash
git add internal/project/detect.go internal/project/detect_test.go
git commit -m "feat: add BTP CLI context detection with trial heuristic"
```

---

### Task 3: Add CLI Fallback for BTP and CF Detection

**Files:**
- Modify: `internal/project/detect.go` (add `btpCLIFallback()`, `cfCLIFallback()`)
- Modify: `internal/project/detect_test.go` (add tests)

- [ ] **Step 1: Write failing tests for CLI fallback**

Add to `internal/project/detect_test.go`:

```go
func TestParseCFTargetOutput(t *testing.T) {
	output := `API endpoint:   https://api.cf.us10.hana.ondemand.com
API version:    3.215.0
user:           user@example.com
org:            MyOrg
space:          dev`

	org, space, target := parseCFTargetOutput(output)
	if org != "MyOrg" {
		t.Errorf("org = %q, want %q", org, "MyOrg")
	}
	if space != "dev" {
		t.Errorf("space = %q, want %q", space, "dev")
	}
	if target != "https://api.cf.us10.hana.ondemand.com" {
		t.Errorf("target = %q, want %q", target, "https://api.cf.us10.hana.ondemand.com")
	}
}

func TestParseCFTargetOutput_EmptyOnNoMatch(t *testing.T) {
	org, space, target := parseCFTargetOutput("some random output")
	if org != "" || space != "" || target != "" {
		t.Error("should return empty strings on unrecognized output")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/project/...`
Expected: compile error — `parseCFTargetOutput` not defined

- [ ] **Step 3: Implement CLI fallback functions**

Add to `internal/project/detect.go`:

1. Add `context` and `os/exec` and `time` to imports.

2. Add CF target output parser:
```go
func parseCFTargetOutput(output string) (org, space, target string) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "org:") {
			org = strings.TrimSpace(strings.TrimPrefix(line, "org:"))
		} else if strings.HasPrefix(line, "space:") {
			space = strings.TrimSpace(strings.TrimPrefix(line, "space:"))
		} else if strings.HasPrefix(line, "API endpoint:") {
			target = strings.TrimSpace(strings.TrimPrefix(line, "API endpoint:"))
		}
	}
	return
}
```

3. Add CF CLI fallback (called from `detectCF` when config file missing):
```go
func cfCLIFallback(ctx *ProjectContext) {
	c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, "cf", "target").Output()
	if err != nil {
		return
	}
	org, space, target := parseCFTargetOutput(string(out))
	ctx.CFOrg = org
	ctx.CFSpace = space
	ctx.CFRegion = extractCFRegion(target)
}
```

4. Add BTP CLI fallback (called from `detectBTP` when config file missing):
```go
func btpCLIFallback(ctx *ProjectContext) {
	c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, "btp", "--format", "json", "target").Output()
	if err != nil {
		return
	}
	var result struct {
		SubAccount struct {
			Subdomain string `json:"subdomain"`
		} `json:"subAccount"`
	}
	if json.Unmarshal(out, &result) != nil {
		return
	}
	if result.SubAccount.Subdomain == "" {
		return
	}
	ctx.BTPSubaccount = result.SubAccount.Subdomain
	ctx.BTPRegion = extractBTPRegion(ctx.BTPSubaccount)
	ctx.BTPIsTrial = strings.Contains(strings.ToLower(ctx.BTPSubaccount), "trial")
}
```

5. Update `detectCF` to fall back to CLI:
```go
func detectCF(ctx *ProjectContext) {
	cfg := readCFConfig()
	if cfg == nil {
		cfCLIFallback(ctx)
		return
	}
	ctx.CFOrg = cfg.OrganizationFields.Name
	ctx.CFSpace = cfg.SpaceFields.Name
	ctx.CFRegion = extractCFRegion(cfg.Target)
}
```

6. Update `detectBTP` to fall back to CLI:
```go
func detectBTP(ctx *ProjectContext) {
	cfg := readBTPConfig()
	if cfg == nil {
		btpCLIFallback(ctx)
		return
	}
	ctx.BTPSubaccount = cfg.TargetHierarchy.SubaccountSubdomain
	if ctx.BTPSubaccount == "" {
		btpCLIFallback(ctx)
		return
	}
	ctx.BTPRegion = extractBTPRegion(ctx.BTPSubaccount)
	ctx.BTPIsTrial = strings.Contains(strings.ToLower(ctx.BTPSubaccount), "trial")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./internal/project/... && go vet ./internal/project/...`
Expected: builds and vets clean

- [ ] **Step 5: Commit**

```bash
git add internal/project/detect.go internal/project/detect_test.go
git commit -m "feat: add CLI fallback for BTP and CF context detection"
```

---

### Task 4: Add HasBTPContext and Update buildFacts

**Files:**
- Modify: `internal/project/detect.go` (add `HasBTPContext()`, update `buildFacts()`)
- Modify: `internal/project/detect_test.go` (add tests)

- [ ] **Step 1: Write failing tests for HasBTPContext and buildFacts with BTP/CF**

Add to `internal/project/detect_test.go`:

```go
func TestHasBTPContext_TrueWhenBTPSet(t *testing.T) {
	ctx := &ProjectContext{BTPSubaccount: "my-sub"}
	if !ctx.HasBTPContext() {
		t.Error("HasBTPContext should be true when BTPSubaccount is set")
	}
}

func TestHasBTPContext_TrueWhenCFSet(t *testing.T) {
	ctx := &ProjectContext{CFOrg: "MyOrg"}
	if !ctx.HasBTPContext() {
		t.Error("HasBTPContext should be true when CFOrg is set")
	}
}

func TestHasBTPContext_FalseWhenEmpty(t *testing.T) {
	ctx := &ProjectContext{}
	if ctx.HasBTPContext() {
		t.Error("HasBTPContext should be false when no BTP/CF fields set")
	}
}

func TestBuildFacts_IncludesBTPAndCF(t *testing.T) {
	ctx := &ProjectContext{
		RawFiles:      make(map[string]bool),
		BTPSubaccount: "trial-abc",
		BTPRegion:     "eu10",
		BTPIsTrial:    true,
		CFOrg:         "MyOrg",
		CFSpace:       "dev",
		CFRegion:      "us10",
	}
	buildFacts(ctx)

	var btpFact, cfFact *Fact
	for i := range ctx.Facts {
		if ctx.Facts[i].Key == "BTP subaccount" {
			btpFact = &ctx.Facts[i]
		}
		if ctx.Facts[i].Key == "Cloud Foundry" {
			cfFact = &ctx.Facts[i]
		}
	}
	if btpFact == nil {
		t.Fatal("missing BTP subaccount fact")
	}
	if btpFact.Value != "trial-abc (eu10, trial)" {
		t.Errorf("BTP fact value = %q, want %q", btpFact.Value, "trial-abc (eu10, trial)")
	}
	if cfFact == nil {
		t.Fatal("missing Cloud Foundry fact")
	}
	if cfFact.Value != "MyOrg/dev (us10)" {
		t.Errorf("CF fact value = %q, want %q", cfFact.Value, "MyOrg/dev (us10)")
	}
}

func TestBuildFacts_BTPWithoutRegion(t *testing.T) {
	ctx := &ProjectContext{
		RawFiles:      make(map[string]bool),
		BTPSubaccount: "my-account",
	}
	buildFacts(ctx)

	var btpFact *Fact
	for i := range ctx.Facts {
		if ctx.Facts[i].Key == "BTP subaccount" {
			btpFact = &ctx.Facts[i]
		}
	}
	if btpFact == nil {
		t.Fatal("missing BTP subaccount fact")
	}
	if btpFact.Value != "my-account" {
		t.Errorf("BTP fact value = %q, want %q", btpFact.Value, "my-account")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/project/...`
Expected: compile error — `HasBTPContext` not defined

- [ ] **Step 3: Implement HasBTPContext and update buildFacts**

Add `HasBTPContext` method to `detect.go`:
```go
func (ctx *ProjectContext) HasBTPContext() bool {
	return ctx.BTPSubaccount != "" || ctx.CFOrg != ""
}
```

Add BTP/CF fact rendering at the end of `buildFacts()`, after the `Auth` block:
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

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./internal/project/... && go vet ./internal/project/...`
Expected: builds and vets clean

- [ ] **Step 5: Commit**

```bash
git add internal/project/detect.go internal/project/detect_test.go
git commit -m "feat: add HasBTPContext and BTP/CF fact rendering in buildFacts"
```

---

### Task 5: Add BTPFacts to ProjectInfo and Update Gather

**Files:**
- Modify: `internal/content/dynamic.go` (add `BTPFacts []ProjectFact` to `ProjectInfo`)
- Modify: `internal/dynamic/gather.go` (update condition, split facts into `Facts` and `BTPFacts`)
- Modify: `internal/dynamic/gather_test.go` (add tests)

- [ ] **Step 1: Write failing tests for BTP context flowing through gather**

Add to `internal/dynamic/gather_test.go`:

Add to `internal/dynamic/gather_test.go`. **Note:** These tests reference `project.ProjectContext` directly, so add `"github.tools.sap/developer-relations/sap-devs-cli/internal/project"` to the import list in `gather_test.go`.

```go
func TestGatherDynamic_BTPContext_NoBTPWhenNotConfigured(t *testing.T) {
	dir := t.TempDir()
	// Isolate from real BTP/CF configs on the machine
	t.Setenv("BTP_CLIENTCONFIG", filepath.Join(dir, "nonexistent.json"))
	t.Setenv("CF_HOME", dir) // empty dir, no .cf/ subdir
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
	// No project, no BTP
	assert.Nil(t, ctx.Project)
}

func TestGatherDynamic_BTPContext_PopulatedFromProjectContext(t *testing.T) {
	pc := &project.ProjectContext{
		RawFiles:      make(map[string]bool),
		BTPSubaccount: "trial-abc",
		BTPRegion:     "eu10",
		BTPIsTrial:    true,
		CFOrg:         "MyOrg",
		CFSpace:       "dev",
		CFRegion:      "us10",
	}
	// Manually call buildFacts equivalent — Detect() does this, but we pass pc directly
	pc.RebuildFacts()

	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{
		ProjectContext: pc,
	})

	require.NotNil(t, ctx.Project)
	// BTP facts should be in BTPFacts, not in Facts
	assert.NotEmpty(t, ctx.Project.BTPFacts, "BTPFacts should be populated")

	var hasBTP, hasCF bool
	for _, f := range ctx.Project.BTPFacts {
		if f.Key == "BTP subaccount" {
			hasBTP = true
		}
		if f.Key == "Cloud Foundry" {
			hasCF = true
		}
	}
	assert.True(t, hasBTP, "BTPFacts should contain BTP subaccount")
	assert.True(t, hasCF, "BTPFacts should contain Cloud Foundry")
}

func TestGatherDynamic_BTPContext_OnlyBTPNoProjectType(t *testing.T) {
	// ProjectContext with BTP but no Type (no project files detected)
	pc := &project.ProjectContext{
		RawFiles:      make(map[string]bool),
		CFOrg:         "MyOrg",
		CFSpace:       "dev",
		CFRegion:      "us10",
	}
	pc.RebuildFacts()

	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{
		ProjectContext: pc,
	})

	require.NotNil(t, ctx.Project, "Project should be non-nil when BTP context exists")
	assert.Empty(t, ctx.Project.Type, "Type should be empty when no project detected")
	assert.Empty(t, ctx.Project.Facts, "Facts should be empty when no project detected")
	assert.NotEmpty(t, ctx.Project.BTPFacts, "BTPFacts should be populated")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/dynamic/...`
Expected: compile error — `BTPFacts` field does not exist on `ProjectInfo`

- [ ] **Step 3: Add BTPFacts to ProjectInfo**

In `internal/content/dynamic.go`, add to the `ProjectInfo` struct:

```go
type ProjectInfo struct {
	Type       string
	CAPVersion string
	Facts      []ProjectFact
	BTPFacts   []ProjectFact
}
```

- [ ] **Step 4: Update gather.go to split facts and handle BTP-only context**

In `internal/dynamic/gather.go`, replace the project detection block (`if pc != nil && pc.Type != "" {` through `d.Project = info`):

```go
if pc != nil && (pc.Type != "" || pc.HasBTPContext()) {
	if pc.CAPVersion != "" {
		for _, p := range opts.Packs {
			if v, ok := p.Versions["@sap/cds"]; ok {
				pc.LatestCAP = v
				break
			}
		}
		if pc.LatestCAP != "" {
			pc.RebuildFacts()
		}
	}
	info := &content.ProjectInfo{
		Type:       pc.Type,
		CAPVersion: pc.CAPVersion,
	}
	btpKeys := map[string]bool{"BTP subaccount": true, "Cloud Foundry": true}
	for _, f := range pc.Facts {
		pf := content.ProjectFact{Key: f.Key, Value: f.Value, Warn: f.Warn}
		if btpKeys[f.Key] {
			info.BTPFacts = append(info.BTPFacts, pf)
		} else {
			info.Facts = append(info.Facts, pf)
		}
	}
	d.Project = info
}
```

- [ ] **Step 5: Fix existing gather tests for BTP/CF environment isolation**

Since BTP/CF detection reads global config (not cwd-scoped), existing gather tests that call `GatherDynamic` without an explicit `ProjectContext` will become flaky on machines with BTP/CF CLIs configured. Add environment isolation to affected tests.

In `internal/dynamic/gather_test.go`, add to the top of these existing test functions:

For `TestGatherDynamic_ProjectType_EmptyWhenNoFiles`:

```go
func TestGatherDynamic_ProjectType_EmptyWhenNoFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BTP_CLIENTCONFIG", filepath.Join(dir, "nonexistent.json"))
	t.Setenv("CF_HOME", dir)
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
	assert.Nil(t, ctx.Project)
}
```

For `TestGatherDynamic_NeverPanics_MissingCWD`:

```go
func TestGatherDynamic_NeverPanics_MissingCWD(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BTP_CLIENTCONFIG", filepath.Join(dir, "nonexistent.json"))
	t.Setenv("CF_HOME", dir)
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: "/nonexistent/dir/xyz"})
	require.NotNil(t, ctx)
	assert.Nil(t, ctx.Project)
}
```

Also add `"path/filepath"` to the import list in `gather_test.go` if not already present.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go build ./internal/dynamic/... && go vet ./internal/dynamic/...`
Expected: builds and vets clean

- [ ] **Step 7: Commit**

```bash
git add internal/content/dynamic.go internal/dynamic/gather.go internal/dynamic/gather_test.go
git commit -m "feat: add BTPFacts to ProjectInfo and split facts at gather time"
```

---

### Task 6: Render BTP Environment Section in Dynamic Context

**Files:**
- Modify: `internal/content/render.go` (add BTP rendering block in `renderDynamic()`)
- Modify: `internal/content/render_dynamic_test.go` (add tests)

- [ ] **Step 1: Write failing tests for BTP rendering**

Add to `internal/content/render_dynamic_test.go`:

```go
func TestRenderDynamic_BTPEnvironment(t *testing.T) {
	d := &content.DynamicContext{
		CLIVersion: "1.5.0",
		Project: &content.ProjectInfo{
			Type: "CAP (Node.js)",
			Facts: []content.ProjectFact{
				{Key: "Project type", Value: "CAP (Node.js)"},
			},
			BTPFacts: []content.ProjectFact{
				{Key: "BTP subaccount", Value: "trial-abc (eu10, trial)"},
				{Key: "Cloud Foundry", Value: "MyOrg/dev (us10)"},
			},
		},
	}

	out := content.RenderDynamic(d)

	if !strings.Contains(out, "**BTP Environment (detected):**") {
		t.Error("missing BTP Environment header")
	}
	if !strings.Contains(out, "BTP subaccount: trial-abc (eu10, trial)") {
		t.Error("missing BTP subaccount fact")
	}
	if !strings.Contains(out, "Cloud Foundry: MyOrg/dev (us10)") {
		t.Error("missing Cloud Foundry fact")
	}
	if !strings.Contains(out, "**Project Context (detected):**") {
		t.Error("project facts should still render")
	}
}

func TestRenderDynamic_BTPOnly_NoProjectFacts(t *testing.T) {
	d := &content.DynamicContext{
		CLIVersion: "1.5.0",
		Project: &content.ProjectInfo{
			BTPFacts: []content.ProjectFact{
				{Key: "Cloud Foundry", Value: "MyOrg/dev (us10)"},
			},
		},
	}

	out := content.RenderDynamic(d)

	if !strings.Contains(out, "**BTP Environment (detected):**") {
		t.Error("missing BTP Environment header")
	}
	if strings.Contains(out, "**Project Context (detected):**") {
		t.Error("should not render project context header when no project facts")
	}
}

func TestRenderDynamic_NoBTP(t *testing.T) {
	d := &content.DynamicContext{
		CLIVersion: "1.5.0",
		Project: &content.ProjectInfo{
			Type: "CAP (Node.js)",
			Facts: []content.ProjectFact{
				{Key: "Project type", Value: "CAP (Node.js)"},
			},
		},
	}

	out := content.RenderDynamic(d)

	if strings.Contains(out, "BTP Environment") {
		t.Error("should not render BTP section when no BTP facts")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/content/...`
Expected: builds OK (no new exports), but tests would fail on content assertions

- [ ] **Step 3: Add BTP rendering block in renderDynamic()**

In `internal/content/render.go`, in `renderDynamic()`, after the project context block (line 195) and before the wired MCP servers block (line 197), add:

```go
// BTP environment (separate from project context)
if d.Project != nil && len(d.Project.BTPFacts) > 0 {
	b.WriteString("\n**BTP Environment (detected):**\n")
	for _, f := range d.Project.BTPFacts {
		b.WriteString(fmt.Sprintf("- %s: %s\n", f.Key, f.Value))
	}
}
```

The complete project context + BTP environment block should now read:

```go
// Project context (omit if no project detected)
if d.Project != nil && len(d.Project.Facts) > 0 {
	b.WriteString("\n**Project Context (detected):**\n")
	for _, f := range d.Project.Facts {
		b.WriteString(fmt.Sprintf("- %s: %s\n", f.Key, f.Value))
	}
	for _, f := range d.ProjectFindings {
		if f.Severity == "error" || f.Severity == "warning" {
			b.WriteString(fmt.Sprintf("- ⚠ %s\n", f.Message))
		}
	}
}

// BTP environment (separate from project context)
if d.Project != nil && len(d.Project.BTPFacts) > 0 {
	b.WriteString("\n**BTP Environment (detected):**\n")
	for _, f := range d.Project.BTPFacts {
		b.WriteString(fmt.Sprintf("- %s: %s\n", f.Key, f.Value))
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./internal/content/... && go vet ./internal/content/...`
Expected: builds and vets clean

- [ ] **Step 5: Commit**

```bash
git add internal/content/render.go internal/content/render_dynamic_test.go
git commit -m "feat: render BTP Environment section in dynamic context output"
```

---

### Task 7: Full Build Verification and Documentation

**Files:**
- Verify: all `go build ./...` and `go vet ./...`
- Modify: `CLAUDE.md` (update Architecture section)

- [ ] **Step 1: Run full build and vet**

Run: `go build ./... && go vet ./...`
Expected: clean build, no warnings

- [ ] **Step 2: Run dry-run inject to verify BTP facts appear**

Run: `SAP_DEVS_DEV=1 go run . inject --dry-run`
Expected: If BTP/CF CLIs are configured locally, the output should contain a `**BTP Environment (detected):**` section with the subaccount and/or CF org/space. If neither is configured, the section is silently omitted.

- [ ] **Step 3: Update CLAUDE.md**

In the `### Project Detection & Health Check` section of `CLAUDE.md`, add a brief note about BTP detection:

After the existing bullet points, add:
```markdown
- `Detect(cwd)` also checks BTP CLI config (`BTP_CLIENTCONFIG` env var or default path) and CF CLI config (`CF_HOME` env var or `~/.cf/config.json`) for active subaccount/region and org/space. Trial accounts are heuristically flagged. Falls back to `btp target` / `cf target` CLI exec with 3-second timeout. BTP/CF context is rendered as a separate `**BTP Environment (detected):**` section via `BTPFacts` on `ProjectInfo`.
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: document BTP active context detection in CLAUDE.md"
```

---

## Summary

| Task | Description | Key Files |
|------|-------------|-----------|
| 1 | CF config parsing + region extraction | `detect.go`, `detect_test.go` |
| 2 | BTP config parsing + trial detection | `detect.go`, `detect_test.go` |
| 3 | CLI fallback for BTP and CF | `detect.go`, `detect_test.go` |
| 4 | `HasBTPContext()` + `buildFacts()` update | `detect.go`, `detect_test.go` |
| 5 | `BTPFacts` on `ProjectInfo` + gather split | `dynamic.go`, `gather.go`, `gather_test.go` |
| 6 | Render BTP Environment section | `render.go`, `render_dynamic_test.go` |
| 7 | Full build verification + docs | `CLAUDE.md` |
