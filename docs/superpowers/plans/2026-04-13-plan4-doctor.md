# sap-devs doctor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs doctor` to check locally installed tool versions against pack requirements, with `--profile` filtering and `--fix` to print install commands, exiting 1 on any failure.

**Architecture:** Add pure helper functions (`compareVersions`, `parseConstraint`, `CheckTool`, `CheckTools`) in a new `internal/content/doctor.go` following the same pattern as `internal/content/resources.go`. Wire them into a thin `cmd/doctor.go` Cobra command. No new packages or dependencies — version comparison uses only `strings` and `strconv`.

**Tech Stack:** Go 1.26, github.com/spf13/cobra (existing), gopkg.in/yaml.v3 (existing), github.com/stretchr/testify (existing)

> **Note on local testing:** `go test` is blocked by Windows Defender on this machine (test binaries land in `%TEMP%` on C:). Use `go build ./...` + `go vet ./...` for local verification. Tests run correctly in CI (`go test ./...` on ubuntu-latest).

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/content/doctor.go` | Create | `CheckStatus`, `ToolResult`, `Runner`, `CheckTool`, `CheckTools`, `parseConstraint`, `compareVersions` |
| `internal/content/doctor_test.go` | Create | Unit tests for all helpers using a fake Runner |
| `cmd/doctor.go` | Create | `doctor` Cobra command with `--profile` and `--fix` flags |

---

## Task 1: Add version comparison helpers (TDD)

**Files:**
- Create: `internal/content/doctor_test.go`
- Create: `internal/content/doctor.go`

The spec defines two unexported helpers: `compareVersions` and `parseConstraint`. Write tests first, then implement.

### Step 1 — Write the tests

- [ ] **Step 1: Create `internal/content/doctor_test.go` with version comparison tests**

```go
package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestParseConstraint_GTE_Satisfied(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest(">=18.0.0", "18.0.0"))
	assert.True(t, content.ParseConstraintForTest(">=18.0.0", "v20.11.0"))
	assert.True(t, content.ParseConstraintForTest(">=18.0.0", "18.0.1"))
}

func TestParseConstraint_GTE_NotSatisfied(t *testing.T) {
	assert.False(t, content.ParseConstraintForTest(">=18.0.0", "17.9.9"))
	assert.False(t, content.ParseConstraintForTest(">=18.0.0", "v17.0.0"))
}

func TestParseConstraint_GT(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest(">18.0.0", "18.0.1"))
	assert.False(t, content.ParseConstraintForTest(">18.0.0", "18.0.0"))
}

func TestParseConstraint_LTE(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest("<=18.0.0", "18.0.0"))
	assert.True(t, content.ParseConstraintForTest("<=18.0.0", "17.9.9"))
	assert.False(t, content.ParseConstraintForTest("<=18.0.0", "18.0.1"))
}

func TestParseConstraint_LT(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest("<18.0.0", "17.9.9"))
	assert.False(t, content.ParseConstraintForTest("<18.0.0", "18.0.0"))
}

func TestParseConstraint_EQ(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest("=18.0.0", "18.0.0"))
	assert.False(t, content.ParseConstraintForTest("=18.0.0", "18.0.1"))
}

func TestParseConstraint_PartialVersion(t *testing.T) {
	// ">=8" is zero-padded to ">=8.0.0"
	assert.True(t, content.ParseConstraintForTest(">=8", "8.0.0"))
	assert.True(t, content.ParseConstraintForTest(">=8", "8.1.0"))
	assert.False(t, content.ParseConstraintForTest(">=8", "7.9.9"))
}

func TestParseConstraint_VersionWithSuffix(t *testing.T) {
	// Trailing non-digit characters are stripped per segment
	assert.True(t, content.ParseConstraintForTest(">=7.0.0", "7.9.3 (release)"))
	assert.True(t, content.ParseConstraintForTest(">=18.0.0", "v20.11.0-alpine3.19"))
}

func TestParseConstraint_UnrecognisedOperator(t *testing.T) {
	// No operator prefix → false
	assert.False(t, content.ParseConstraintForTest("18.0.0", "18.0.0"))
}

