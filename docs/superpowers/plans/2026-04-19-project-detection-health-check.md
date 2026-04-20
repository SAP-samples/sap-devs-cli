# Project-Aware Context Detection & Health Check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add project file detection and health checking to the `doctor` command, and inject detected project context (facts + warnings) into AI tools during `inject`.

**Architecture:** A new `internal/project` package provides `Detect()` (scans project files) and `Check()` (validates against pack knowledge). Both `cmd/doctor.go` and `cmd/inject.go` consume it. The existing `DynamicContext.ProjectType string` is replaced with `DynamicContext.Project *project.ProjectContext`. Rendering in `render.go` expands the project section from a single line to a structured facts + warnings block.

**Tech Stack:** Go 1.22+, cobra CLI, YAML/JSON parsing (stdlib `encoding/json`, `gopkg.in/yaml.v3`), no new external dependencies.

**Spec:** `docs/superpowers/specs/2026-04-19-project-detection-health-check-design.md`

**Local validation:** `go build ./...` and `go vet ./...` (go test fails on Windows; CI is authoritative).

---

### Task 1: Create `internal/project` package — types and `Detect()` skeleton

**Files:**
- Create: `internal/project/detect.go`
- Create: `internal/project/detect_test.go`

This task builds the core detection engine: types (`ProjectContext`, `Fact`) and the `Detect()` function that scans project files.

- [ ] **Step 1: Write the failing test for `Detect()` with a CAP Node.js fixture**

Create `internal/project/detect_test.go`:

