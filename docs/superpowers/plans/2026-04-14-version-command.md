# Version Command Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enhance `sap-devs version` to explain dev builds and add a `--verbose/-v` flag showing Go version, OS, and arch.

**Architecture:** All changes are confined to `cmd/version.go`. A package-level `verbose bool` variable is bound to a local `--verbose/-v` cobra flag. The `Run` closure writes via `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` (cobra's output writer API, consistent with the rest of the codebase). This lets tests capture stdout and stderr independently using cobra's `SetOut`/`SetErr`.

**Tech Stack:** Go stdlib (`fmt`, `os`, `runtime`), cobra flag API (`BoolVarP`), testify

---

## File Map

| Action | Path | Responsibility |
|--------|------|---------------|
| Modify | `cmd/version.go` | Version variable, flag declaration, Run logic |
| Create | `cmd/version_test.go` | Table-driven tests for default and verbose output |

---

### Task 1: Write failing tests for the version command

**Files:**
- Create: `cmd/version_test.go`

The existing test helpers live in `cmd/config_token_test.go` (same `cmd_test` package):

- `executeCommand(root, args...)` — runs cobra, returns combined output, resets flags
- `resetFlags(cmd)` — recursively resets all cobra flags to defaults

For version tests we need stdout and stderr captured **separately** (to assert the dev hint is on stderr, not stdout). Define a local `executeVersionCommand` helper in `version_test.go` that sets different buffers. `resetFlags` from `config_token_test.go` is accessible because both files share the `cmd_test` package.

The tests also need to temporarily override `Version`. Expose `GetVersion()`/`SetVersion()` helpers from `cmd/version.go` (added in Task 2).

- [ ] **Step 1: Create `cmd/version_test.go`**

```go
// cmd/version_test.go
package cmd_test

import (
    "bytes"
    "runtime"
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/SAP-samples/sap-devs-cli/cmd"
)

// executeVersionCommand runs the version command with extra args and returns
// (stdout, stderr) as separate strings. It uses cobra's SetOut/SetErr so the
// implementation must write via cmd.OutOrStdout() / cmd.ErrOrStderr().
func executeVersionCommand(t *testing.T, args ...string) (stdout, stderr string) {
    t.Helper()
    root := cmd.RootCmd()
    bufOut := new(bytes.Buffer)
    bufErr := new(bytes.Buffer)
    root.SetOut(bufOut)
    root.SetErr(bufErr)
    root.SetArgs(append([]string{"version"}, args...))
    resetFlags(root) // defined in config_token_test.go, same package
    root.Execute()   //nolint:errcheck
    return bufOut.String(), bufErr.String()
}

func TestVersionDefault_DevBuild(t *testing.T) {
    stdout, stderr := executeVersionCommand(t)
    assert.Equal(t, "dev\n", stdout)
    assert.Contains(t, stderr, "dev build")
    assert.Contains(t, stderr, "auto-update is disabled")
}

func TestVersionDefault_RealBuild(t *testing.T) {
    original := cmd.GetVersion()
    cmd.SetVersion("v1.2.3")
    defer cmd.SetVersion(original)

    stdout, stderr := executeVersionCommand(t)
    assert.Equal(t, "v1.2.3\n", stdout)
    assert.Empty(t, stderr, "no hint expected for real builds")
}

func TestVersionVerbose_DevBuild(t *testing.T) {
    stdout, stderr := executeVersionCommand(t, "--verbose")
    assert.Contains(t, stdout, "sap-devs dev")
    assert.Contains(t, stdout, "go:")
    assert.Contains(t, stdout, "os/arch:")
    assert.Contains(t, stderr, "dev build")
    assert.Contains(t, stderr, "auto-update is disabled")
}

func TestVersionVerbose_RealBuild(t *testing.T) {
    original := cmd.GetVersion()
    cmd.SetVersion("v1.2.3")
    defer cmd.SetVersion(original)

    stdout, stderr := executeVersionCommand(t, "--verbose")
    assert.Contains(t, stdout, "sap-devs v1.2.3")
    assert.Contains(t, stdout, "go:")
    assert.Contains(t, stdout, "os/arch:")
    assert.Empty(t, stderr)
}

func TestVersionShortFlag(t *testing.T) {
    stdout, _ := executeVersionCommand(t, "-v")
    assert.True(t, strings.Contains(stdout, "go:"), "-v short flag should produce verbose output")
}

func TestVersionVerbose_GoAndOSPresent(t *testing.T) {
    stdout, _ := executeVersionCommand(t, "--verbose")
    assert.Contains(t, stdout, runtime.Version())
    assert.Contains(t, stdout, runtime.GOOS)
    assert.Contains(t, stdout, runtime.GOARCH)
}
```

- [ ] **Step 2: Verify the test file does not compile yet**

```bash
go build ./cmd/...
```

Expected: compile error — `cmd.GetVersion` and `cmd.SetVersion` are undefined.

---

### Task 2: Implement the enhanced version command

**Files:**

- Modify: `cmd/version.go`

The key implementation constraint: use `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` (not `fmt.Print*` / `os.Stderr` directly) so cobra's output redirection works in tests.

- [ ] **Step 1: Replace `cmd/version.go` with the new implementation**

```go
package cmd

import (
    "fmt"
    "runtime"

    "github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// verbose is bound to the --verbose / -v flag on versionCmd.
var verbose bool

// GetVersion returns the current Version (used in tests).
func GetVersion() string { return Version }

// SetVersion sets Version (used in tests to simulate real builds).
func SetVersion(v string) { Version = v }

var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Print the sap-devs version",
    Run: func(cmd *cobra.Command, args []string) {
        out := cmd.OutOrStdout()
        errOut := cmd.ErrOrStderr()

        if verbose {
            fmt.Fprintf(out, "sap-devs %s\n", Version)
            fmt.Fprintf(out, "  go:      %s\n", runtime.Version())
            fmt.Fprintf(out, "  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
        } else {
            fmt.Fprintln(out, Version)
        }
        if Version == "dev" {
            fmt.Fprintln(errOut, "(dev build: built without -ldflags version injection — auto-update is disabled)")
        }
    },
}

func init() {
    versionCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Print Go version, OS, and architecture")
    rootCmd.AddCommand(versionCmd)
}
```

- [ ] **Step 2: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: exits 0, no output.

- [ ] **Step 3: Run vet**

```bash
go vet ./...
```

Expected: exits 0, no output.

- [ ] **Step 4: Run the version tests**

```bash
go test ./cmd/... -run TestVersion -v
```

Expected: all `TestVersion*` tests pass.

> **Windows note:** Windows Defender blocks test binary execution from `~/.config` paths. If tests fail to run, skip — CI on ubuntu-latest is authoritative. Use `go build ./...` + `go vet ./...` as the local verification signal.

- [ ] **Step 5: Manual smoke test — dev build**

```bash
go run . version
go run . version --verbose
go run . version -v
```

Expected for `version` (stdout):

```text
dev
```

Expected stderr:

```text
(dev build: built without -ldflags version injection — auto-update is disabled)
```

Expected for `version --verbose` (stdout):

```text
sap-devs dev
  go:      go1.xx.x
  os/arch: <your-os>/<your-arch>
```

Same hint on stderr.

- [ ] **Step 6: Manual smoke test — versioned build**

```powershell
$VERSION = git describe --tags --always --dirty
go build -ldflags "-X github.com/SAP-samples/sap-devs-cli/cmd.Version=$VERSION" -o sap-devs.exe .
.\sap-devs.exe version
.\sap-devs.exe version --verbose
```

Expected for `version` (stdout only, no stderr):

```text
v0.0.1-132-g666c39c
```

Expected for `version --verbose` (stdout only, no stderr):

```text
sap-devs v0.0.1-132-g666c39c
  go:      go1.xx.x
  os/arch: <your-os>/<your-arch>
```

- [ ] **Step 7: Commit**

```bash
git add cmd/version.go cmd/version_test.go
git commit -m "feat: enhance version command with dev hint and --verbose flag"
```