func TestParseConstraint_UnparsableFound(t *testing.T) {
	// Cannot parse found version → false
	assert.False(t, content.ParseConstraintForTest(">=18.0.0", "not-a-version"))
}
```

> **Note:** `parseConstraint` and `compareVersions` are unexported. Expose them for testing via a
> thin exported wrapper `ParseConstraintForTest` in `doctor.go`. This is the standard Go pattern
> for testing unexported helpers without a separate `_test` package workaround.

- [ ] **Step 2: Verify tests fail to compile (functions not yet defined)**

Run: `go build ./...`
Expected: compile error — `content.ParseConstraintForTest` undefined.

### Step 2 — Implement the helpers

- [ ] **Step 3: Create `internal/content/doctor.go` with the version helpers**

```go
package content

import (
	"strconv"
	"strings"
)

// compareVersions compares two version strings of exactly three dot-separated
// integer segments and returns -1, 0, or 1. Each segment has any trailing
// non-digit characters stripped before parsing. Both inputs must already be
// zero-padded to three components by the caller.
func compareVersions(a, b string) int {
	aParts := strings.SplitN(a, ".", 3)
	bParts := strings.SplitN(b, ".", 3)
	for i := 0; i < 3; i++ {
		av := parseSegment(aParts[i])
		bv := parseSegment(bParts[i])
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

// parseSegment strips trailing non-digit characters from a version segment
// and returns its integer value, or 0 if unparseable.
func parseSegment(s string) int {
	// Trim from first non-digit after at least one digit (handles "0-alpine3.19", "7 (release)")
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}

// padVersion zero-pads a version string to exactly three dot-separated components.
// "8" → "8.0.0", "8.1" → "8.1.0", "8.1.2" → "8.1.2".
func padVersion(v string) string {
	parts := strings.Split(v, ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	return strings.Join(parts[:3], ".")
}

// parseConstraint parses a required string of the form ">=1.2.3", ">1.2.3",
// "=1.2.3", "<=1.2.3", or "<1.2.3" and compares it against found.
// Both version strings have a leading "v" stripped and are zero-padded to
// three components before comparison. Returns false if the operator is not
// recognised or either version cannot be usefully parsed.
func parseConstraint(required, found string) bool {
	var op, reqVer string
	switch {
	case strings.HasPrefix(required, ">="):
		op, reqVer = ">=", required[2:]
	case strings.HasPrefix(required, ">"):
		op, reqVer = ">", required[1:]
	case strings.HasPrefix(required, "<="):
		op, reqVer = "<=", required[2:]
	case strings.HasPrefix(required, "<"):
		op, reqVer = "<", required[1:]
	case strings.HasPrefix(required, "="):
		op, reqVer = "=", required[1:]
	default:
		return false
	}

	// Normalise: strip leading "v", zero-pad to three components
	reqVer = padVersion(strings.TrimPrefix(strings.TrimSpace(reqVer), "v"))
	foundNorm := strings.TrimPrefix(strings.TrimSpace(found), "v")

	// Guard: if found doesn't start with a digit it cannot be parsed — return false.
	if len(foundNorm) == 0 || foundNorm[0] < '0' || foundNorm[0] > '9' {
		return false
	}
	foundVer := padVersion(foundNorm)

	cmp := compareVersions(foundVer, reqVer)
	switch op {
	case ">=":
		return cmp >= 0
	case ">":
		return cmp > 0
	case "<=":
		return cmp <= 0
	case "<":
		return cmp < 0
	case "=":
		return cmp == 0
	}
	return false
}

// ParseConstraintForTest exposes parseConstraint for use in external test packages.
func ParseConstraintForTest(required, found string) bool {
	return parseConstraint(required, found)
}
```

- [ ] **Step 4: Verify it builds**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 5: Verify vet**

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 6: Commit**

```bash
git add internal/content/doctor.go internal/content/doctor_test.go
git commit -m "feat: add compareVersions and parseConstraint helpers"
```

---

## Task 2: Add CheckTool and CheckTools (TDD)

**Files:**
- Modify: `internal/content/doctor_test.go`
- Modify: `internal/content/doctor.go`

### Step 1 — Write the tests

- [ ] **Step 1: Append CheckTool and CheckTools tests to `internal/content/doctor_test.go`**

Add after the existing `TestParseConstraint_*` tests:

```go
func fakeRunner(output string, err error) content.Runner {
	return func(command string) (string, error) {
		return output, err
	}
}

func toolDef(id, required, command, pattern string) content.ToolDef {
	return content.ToolDef{
		ID:       id,
		Name:     id,
		Required: required,
		Detect: content.ToolDetect{
			Command: command,
			Pattern: pattern,
		},
	}
}

func TestCheckTool_OK(t *testing.T) {
	tool := toolDef("node", ">=18.0.0", "node --version", `v(\d+\.\d+\.\d+)`)
	result := content.CheckTool(tool, fakeRunner("v20.11.0", nil))
	assert.Equal(t, content.StatusOK, result.Status)
	assert.Equal(t, "v20.11.0", result.Found)
}

func TestCheckTool_Fail(t *testing.T) {
	tool := toolDef("cds", ">=7.0.0", "cds --version", `@sap/cds: (\d+\.\d+\.\d+)`)
	result := content.CheckTool(tool, fakeRunner("@sap/cds: 6.8.2\n", nil))
	assert.Equal(t, content.StatusFail, result.Status)
	assert.Equal(t, "6.8.2", result.Found)
}

func TestCheckTool_Missing_RunnerError(t *testing.T) {
	tool := toolDef("cf", ">=8.0.0", "cf --version", `cf version (\d+\.\d+\.\d+)`)
	result := content.CheckTool(tool, fakeRunner("", fmt.Errorf("not found")))
	assert.Equal(t, content.StatusMissing, result.Status)
	assert.Empty(t, result.Found)
}

func TestCheckTool_PatternNoMatch(t *testing.T) {
	tool := toolDef("cf", ">=8.0.0", "cf --version", `cf version (\d+\.\d+\.\d+)`)
	result := content.CheckTool(tool, fakeRunner("some unrelated output", nil))
	assert.Equal(t, content.StatusMissing, result.Status)
	assert.Empty(t, result.Found)
}

func TestCheckTool_Latest_Present(t *testing.T) {
	tool := toolDef("btp", "latest", "btp --version", `SAP BTP command line interface \(client v(\S+)\)`)
	result := content.CheckTool(tool, fakeRunner("SAP BTP command line interface (client v3.65.0)\n", nil))
	assert.Equal(t, content.StatusUnknown, result.Status)
	assert.Equal(t, "3.65.0", result.Found)
}

func TestCheckTool_Latest_Missing(t *testing.T) {
	tool := toolDef("btp", "latest", "btp --version", `SAP BTP command line interface \(client v(\S+)\)`)
	result := content.CheckTool(tool, fakeRunner("", fmt.Errorf("not found")))
	assert.Equal(t, content.StatusMissing, result.Status)
}

func TestCheckTools_Dedup(t *testing.T) {
	callCount := 0
	countingRunner := func(command string) (string, error) {
		callCount++
		return "v20.11.0", nil
	}
	tools := []content.ToolDef{
		toolDef("node", ">=18.0.0", "node --version", `v(\d+\.\d+\.\d+)`),
		toolDef("node", ">=18.0.0", "node --version", `v(\d+\.\d+\.\d+)`), // duplicate
	}
	results := content.CheckTools(tools, countingRunner)
	assert.Len(t, results, 1)
	assert.Equal(t, 1, callCount)
}
```

Also add `"fmt"` to the import block at the top of the test file (required for `fmt.Errorf`):

```go
import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)
```

- [ ] **Step 2: Verify tests fail to compile**

Run: `go build ./...`
Expected: compile error — `content.CheckTool`, `content.CheckTools`, `content.StatusOK` etc. undefined.

### Step 2 — Implement CheckTool and CheckTools

- [ ] **Step 3: Append CheckTool and CheckTools to `internal/content/doctor.go`**

Add after the `ParseConstraintForTest` function:

```go
// CheckStatus represents the result of a tool version check.
type CheckStatus string

const (
	StatusOK      CheckStatus = "ok"
	StatusFail    CheckStatus = "fail"
	StatusMissing CheckStatus = "missing"
	StatusUnknown CheckStatus = "unknown" // required is "latest"
)

// ToolResult holds the outcome of checking a single tool.
type ToolResult struct {
	Tool   ToolDef
	Status CheckStatus
	Found  string // raw captured version string, empty if missing
}

// Runner abstracts exec.Command for testability.
// It receives the full command string (e.g. "node --version") and returns
// the combined stdout+stderr output.
type Runner func(command string) (string, error)

// CheckTool runs the tool's detect command via run, extracts the version using
// the tool's regex pattern, and compares it against tool.Required.
func CheckTool(tool ToolDef, run Runner) ToolResult {
	output, err := run(tool.Detect.Command)
	if err != nil {
		return ToolResult{Tool: tool, Status: StatusMissing}
	}

	re, err := regexp.Compile(tool.Detect.Pattern)
	if err != nil {
		return ToolResult{Tool: tool, Status: StatusMissing}
	}
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return ToolResult{Tool: tool, Status: StatusMissing}
	}
	found := matches[1]

	if tool.Required == "latest" {
		return ToolResult{Tool: tool, Status: StatusUnknown, Found: found}
	}

	if parseConstraint(tool.Required, found) {
		return ToolResult{Tool: tool, Status: StatusOK, Found: found}
	}
	return ToolResult{Tool: tool, Status: StatusFail, Found: found}
}

// CheckTools runs CheckTool for each tool, deduplicating by ID (first seen wins).
func CheckTools(tools []ToolDef, run Runner) []ToolResult {
	seen := make(map[string]bool)
	var results []ToolResult
	for _, t := range tools {
		if seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		results = append(results, CheckTool(t, run))
	}
	return results
}
```

Also add `"regexp"` to the imports in `doctor.go`. The full import block should be:

```go
import (
	"regexp"
	"strconv"
	"strings"
)
```

- [ ] **Step 4: Verify it builds**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 5: Verify vet**

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 6: Commit**

```bash
git add internal/content/doctor.go internal/content/doctor_test.go
git commit -m "feat: add CheckTool and CheckTools with Runner abstraction"
```

---

## Task 3: Create cmd/doctor.go

**Files:**
- Create: `cmd/doctor.go`

Study `cmd/resources.go` for the Cobra subcommand pattern and `cmd/root.go` for `newContentLoader`. The module path is `github.tools.sap/developer-relations/sap-devs-cli`.

- [ ] **Step 1: Create `cmd/doctor.go`**

```go
package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

// profileActive is the sentinel value for --profile that means "use the configured profile".
const profileActive = "@active"

var doctorProfile string
var doctorFix bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check local tool versions against pack requirements",
	RunE: func(cmd *cobra.Command, args []string) error {
		loader, err := newContentLoader()
		if err != nil {
			return err
		}

		var packs []*content.Pack
		switch doctorProfile {
		case "":
			packs, err = loader.LoadPacks(nil)
			if err != nil {
				return err
			}
		case profileActive:
			paths, err := xdg.New()
			if err != nil {
				return err
			}
			profileCfg, err := config.LoadProfile(paths.ConfigDir)
			if err != nil {
				return err
			}
			if profileCfg.ID == "" {
				return fmt.Errorf("no profile set — run 'sap-devs profile set <name>' first")
			}
			active, err := loader.FindProfile(profileCfg.ID)
			if err != nil {
				return err
			}
			if active == nil {
				return fmt.Errorf("profile %q not found — run 'sap-devs sync' to refresh content", profileCfg.ID)
			}
			packs, err = loader.LoadPacks(active)
			if err != nil {
				return err
			}
		default:
			p, err := loader.FindProfile(doctorProfile)
			if err != nil {
				return err
			}
			if p == nil {
				return fmt.Errorf("profile %q not found — run 'sap-devs sync' to refresh content", doctorProfile)
			}
			packs, err = loader.LoadPacks(p)
			if err != nil {
				return err
			}
		}

		// Collect all tools across packs
		var tools []content.ToolDef
		for _, p := range packs {
			tools = append(tools, p.Tools...)
		}

		if len(tools) == 0 {
			fmt.Println("No tools defined for the current selection.")
			return nil
		}

		results := content.CheckTools(tools, execRunner)
		printDoctorTable(results)

		if doctorFix {
			printInstallCommands(results)
		}

		for _, r := range results {
			if r.Status == content.StatusFail || r.Status == content.StatusMissing {
				return fmt.Errorf("one or more tools failed the version check")
			}
		}
		return nil
	},
}