```go
package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_CAPNodeJS(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"@sap/cds": "9.6.2"},
		"devDependencies": {"@sap/cds-dk": "9.6.2"}
	}`)
	writeFile(t, dir, ".cdsrc.json", `{}`)
	writeFile(t, dir, "xs-security.json", `{"xsappname":"myapp"}`)
	writeFile(t, dir, "mta.yaml", `ID: myapp`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "CAP (Node.js)" {
		t.Errorf("Type = %q, want %q", ctx.Type, "CAP (Node.js)")
	}
	if ctx.CAPVersion != "9.6.2" {
		t.Errorf("CAPVersion = %q, want %q", ctx.CAPVersion, "9.6.2")
	}
	if ctx.Auth != "xsuaa" {
		t.Errorf("Auth = %q, want %q", ctx.Auth, "xsuaa")
	}
	if ctx.Deployment != "mta-cf" {
		t.Errorf("Deployment = %q, want %q", ctx.Deployment, "mta-cf")
	}
	if ctx.HasCDSRC != true {
		t.Error("HasCDSRC should be true")
	}
	if !ctx.RawFiles["package.json"] || !ctx.RawFiles[".cdsrc.json"] || !ctx.RawFiles["xs-security.json"] || !ctx.RawFiles["mta.yaml"] {
		t.Error("RawFiles should record all detected signal files")
	}
	if len(ctx.Facts) == 0 {
		t.Error("Facts should be populated")
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/project/... -run TestDetect_CAPNodeJS -v`
Expected: FAIL — package doesn't exist yet.

- [ ] **Step 3: Implement `Detect()` with all detectors**

Create `internal/project/detect.go`:

```go
package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type ProjectContext struct {
	Type          string
	CAPVersion    string
	LatestCAP     string
	Database      string
	Deployment    string
	Auth          string
	HasCDSRC      bool
	HasDefaultEnv bool
	Facts         []Fact
	RawFiles      map[string]bool
}

type Fact struct {
	Key   string
	Value string
	Warn  string
}

func Detect(cwd string) (*ProjectContext, error) {
	if cwd == "" {
		return &ProjectContext{RawFiles: make(map[string]bool)}, nil
	}
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

func detectCAP(cwd string, ctx *ProjectContext) {
	// .cdsrc.json
	if fileExists(filepath.Join(cwd, ".cdsrc.json")) {
		ctx.HasCDSRC = true
		ctx.RawFiles[".cdsrc.json"] = true
	}

	// package.json — CAP Node.js
	pkgPath := filepath.Join(cwd, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		ctx.RawFiles["package.json"] = true
		var pkg packageJSON
		if json.Unmarshal(data, &pkg) == nil {
			if v, ok := pkg.Dependencies["@sap/cds"]; ok {
				ctx.Type = "CAP (Node.js)"
				ctx.CAPVersion = cleanVersion(v)
			} else if _, ok := pkg.DevDependencies["@sap/cds"]; ok {
				ctx.Type = "CAP (Node.js)"
			}
			// Database detection from cds.requires
			ctx.Database = detectDatabase(pkg)
		}
	}

	// pom.xml — CAP Java
	if ctx.Type == "" {
		pomPath := filepath.Join(cwd, "pom.xml")
		if data, err := os.ReadFile(pomPath); err == nil {
			ctx.RawFiles["pom.xml"] = true
			if strings.Contains(string(data), "com.sap.cds") {
				ctx.Type = "CAP (Java)"
			}
		}
	}

	// If .cdsrc.json exists but no @sap/cds dep found, still mark as CAP
	if ctx.HasCDSRC && ctx.Type == "" {
		ctx.Type = "CAP (Node.js)"
	}
}

func detectMTA(cwd string, ctx *ProjectContext) {
	for _, name := range []string{"mta.yaml", ".mta.yaml"} {
		if fileExists(filepath.Join(cwd, name)) {
			ctx.RawFiles[name] = true
			ctx.Deployment = "mta-cf"
			return
		}
	}
}

func detectAuth(cwd string, ctx *ProjectContext) {
	if fileExists(filepath.Join(cwd, "xs-security.json")) {
		ctx.RawFiles["xs-security.json"] = true
		ctx.Auth = "xsuaa"
	}
}

func detectAppRouter(cwd string, ctx *ProjectContext) {
	if fileExists(filepath.Join(cwd, "xs-app.json")) {
		ctx.RawFiles["xs-app.json"] = true
		if ctx.Type == "" {
			ctx.Type = "Fiori / BAS app"
		}
	}
}

func detectKyma(cwd string, ctx *ProjectContext) {
	for _, name := range []string{"chart", "helm"} {
		info, err := os.Stat(filepath.Join(cwd, name))
		if err == nil && info.IsDir() {
			ctx.RawFiles[name+"/"] = true
			if ctx.Deployment == "" {
				ctx.Deployment = "helm-kyma"
			}
			return
		}
	}
}

func detectDefaultEnv(cwd string, ctx *ProjectContext) {
	if fileExists(filepath.Join(cwd, "default-env.json")) {
		ctx.RawFiles["default-env.json"] = true
		ctx.HasDefaultEnv = true
	}
}

func buildFacts(ctx *ProjectContext) {
	if ctx.Type == "" {
		// Check for plain Node.js as fallback
		if ctx.RawFiles["package.json"] {
			ctx.Type = "Node.js"
		}
	}
	if ctx.Type != "" {
		ctx.Facts = append(ctx.Facts, Fact{Key: "Project type", Value: ctx.Type})
	}
	if ctx.CAPVersion != "" {
		f := Fact{Key: "CAP version", Value: "@sap/cds " + ctx.CAPVersion}
		if ctx.LatestCAP != "" {
			cmp := CompareVersions(ctx.CAPVersion, ctx.LatestCAP)
			if cmp < 0 {
				f.Warn = "update available: " + ctx.LatestCAP
				f.Value += " (latest: " + ctx.LatestCAP + ")"
			}
		}
		ctx.Facts = append(ctx.Facts, f)
	}
	if ctx.Database != "" {
		label := ctx.Database
		if ctx.Database == "hana" {
			label = "SAP HANA Cloud"
		}
		ctx.Facts = append(ctx.Facts, Fact{Key: "Database", Value: label})
	}
	if ctx.Deployment != "" {
		label := ctx.Deployment
		if ctx.Deployment == "mta-cf" {
			label = "MTA to Cloud Foundry"
		} else if ctx.Deployment == "helm-kyma" {
			label = "Helm to Kyma/Kubernetes"
		}
		ctx.Facts = append(ctx.Facts, Fact{Key: "Deployment", Value: label})
	}
	if ctx.Auth != "" {
		ctx.Facts = append(ctx.Facts, Fact{Key: "Auth", Value: "XSUAA (xs-security.json detected)"})
	}
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	CDS             *cdsConfig        `json:"cds"`
}

type cdsConfig struct {
	Requires map[string]json.RawMessage `json:"requires"`
}

func detectDatabase(pkg packageJSON) string {
	// Check cds.requires for hana/sqlite/postgres
	if pkg.CDS != nil && pkg.CDS.Requires != nil {
		for key := range pkg.CDS.Requires {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "hana") {
				return "hana"
			}
		}
	}
	// Check dependencies for hana driver
	for dep := range pkg.Dependencies {
		if strings.Contains(dep, "hana") {
			return "hana"
		}
	}
	return ""
}

func cleanVersion(v string) string {
	v = strings.TrimLeft(v, "^~>=<! ")
	return v
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/project/... -run TestDetect_CAPNodeJS -v`
Expected: PASS

- [ ] **Step 5: Add additional detection tests**

Add to `detect_test.go`:

```go
func TestDetect_CAPJava(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pom.xml", `<project><dependencies>
		<dependency><groupId>com.sap.cds</groupId></dependency>
	</dependencies></project>`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "CAP (Java)" {
		t.Errorf("Type = %q, want %q", ctx.Type, "CAP (Java)")
	}
}

func TestDetect_MTA(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "mta.yaml", "ID: myapp")

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Deployment != "mta-cf" {
		t.Errorf("Deployment = %q, want %q", ctx.Deployment, "mta-cf")
	}
}

func TestDetect_Fiori(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "xs-app.json", `{"welcomeFile":"/index.html"}`)
	writeFile(t, dir, "xs-security.json", `{"xsappname":"myapp"}`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "Fiori / BAS app" {
		t.Errorf("Type = %q, want %q", ctx.Type, "Fiori / BAS app")
	}
	if ctx.Auth != "xsuaa" {
		t.Errorf("Auth = %q, want %q", ctx.Auth, "xsuaa")
	}
}

func TestDetect_Kyma(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "chart"), 0755)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Deployment != "helm-kyma" {
		t.Errorf("Deployment = %q, want %q", ctx.Deployment, "helm-kyma")
	}
}

func TestDetect_DefaultEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "default-env.json", `{}`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ctx.HasDefaultEnv {
		t.Error("HasDefaultEnv should be true")
	}
}

func TestDetect_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "" {
		t.Errorf("Type should be empty for empty dir, got %q", ctx.Type)
	}
	if len(ctx.Facts) != 0 {
		t.Errorf("Facts should be empty for empty dir, got %d", len(ctx.Facts))
	}
}

func TestDetect_EmptyCWD(t *testing.T) {
	ctx, err := Detect("")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "" {
		t.Errorf("Type should be empty for empty CWD, got %q", ctx.Type)
	}
}

func TestDetect_PlainNodeJS(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"myapp"}`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Type != "Node.js" {
		t.Errorf("Type = %q, want %q", ctx.Type, "Node.js")
	}
}

func TestDetect_HANADatabase(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"@sap/cds": "9.6.2"},
		"cds": {"requires": {"db": {}, "hana": {}}}
	}`)

	ctx, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Database != "hana" {
		t.Errorf("Database = %q, want %q", ctx.Database, "hana")
	}
}
```

- [ ] **Step 6: Run all detection tests**

Run: `go test ./internal/project/... -v`
Expected: All PASS

- [ ] **Step 7: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean build, no warnings.

- [ ] **Step 8: Commit**

```bash
git add internal/project/detect.go internal/project/detect_test.go
git commit -m "feat(project): add project detection engine with Detect()"
```

---

### Task 2: Add semver comparison utility

**Files:**
- Create: `internal/project/semver.go`
- Create: `internal/project/semver_test.go`

- [ ] **Step 1: Write failing tests for `CompareVersions` and `VersionStaleness`**

Create `internal/project/semver_test.go`:

```go
package project

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"9.6.2", "9.8.0", -1},
		{"9.8.0", "9.6.2", 1},
		{"9.8.0", "9.8.0", 0},
		{"10.0.0", "9.99.99", 1},
		{"1.2.3", "1.2.4", -1},
		{"invalid", "9.8.0", 0},
		{"9.8.0", "invalid", 0},
	}
	for _, tt := range tests {
		got := CompareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestVersionStaleness(t *testing.T) {
	tests := []struct {
		current, latest string
		wantSev         string
	}{
		{"9.6.0", "9.8.0", "warning"},  // 2 minor behind
		{"9.7.0", "9.8.0", ""},         // 1 minor behind — ok
		{"8.0.0", "9.8.0", "error"},    // 1 major behind
		{"9.8.0", "9.8.0", ""},         // up to date
		{"9.9.0", "9.8.0", ""},         // ahead — ok
		{"invalid", "9.8.0", ""},       // unparseable — skip
	}
	for _, tt := range tests {
		got := VersionStaleness(tt.current, tt.latest)
		if got != tt.wantSev {
			t.Errorf("VersionStaleness(%q, %q) = %q, want %q", tt.current, tt.latest, got, tt.wantSev)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/project/... -run TestCompare -v`
Expected: FAIL — `CompareVersions` not defined.

- [ ] **Step 3: Implement `semver.go`**

Create `internal/project/semver.go`:

```go
package project

import (
	"strconv"
	"strings"
)

type semver struct {
	Major, Minor, Patch int
	Valid               bool
}

func parseSemver(s string) semver {
	s = strings.TrimLeft(s, "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 2 {
		return semver{}
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	patch := 0
	if len(parts) == 3 {
		// Strip pre-release suffix (e.g., "0-rc.1")
		p := strings.SplitN(parts[2], "-", 2)[0]
		patch, _ = strconv.Atoi(p)
	}
	if err1 != nil || err2 != nil {
		return semver{}
	}
	return semver{Major: major, Minor: minor, Patch: patch, Valid: true}
}

// CompareVersions returns -1 if a < b, 0 if equal (or unparseable), 1 if a > b.
func CompareVersions(a, b string) int {
	va, vb := parseSemver(a), parseSemver(b)
	if !va.Valid || !vb.Valid {
		return 0
	}
	if va.Major != vb.Major {
		if va.Major < vb.Major {
			return -1
		}
		return 1
	}
	if va.Minor != vb.Minor {
		if va.Minor < vb.Minor {
			return -1
		}
		return 1
	}
	if va.Patch != vb.Patch {
		if va.Patch < vb.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// VersionStaleness returns "error" (>1 major behind), "warning" (>2 minor behind),
// or "" (up-to-date or unparseable).
func VersionStaleness(current, latest string) string {
	vc, vl := parseSemver(current), parseSemver(latest)
	if !vc.Valid || !vl.Valid {
		return ""
	}
	if vl.Major-vc.Major > 0 {
		return "error"
	}
	if vc.Major == vl.Major && vl.Minor-vc.Minor >= 2 {
		return "warning"
	}
	return ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/project/... -run "TestCompare|TestVersion" -v`
Expected: All PASS

- [ ] **Step 5: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean.

- [ ] **Step 6: Commit**

```bash
git add internal/project/semver.go internal/project/semver_test.go
git commit -m "feat(project): add semver comparison for version staleness checks"
```

---

### Task 3: Add health check engine (`Check()`)

**Files:**
- Create: `internal/project/check.go`
- Create: `internal/project/check_test.go`

- [ ] **Step 1: Write failing tests for `Check()`**

Create `internal/project/check_test.go`:

```go
package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestCheck_DefaultEnvNotGitignored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "default-env.json", `{}`)
	writeFile(t, dir, "package.json", `{"dependencies":{"@sap/cds":"9.8.0"}}`)
	// No .gitignore

	ctx, _ := Detect(dir)
	findings := Check(ctx, dir, nil)

	found := findBySeverity(findings, "error")
	if len(found) == 0 {
		t.Error("expected error-severity finding for default-env.json not gitignored")
	}
}

func TestCheck_DefaultEnvGitignored(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "default-env.json", `{}`)
	writeFile(t, dir, ".gitignore", "default-env.json\n")
	writeFile(t, dir, "package.json", `{"dependencies":{"@sap/cds":"9.8.0"}}`)

	ctx, _ := Detect(dir)
	findings := Check(ctx, dir, nil)

	for _, f := range findings {
		if f.Category == "practice" && f.Severity == "error" {
			t.Errorf("should not flag default-env.json when gitignored, got: %s", f.Message)
		}
	}
}

func TestCheck_VersionStaleness(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"dependencies":{"@sap/cds":"9.4.0"}}`)

	ctx, _ := Detect(dir)
	packs := []*content.Pack{
		{ID: "cap", Versions: map[string]string{"@sap/cds": "9.8.0"}},
	}
	findings := Check(ctx, dir, packs)

	found := findByCategory(findings, "version")
	if len(found) == 0 {
		t.Error("expected version staleness finding")
	}
}