// execRunner runs a command string using exec.Command and returns combined output.
func execRunner(command string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	out, err := exec.Command(parts[0], parts[1:]...).CombinedOutput()
	return string(out), err
}

// printDoctorTable prints an aligned table of tool check results.
func printDoctorTable(results []content.ToolResult) {
	fmt.Printf("%-20s %-12s %-12s %s\n", "TOOL", "REQUIRED", "FOUND", "STATUS")
	fmt.Println(strings.Repeat("-", 62))
	for _, r := range results {
		found := r.Found
		if found == "" {
			found = "-"
		}
		status := statusLabel(r.Status)
		fmt.Printf("%-20s %-12s %-12s %s\n", r.Tool.ID, r.Tool.Required, found, status)
	}
}

func statusLabel(s content.CheckStatus) string {
	switch s {
	case content.StatusOK:
		return "ok"
	case content.StatusUnknown:
		return "ok (unverified)"
	case content.StatusFail:
		return "FAIL"
	case content.StatusMissing:
		return "MISSING"
	}
	return string(s)
}

// printInstallCommands prints install hints for failed/missing tools.
func printInstallCommands(results []content.ToolResult) {
	var toFix []content.ToolResult
	for _, r := range results {
		if r.Status == content.StatusFail || r.Status == content.StatusMissing {
			toFix = append(toFix, r)
		}
	}
	if len(toFix) == 0 {
		return
	}
	fmt.Println("\nInstall commands:")
	for _, r := range toFix {
		cmd := installCommand(r.Tool)
		fmt.Printf("  %-20s %s\n", r.Tool.ID, cmd)
	}
}