func TestCheck_MissingLintScript(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"@sap/cds": "9.8.0"},
		"scripts": {"start": "cds-serve"}
	}`)

	ctx, _ := Detect(dir)
	findings := Check(ctx, dir, nil)

	found := findByMessage(findings, "lint")
	if len(found) == 0 {
		t.Error("expected warning about missing lint script")
	}
}

func TestCheck_NoFindingsForCleanProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"@sap/cds": "9.8.0"},
		"devDependencies": {"@sap/cds-dk": "9.8.0"},
		"scripts": {"lint": "npx cds lint"}
	}`)
	writeFile(t, dir, ".gitignore", "node_modules/\n")

	ctx, _ := Detect(dir)
	packs := []*content.Pack{
		{ID: "cap", Versions: map[string]string{"@sap/cds": "9.8.0"}},
	}
	findings := Check(ctx, dir, packs)

	errors := findBySeverity(findings, "error")
	if len(errors) > 0 {
		t.Errorf("expected no errors for clean project, got: %v", errors)
	}
}

func findBySeverity(findings []Finding, sev string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Severity == sev {
			out = append(out, f)
		}
	}
	return out
}

func findByCategory(findings []Finding, cat string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Category == cat {
			out = append(out, f)
		}
	}
	return out
}

func findByMessage(findings []Finding, substr string) []Finding {
	var out []Finding
	for _, f := range findings {
		if strings.Contains(f.Message, substr) {
			out = append(out, f)
		}
	}
	return out
}
```

Note: The `contains` helper uses a simple substring check. The `writeFile` helper is already defined in `detect_test.go` in the same package.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/project/... -run TestCheck -v`
Expected: FAIL — `Check` and `Finding` not defined; `Pack.Versions` field doesn't exist yet.