// installCommand returns the best install command for the current OS,
// falling back to "all", then to the docs URL.
func installCommand(tool content.ToolDef) string {
	osKey := map[string]string{
		"windows": "windows",
		"darwin":  "macos",
		"linux":   "linux",
	}[runtime.GOOS]

	if cmd, ok := tool.Install[osKey]; ok && cmd != "" {
		return cmd
	}
	if cmd, ok := tool.Install["all"]; ok && cmd != "" {
		return cmd
	}
	if tool.Docs != "" {
		return "see: " + tool.Docs
	}
	return "no install command available"
}

func init() {
	doctorCmd.Flags().StringVar(&doctorProfile, "profile", "", `profile to check ("@active" for configured profile, or a profile ID)`)
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "print install commands for failed or missing tools")
	rootCmd.AddCommand(doctorCmd)
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 3: Verify vet**

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 4: Commit**

```bash
git add cmd/doctor.go
git commit -m "feat: add doctor command with --profile and --fix flags"
```

---

## Task 4: Final Verification

- [ ] **Step 1: Full build**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 2: Full vet**

Run: `go vet ./...`
Expected: clean.

- [ ] **Step 3: Build the binary**

Run: `go build -o sap-devs.exe .`
Expected: binary produced.

- [ ] **Step 4: Smoke test — no tools (empty content cache)**

Run: `./sap-devs.exe doctor`
Expected: either `"No tools defined for the current selection."` or a table of results if content is cached.

- [ ] **Step 5: Smoke test — unknown profile**

Run: `./sap-devs.exe doctor --profile nonexistent`
Expected: `Error: profile "nonexistent" not found — run 'sap-devs sync' to refresh content`

- [ ] **Step 6: Smoke test — @active with no profile set**

Run: `./sap-devs.exe doctor --profile @active`
Expected: `Error: no profile set — run 'sap-devs profile set <name>' first`

- [ ] **Step 7: Smoke test — help output**

Run: `./sap-devs.exe doctor --help`
Expected: usage shown with `--profile` and `--fix` flags listed.

- [ ] **Step 8: Clean up binary and commit**

```bash
rm -f sap-devs.exe
git add -A
git commit -m "chore: plan4 doctor complete"
```