- [ ] **Step 3: Add `Versions` field to `Pack` and `packMeta`**

Modify `internal/content/pack.go`:

At line 54 (inside the `Pack` struct, after `LearningPaths`), add:

```go
	Versions map[string]string // latest known versions (e.g., "@sap/cds": "9.8.0")
```

At line 304 (inside the `packMeta` struct, after the `Locales` field), add:

```go
	Versions map[string]string `yaml:"versions,omitempty"`
```

In the `LoadPack()` function, after line 330 (`AdditivePosition: meta.AdditivePosition,`) add:

```go
		Versions:         meta.Versions,
```

- [ ] **Step 4: Update `pack.schema.json`**

In `content/schemas/pack.schema.json`, add a new property after the `additive_position` block (after line 68, before the closing `}`):

```json
    "versions": {
      "type": "object",
      "description": "Latest known versions for staleness checks (e.g., {\"@sap/cds\": \"9.8.0\"})",
      "additionalProperties": {
        "type": "string"
      }
    }
```

- [ ] **Step 5: Implement `Check()`**

Create `internal/project/check.go`:

```go
package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

type Finding struct {
	Category string
	Severity string
	Message  string
	Fix      string
}

func Check(ctx *ProjectContext, cwd string, packs []*content.Pack) []Finding {
	if ctx == nil || ctx.Type == "" {
		return nil
	}
	var findings []Finding
	findings = append(findings, checkDependencies(ctx, cwd)...)
	findings = append(findings, checkVersions(ctx, packs)...)
	findings = append(findings, checkPractices(ctx, cwd)...)
	findings = append(findings, checkConstraints(ctx, cwd)...)
	return findings
}

func checkDependencies(ctx *ProjectContext, cwd string) []Finding {
	var findings []Finding
	isCAP := strings.HasPrefix(ctx.Type, "CAP")

	if isCAP && ctx.RawFiles["package.json"] {
		data, err := os.ReadFile(filepath.Join(cwd, "package.json"))
		if err == nil {
			var pkg struct {
				DevDependencies map[string]string `json:"devDependencies"`
			}
			if json.Unmarshal(data, &pkg) == nil {
				if _, ok := pkg.DevDependencies["@sap/cds-dk"]; !ok {
					findings = append(findings, Finding{
						Category: "dependency",
						Severity: "warning",
						Message:  "@sap/cds-dk not found in devDependencies",
						Fix:      "Run 'npm add -D @sap/cds-dk'",
					})
				}
			}
		}
	}

	if ctx.HasDefaultEnv && !ctx.RawFiles[".gitignore"] {
		if !fileExists(filepath.Join(cwd, ".gitignore")) {
			findings = append(findings, Finding{
				Category: "dependency",
				Severity: "warning",
				Message:  "No .gitignore found in project with default-env.json",
				Fix:      "Create .gitignore and add 'default-env.json'",
			})
		}
	}

	return findings
}

func checkVersions(ctx *ProjectContext, packs []*content.Pack) []Finding {
	var findings []Finding
	versions := collectVersions(packs)

	if ctx.CAPVersion != "" {
		if latest, ok := versions["@sap/cds"]; ok {
			sev := VersionStaleness(ctx.CAPVersion, latest)
			if sev != "" {
				findings = append(findings, Finding{
					Category: "version",
					Severity: sev,
					Message:  fmt.Sprintf("@sap/cds %s is behind latest %s", ctx.CAPVersion, latest),
					Fix:      "Run 'npm update @sap/cds'",
				})
			}
		}
	}
	return findings
}

func checkPractices(ctx *ProjectContext, cwd string) []Finding {
	var findings []Finding

	if ctx.HasDefaultEnv && !isGitignored(cwd, "default-env.json") {
		findings = append(findings, Finding{
			Category: "practice",
			Severity: "error",
			Message:  "default-env.json is not in .gitignore (credential leak risk)",
			Fix:      "Add 'default-env.json' to .gitignore",
		})
	}

	return findings
}

func checkConstraints(ctx *ProjectContext, cwd string) []Finding {
	var findings []Finding
	isCAP := strings.HasPrefix(ctx.Type, "CAP")

	if isCAP && ctx.RawFiles["package.json"] {
		data, err := os.ReadFile(filepath.Join(cwd, "package.json"))
		if err == nil {
			var pkg struct {
				Scripts map[string]string `json:"scripts"`
			}
			if json.Unmarshal(data, &pkg) == nil {
				if _, ok := pkg.Scripts["lint"]; !ok {
					findings = append(findings, Finding{
						Category: "constraint",
						Severity: "warning",
						Message:  "No 'lint' script in package.json",
						Fix:      "Add '\"lint\": \"npx cds lint\"' to scripts",
					})
				}
			}
		}
	}

	return findings
}

func collectVersions(packs []*content.Pack) map[string]string {
	versions := make(map[string]string)
	// Packs are sorted by weight descending; first one wins per key.
	for _, p := range packs {
		for k, v := range p.Versions {
			if _, exists := versions[k]; !exists {
				versions[k] = v
			}
		}
	}
	return versions
}

func isGitignored(cwd, filename string) bool {
	f, err := os.Open(filepath.Join(cwd, ".gitignore"))
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == filename {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/project/... -v`
Expected: All PASS (detection + check tests)

- [ ] **Step 7: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean.

- [ ] **Step 8: Commit**

```bash
git add internal/project/check.go internal/project/check_test.go internal/content/pack.go content/schemas/pack.schema.json
git commit -m "feat(project): add health check engine with Check()"
```

---

### Task 4: Add `versions` data to CAP pack

**Files:**
- Modify: `content/packs/cap/pack.yaml`

- [ ] **Step 1: Add versions map to CAP pack**

In `content/packs/cap/pack.yaml`, add at the end (after the `locales:` block):

```yaml
versions:
  "@sap/cds": "9.8.0"
  "@sap/cds-dk": "9.8.0"
```

- [ ] **Step 2: Verify build still works (schema validation)**

Run: `go build ./... && go vet ./...`
Expected: Clean.

- [ ] **Step 3: Commit**

```bash
git add content/packs/cap/pack.yaml
git commit -m "feat(cap): add latest known versions for staleness checks"
```

---

### Task 5: Integrate detection into `DynamicContext` and `renderDynamic()`

**Files:**
- Modify: `internal/content/dynamic.go:13` — replace `ProjectType string` with `Project *project.ProjectContext`
- Modify: `internal/content/render.go:108-160` — expand project rendering
- Modify: `internal/dynamic/gather.go:51` — call `project.Detect()` instead of `detectProjectType()`

- [ ] **Step 1: Write failing test for the new render output**

Create or add to `internal/content/render_test.go`. First, add a `RenderDynamic` export to `internal/content/export_test.go` (following the project's existing pattern for exposing unexported functions to tests):

```go
func RenderDynamic(d *DynamicContext) string { return renderDynamic(d) }
```

Then create `internal/content/render_dynamic_test.go` using the external test package (`content_test`):

```go
package content_test

import (
	"strings"
	"testing"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/project"
)

func TestRenderDynamic_ProjectContext(t *testing.T) {
	pc := &project.ProjectContext{
		Type:       "CAP (Node.js)",
		CAPVersion: "9.6.2",
		Database:   "hana",
		Deployment: "mta-cf",
		Auth:       "xsuaa",
		Facts: []project.Fact{
			{Key: "Project type", Value: "CAP (Node.js)"},
			{Key: "CAP version", Value: "@sap/cds 9.6.2 (latest: 9.8.0)", Warn: "update available: 9.8.0"},
			{Key: "Database", Value: "SAP HANA Cloud"},
			{Key: "Deployment", Value: "MTA to Cloud Foundry"},
			{Key: "Auth", Value: "XSUAA (xs-security.json detected)"},
		},
	}
	d := &content.DynamicContext{
		CLIVersion: "1.5.0",
		Project:    pc,
	}

	out := content.RenderDynamic(d)

	if !strings.Contains(out, "**Project Context (detected):**") {
		t.Error("missing Project Context header")
	}
	if !strings.Contains(out, "CAP version") {
		t.Error("missing CAP version fact")
	}
	if !strings.Contains(out, "SAP HANA Cloud") {
		t.Error("missing database fact")
	}
}

func TestRenderDynamic_NoProject(t *testing.T) {
	d := &content.DynamicContext{CLIVersion: "1.5.0"}

	out := content.RenderDynamic(d)

	if strings.Contains(out, "Project Context") {
		t.Error("should not render project section when no project detected")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/content/... -run TestRenderDynamic_Project -v`
Expected: FAIL — `DynamicContext` has no `Project` field.

- [ ] **Step 3: Update `DynamicContext` in `dynamic.go`**

In `internal/content/dynamic.go`, add the import and replace the `ProjectType` field:

Replace line 4:
```go
import "time"
```
with:
```go
import (
	"time"

	"github.com/SAP-samples/sap-devs-cli/internal/project"
)
```

Replace line 13 (`ProjectType     string`):
```go
	Project         *project.ProjectContext
```

- [ ] **Step 4: Update `renderDynamic()` in `render.go`**

In `internal/content/render.go`, add the project import and replace lines 137-140 (the `ProjectType` rendering block) with the new project context rendering:

Replace:
```go
	// Project type (omit if empty)
	if d.ProjectType != "" {
		b.WriteString(fmt.Sprintf("**Project type:** %s\n", d.ProjectType))
	}
```

With:
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
```

Also add a `ProjectFindings` field to `DynamicContext` in `dynamic.go`:

```go
	ProjectFindings []project.Finding
```

- [ ] **Step 5: Update `gather.go` to use `project.Detect()`**

In `internal/dynamic/gather.go`:

Add import:
```go
	"github.com/SAP-samples/sap-devs-cli/internal/project"
```

Replace line 50-51:
```go
	// Project type
	d.ProjectType = detectProjectType(opts.CWD)
```
With:
```go
	// Project detection
	if pc, err := project.Detect(opts.CWD); err == nil {
		d.Project = pc
	}
```

- [ ] **Step 6: Update `cmd/inject.go` to run health checks and pass findings**

In `cmd/inject.go`, after the `dynCtx` is gathered (after line 250), add:

```go
	// Run project health checks and attach findings to dynamic context
	if dynCtx.Project != nil && dynCtx.Project.Type != "" {
		dynCtx.ProjectFindings = project.Check(dynCtx.Project, cwd, packs)
		// Enrich facts with version info from packs
		versions := make(map[string]string)
		for _, p := range packs {
			for k, v := range p.Versions {
				if _, exists := versions[k]; !exists {
					versions[k] = v
				}
			}
		}
		if dynCtx.Project.CAPVersion != "" {
			if latest, ok := versions["@sap/cds"]; ok {
				dynCtx.Project.LatestCAP = latest
			}
		}
	}
```

Add the project import to `cmd/inject.go`:
```go
	"github.com/SAP-samples/sap-devs-cli/internal/project"
```

- [ ] **Step 7: Fix compilation — remove or update references to `ProjectType`**

Search the codebase for all remaining references to `ProjectType` and `detectProjectType`. Update them:
- `internal/dynamic/gather.go`: Remove the `detectProjectType()` and `hasSAPCDS()` functions (they are superseded by `internal/project`)
- Any tests referencing `ProjectType` should be updated to use `Project`

- [ ] **Step 8: Run tests and verify build**

Run: `go build ./... && go vet ./...`
Then: `go test ./internal/content/... -v` and `go test ./internal/project/... -v`
Expected: Clean build, all tests pass.

- [ ] **Step 9: Commit**

```bash
git add internal/content/dynamic.go internal/content/render.go internal/dynamic/gather.go cmd/inject.go
git commit -m "feat: integrate project detection into inject pipeline"
```

---

### Task 6: Enhance `cmd/doctor.go` with project health checks

**Files:**
- Modify: `cmd/doctor.go`
- Modify: `internal/i18n/catalogs/en.json`
- Modify: `internal/i18n/catalogs/de.json` (placeholder keys)

- [ ] **Step 1: Add i18n keys for project health output**

In `internal/i18n/catalogs/en.json`, add after the existing `doctor.status_missing` entry (after line 69):

```json
  "doctor.project.header": "Project Health",
  "doctor.project.no_project": "No project detected in current directory.",
  "doctor.project.no_findings": "No issues found.",
  "doctor.project.severity_error": "ERROR",
  "doctor.project.severity_warning": "WARNING",
  "doctor.project.severity_info": "INFO",
  "doctor.project.fix_prefix": "Fix:",
```

In `internal/i18n/catalogs/de.json`, add corresponding placeholder keys (English values are fine as placeholders):

```json
  "doctor.project.header": "Projektgesundheit",
  "doctor.project.no_project": "Kein Projekt im aktuellen Verzeichnis erkannt.",
  "doctor.project.no_findings": "Keine Probleme gefunden.",
  "doctor.project.severity_error": "FEHLER",
  "doctor.project.severity_warning": "WARNUNG",
  "doctor.project.severity_info": "INFO",
  "doctor.project.fix_prefix": "Fix:",
```

- [ ] **Step 2: Add `--tools-only` and `--project-only` flags to doctor**

In `cmd/doctor.go`, add flag variables near the top (after line 20):

```go
var doctorToolsOnly bool
var doctorProjectOnly bool
```

In `init()` (at line 183), add the new flags:

```go
	doctorCmd.Flags().BoolVar(&doctorToolsOnly, "tools-only", false, "check tool versions only (skip project health)")
	doctorCmd.Flags().BoolVar(&doctorProjectOnly, "project-only", false, "check project health only (skip tool versions)")
```

- [ ] **Step 3: Update the `RunE` function to orchestrate both checks**

Replace the body of `RunE` in `cmd/doctor.go` to include project health after tool checks:

After the existing tool-check block (after line 97's error check), add project health check logic:

```go
		// Project health checks
		if !doctorToolsOnly {
			cwd, _ := os.Getwd()
			pc, err := project.Detect(cwd)
			if err == nil && pc.Type != "" {
				findings := project.Check(pc, cwd, packs)
				fmt.Fprintln(cmd.OutOrStdout())
				printProjectHealth(cmd.OutOrStdout(), pc, findings, i18n.ActiveLang)
				for _, f := range findings {
					if f.Severity == "error" {
						return fmt.Errorf("one or more project health checks failed")
					}
				}
			}
		}
```

Guard the existing tool checks with `!doctorProjectOnly`: wrap the tool-collection, `CheckTools`, `printDoctorTable`, and `printInstallCommands` calls in `if !doctorProjectOnly { ... }`.

Add imports to `cmd/doctor.go`:
```go
	"io"
	"os"

	"github.com/SAP-samples/sap-devs-cli/internal/project"
```

Note: `"strings"` and `"fmt"` are already imported.

- [ ] **Step 4: Implement `printProjectHealth()`**

Add to `cmd/doctor.go`:

```go
func printProjectHealth(w io.Writer, pc *project.ProjectContext, findings []project.Finding, lang string) {
	header := fmt.Sprintf("%s (%s)", i18n.T(lang, "doctor.project.header"), pc.Type)
	if pc.Deployment != "" {
		deployLabel := pc.Deployment
		if pc.Deployment == "mta-cf" {
			deployLabel = "MTA to Cloud Foundry"
		} else if pc.Deployment == "helm-kyma" {
			deployLabel = "Helm to Kyma"
		}
		header += " — " + deployLabel
	}
	fmt.Fprintln(w, header)
	fmt.Fprintln(w, strings.Repeat("─", 55))

	if len(findings) == 0 {
		fmt.Fprintln(w, i18n.T(lang, "doctor.project.no_findings"))
		return
	}

	for _, f := range findings {
		var icon, sevLabel string
		switch f.Severity {
		case "error":
			icon = "✗"
			sevLabel = i18n.T(lang, "doctor.project.severity_error")
		case "warning":
			icon = "⚠"
			sevLabel = i18n.T(lang, "doctor.project.severity_warning")
		default:
			icon = "ℹ"
			sevLabel = i18n.T(lang, "doctor.project.severity_info")
		}
		fmt.Fprintf(w, "%s %-8s %s\n", icon, sevLabel, f.Message)
		if f.Fix != "" {
			fmt.Fprintf(w, "           %s %s\n", i18n.T(lang, "doctor.project.fix_prefix"), f.Fix)
		}
	}
}
```

Add `"io"` to the imports.

- [ ] **Step 5: Verify build**

Run: `go build ./... && go vet ./...`
Expected: Clean.

- [ ] **Step 6: Commit**

```bash
git add cmd/doctor.go internal/i18n/catalogs/en.json internal/i18n/catalogs/de.json
git commit -m "feat(doctor): add project health checks with --tools-only and --project-only flags"
```

---

### Task 7: Update CLAUDE.md and documentation

**Files:**
- Modify: `CLAUDE.md` — update Architecture Overview and CLI Commands table
- Modify: `TODO.md` — mark features as completed

- [ ] **Step 1: Update CLAUDE.md Architecture section**

Add a new subsection after the "### CLI Commands" table in `CLAUDE.md`:

```markdown
### Project Detection & Health Check

`internal/project` ([internal/project/detect.go](internal/project/detect.go), [internal/project/check.go](internal/project/check.go)) provides two entry points:

- `Detect(cwd)` scans project files (package.json, .mta.yaml, xs-security.json, etc.) and returns a `ProjectContext` with typed fields (CAPVersion, Database, Deployment, Auth) and a `Facts` slice for flexible rendering.
- `Check(ctx, cwd, packs)` runs health checks (dependency, version staleness, best-practice, constraint compliance) and returns `[]Finding` with severity/fix.

Both are consumed by `cmd/inject.go` (project context injected into AI tools) and `cmd/doctor.go` (health check table output).
```

Update the `doctor` row in the CLI Commands table to reflect the new capabilities:

```
| `doctor` | Check tool versions and project health (`--tools-only`, `--project-only`, `--fix` for install/fix hints) |
```

- [ ] **Step 2: Update TODO.md**

Mark the "Project-aware context detection on inject" section as completed (add a ✅ or move to a "Completed" section per the project's convention).

- [ ] **Step 3: Verify build one final time**

Run: `go build ./... && go vet ./...`
Expected: Clean.

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md TODO.md
git commit -m "docs: update CLAUDE.md and TODO.md for project detection feature"
```

---

### Task 8: End-to-end manual verification

**Files:** None (verification only)

- [ ] **Step 1: Test doctor in a CAP project directory**

```bash
cd /d/projects/cloud-cap-hana-swapi/cap
/d/projects/sap-devs-cli/sap-devs doctor
```

Expected: Tool Versions table followed by Project Health section showing CAP version, deployment, and any findings.

- [ ] **Step 2: Test doctor --project-only**

```bash
/d/projects/sap-devs-cli/sap-devs doctor --project-only
```

Expected: Only the Project Health section, no tool versions.

- [ ] **Step 3: Test doctor --tools-only**

```bash
/d/projects/sap-devs-cli/sap-devs doctor --tools-only
```

Expected: Only tool versions, no project health section.

- [ ] **Step 4: Test inject --dry-run to see project context**

```bash
cd /d/projects/cloud-cap-hana-swapi/cap
SAP_DEVS_DEV=1 /d/projects/sap-devs-cli/sap-devs inject --dry-run
```

Expected: Injected output includes "**Project Context (detected):**" with facts about the CAP project.

- [ ] **Step 5: Test in an empty directory**

```bash
cd /tmp
/d/projects/sap-devs-cli/sap-devs doctor
```

Expected: Tool Versions table only, no Project Health section.

- [ ] **Step 6: Build binary for final validation**

```bash
cd /d/projects/sap-devs-cli
VERSION=$(git describe --tags --always --dirty)
go build -ldflags "-X github.com/SAP-samples/sap-devs-cli/cmd.Version=${VERSION}" -o sap-devs .
```

Expected: Clean build.
