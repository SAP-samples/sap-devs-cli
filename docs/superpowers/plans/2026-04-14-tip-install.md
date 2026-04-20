# tip install / tip uninstall Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs tip install` and `sap-devs tip uninstall` subcommands that add/remove `sap-devs tip` from the user's shell profiles, backed by a new reusable `internal/shellhook` package.

**Architecture:** Create `internal/shellhook` with `Add`/`Remove` public functions that delegate to unexported `addToProfiles`/`removeFromProfiles` helpers. Tests call the helpers directly with explicit paths, avoiding any `runtime.GOOS` dependency at test time. `cmd/tip.go` gets two new Cobra subcommands; `cmd/init.go` is refactored to call `shellhook.Add` in place of its own `addShellHook`.

**Tech Stack:** Go standard library (`bufio`, `os`, `errors`, `strings`), `cobra` (existing CLI framework)

---

## Notes for the Implementer

- **Windows:** `go test` always fails locally (Windows Defender blocks test binary execution). Use `go build ./...` + `go vet ./...` locally. CI (ubuntu-latest GitHub Actions) is the authoritative test runner for `go test`.
- **Module path:** `github.com/SAP-samples/sap-devs-cli` — use this prefix for all internal imports.
- **Spec:** `docs/superpowers/specs/2026-04-14-tip-install-design.md`

---

## File Map

| Action | File | Responsibility |
| --- | --- | --- |
| Create | `internal/shellhook/shellhook.go` | Platform-aware profile detection; `Add` and `Remove` logic |
| Create | `internal/shellhook/shellhook_test.go` | Unit tests calling unexported helpers directly (no OS dependency) |
| Modify | `cmd/tip.go` | Register `install` and `uninstall` subcommands on `tipCmd` |
| Modify | `cmd/init.go` | Replace `addShellHook()` with `shellhook.Add(...)` |
| Modify | `docs/user/user-guide.md` | Document `tip install` / `tip uninstall` |

---

### Task 1: `internal/shellhook` — package scaffold and profile detection

**Files:**

- Create: `internal/shellhook/shellhook.go`
- Create: `internal/shellhook/shellhook_test.go`

- [ ] **Step 1: Create `internal/shellhook/shellhook.go`**

```go
package shellhook

import (
	"os"
	"path/filepath"
	"runtime"
)

// Result describes what happened to a single profile file.
type Result struct {
	Path    string
	Updated bool // false = already present (Add) or line not found (Remove)
}

// homeDir is a variable so tests can substitute a temp directory.
var homeDir = os.UserHomeDir

// candidateProfiles returns platform-appropriate shell profile paths
// rooted at homeDir(). Does not check whether paths exist.
func candidateProfiles() ([]string, error) {
	home, err := homeDir()
	if err != nil {
		return nil, err
	}
	return profilesForOS(runtime.GOOS, home), nil
}

// profilesForOS returns candidate profile paths for a given GOOS and home
// directory. Kept separate so tests can exercise any platform without
// running on it.
func profilesForOS(goos, home string) []string {
	if goos == "windows" {
		return []string{
			filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
			filepath.Join(home, ".bashrc"),
			filepath.Join(home, ".bash_profile"),
		}
	}
	return []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".zprofile"),
	}
}
```

- [ ] **Step 2: Create `internal/shellhook/shellhook_test.go`**

```go
package shellhook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

- [ ] **Step 3: Write and run tests for `profilesForOS`**

Append to `shellhook_test.go`:

```go
func TestProfilesForOS_Linux(t *testing.T) {
	profiles := profilesForOS("linux", "/home/user")
	want := []string{
		"/home/user/.zshrc",
		"/home/user/.bashrc",
		"/home/user/.bash_profile",
		"/home/user/.zprofile",
	}
	if len(profiles) != len(want) {
		t.Fatalf("got %v, want %v", profiles, want)
	}
	for i, p := range profiles {
		if p != want[i] {
			t.Errorf("[%d] got %q, want %q", i, p, want[i])
		}
	}
}

func TestProfilesForOS_Windows(t *testing.T) {
	profiles := profilesForOS("windows", `C:\Users\user`)
	if len(profiles) != 3 {
		t.Fatalf("expected 3 windows profiles, got %d: %v", len(profiles), profiles)
	}
	if !strings.Contains(profiles[0], "PowerShell") {
		t.Errorf("expected PowerShell profile first, got %q", profiles[0])
	}
}
```

- [ ] **Step 4: Build to verify compilation**

```bash
go build ./internal/shellhook/...
go vet ./internal/shellhook/...
```

Expected: no output, exit 0.

- [ ] **Step 5: Commit**

```bash
git add internal/shellhook/shellhook.go internal/shellhook/shellhook_test.go
git commit -m "feat: scaffold internal/shellhook with platform profile detection"
```

---

### Task 2: `shellhook.Add`

**Files:**
- Modify: `internal/shellhook/shellhook.go`
- Modify: `internal/shellhook/shellhook_test.go`

- [ ] **Step 1: Write failing tests for `addToProfiles`**

Tests call `addToProfiles` directly with explicit paths, so they are OS-independent.

Append to `shellhook_test.go`:

```go
func TestAdd_SingleProfileAbsent(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(rc, []byte("# existing\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || !results[0].Updated {
		t.Fatalf("expected one updated result, got %+v", results)
	}
	data, _ := os.ReadFile(rc)
	if !strings.Contains(string(data), "sap-devs tip") {
		t.Error("expected hook in profile")
	}
}

func TestAdd_LineAlreadyPresent(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(rc, []byte("# SAP developer tips\nsap-devs tip\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Updated {
		t.Fatalf("expected one skipped result, got %+v", results)
	}
}

func TestAdd_LineAsSubstringNotCounted(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	// "sap-devs tip" exists only as a substring — must NOT be treated as present
	if err := os.WriteFile(rc, []byte("# runs sap-devs tip daily\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || !results[0].Updated {
		t.Fatalf("expected append (substring should not count), got %+v", results)
	}
}

func TestAdd_SkipsWhenLinePresent_WithOrphanedComment(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	// file has a full hook block AND an orphaned comment — line is present, so skip
	content := "# SAP developer tips\nsap-devs tip\n# SAP developer tips\n"
	if err := os.WriteFile(rc, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Updated {
		t.Fatalf("expected skipped (line present), got %+v", results)
	}
}

func TestAdd_MultipleProfiles(t *testing.T) {
	dir := t.TempDir()
	zshrc := filepath.Join(dir, ".zshrc")
	bashrc := filepath.Join(dir, ".bashrc")
	for _, rc := range []string{zshrc, bashrc} {
		if err := os.WriteFile(rc, []byte("# existing\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{zshrc, bashrc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := 0
	for _, r := range results {
		if r.Updated {
			updated++
		}
	}
	if updated != 2 {
		t.Fatalf("expected 2 updated profiles, got %d (%+v)", updated, results)
	}
}

func TestAdd_NoProfiles(t *testing.T) {
	// pass an empty candidate list — simulates no profiles existing
	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{})
	if err == nil {
		t.Fatal("expected error when no profiles exist")
	}
	if len(results) != 0 {
		t.Fatalf("expected no results, got %+v", results)
	}
}

func TestAdd_WindowsPowerShellPath(t *testing.T) {
	dir := t.TempDir()
	psPath := filepath.Join(dir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	if err := os.MkdirAll(filepath.Dir(psPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(psPath, []byte("# existing\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{psPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || !results[0].Updated {
		t.Fatalf("expected updated result, got %+v", results)
	}
	data, _ := os.ReadFile(psPath)
	if !strings.Contains(string(data), "sap-devs tip") {
		t.Error("expected hook in PowerShell profile")
	}
}
```

- [ ] **Step 2: Implement `addToProfiles`, `Add`, and `hasLine` in `shellhook.go`**

Update the imports in `shellhook.go`:

```go
import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)
```

Append to `shellhook.go`:

```go
// Add appends comment and line to every existing profile that does not
// already contain line as a complete line. Returns one Result per
// candidate profile found on disk.
func Add(line, comment string) ([]Result, error) {
	candidates, err := candidateProfiles()
	if err != nil {
		return nil, err
	}
	return addToProfiles(line, comment, candidates)
}

// addToProfiles is the testable core of Add: it operates on an explicit
// list of paths rather than calling candidateProfiles().
func addToProfiles(line, comment string, candidates []string) ([]Result, error) {
	var results []Result
	var errs []error

	for _, path := range candidates {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		if hasLine(string(data), line) {
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		_, writeErr := fmt.Fprintf(f, "\n%s\n%s\n", comment, line)
		f.Close()
		if writeErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, writeErr))
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		results = append(results, Result{Path: path, Updated: true})
	}

	if len(results) == 0 && len(errs) == 0 {
		return nil, fmt.Errorf("no shell profile found; add %q to your shell profile manually", line)
	}
	return results, errors.Join(errs...)
}

// hasLine reports whether s contains line as a complete line.
// bufio.Scanner trims \r\n automatically, so CRLF files are handled correctly.
func hasLine(s, line string) bool {
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		if scanner.Text() == line {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Build and vet**

```bash
go build ./internal/shellhook/...
go vet ./internal/shellhook/...
```

Expected: no output, exit 0.

- [ ] **Step 4: Commit**

```bash
git add internal/shellhook/shellhook.go internal/shellhook/shellhook_test.go
git commit -m "feat: implement shellhook.Add with full-line duplicate detection"
```

---

### Task 3: `shellhook.Remove`

**Files:**
- Modify: `internal/shellhook/shellhook.go`
- Modify: `internal/shellhook/shellhook_test.go`

- [ ] **Step 1: Write failing tests for `removeFromProfiles`**

Append to `shellhook_test.go`:

```go
func TestRemove_LinePresent(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(rc, []byte("# SAP developer tips\nsap-devs tip\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || !results[0].Updated {
		t.Fatalf("expected one updated result, got %+v", results)
	}
	data, _ := os.ReadFile(rc)
	if strings.Contains(string(data), "sap-devs tip") {
		t.Error("hook line should have been removed")
	}
	if strings.Contains(string(data), "# SAP developer tips") {
		t.Error("comment line should have been removed along with hook")
	}
}

func TestRemove_LineAbsent(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(rc, []byte("# existing\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Updated {
		t.Fatalf("expected one not-updated result, got %+v", results)
	}
}

func TestRemove_SubstringNotRemoved(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	original := "# runs sap-devs tip daily\n"
	if err := os.WriteFile(rc, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Updated {
		t.Error("substring-only line should not have been removed")
	}
	data, _ := os.ReadFile(rc)
	if string(data) != original {
		t.Error("file should be unchanged")
	}
}

func TestRemove_MultipleProfiles(t *testing.T) {
	dir := t.TempDir()
	zshrc := filepath.Join(dir, ".zshrc")
	bashrc := filepath.Join(dir, ".bashrc")
	content := "# SAP developer tips\nsap-devs tip\n"
	for _, rc := range []string{zshrc, bashrc} {
		if err := os.WriteFile(rc, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{zshrc, bashrc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := 0
	for _, r := range results {
		if r.Updated {
			updated++
		}
	}
	if updated != 2 {
		t.Fatalf("expected 2 profiles updated, got %d (%+v)", updated, results)
	}
}

func TestRemove_NoProfiles(t *testing.T) {
	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{})
	if err != nil {
		t.Fatalf("unexpected error for empty candidate list: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %+v", results)
	}
}

func TestRemove_OrphanedCommentPreserved(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	// comment is present but NOT immediately followed by the hook line
	if err := os.WriteFile(rc, []byte("# SAP developer tips\nsome-other-command\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Updated {
		t.Error("orphaned comment should not cause a change")
	}
	data, _ := os.ReadFile(rc)
	if !strings.Contains(string(data), "# SAP developer tips") {
		t.Error("orphaned comment should be preserved")
	}
}

func TestRemove_MultipleHookBlocks(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	// two full comment+hook pairs in the same file
	content := "# SAP developer tips\nsap-devs tip\n# other\n# SAP developer tips\nsap-devs tip\n"
	if err := os.WriteFile(rc, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !results[0].Updated {
		t.Error("expected Updated=true")
	}
	data, _ := os.ReadFile(rc)
	if strings.Contains(string(data), "sap-devs tip") {
		t.Error("all hook lines should be removed")
	}
	if !strings.Contains(string(data), "# other") {
		t.Error("unrelated lines should be preserved")
	}
}

func TestRemove_WindowsPowerShellPath(t *testing.T) {
	dir := t.TempDir()
	psPath := filepath.Join(dir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	if err := os.MkdirAll(filepath.Dir(psPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(psPath, []byte("# SAP developer tips\nsap-devs tip\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{psPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || !results[0].Updated {
		t.Fatalf("expected updated result, got %+v", results)
	}
	data, _ := os.ReadFile(psPath)
	if strings.Contains(string(data), "sap-devs tip") {
		t.Error("hook should be removed from PowerShell profile")
	}
}
```

- [ ] **Step 2: Implement `removeFromProfiles` and `Remove` in `shellhook.go`**

Append to `shellhook.go`:

```go
// Remove strips every occurrence of line (full-line match) and any
// immediately preceding line equal to comment from all existing profiles.
// Returns one Result per candidate profile found on disk.
func Remove(line, comment string) ([]Result, error) {
	candidates, err := candidateProfiles()
	if err != nil {
		return nil, err
	}
	return removeFromProfiles(line, comment, candidates)
}

// removeFromProfiles is the testable core of Remove.
// Note: splitting on "\n" and joining back on "\n" preserves CRLF files
// naturally — lines retain their trailing \r, and comparison uses
// strings.TrimRight to strip it before matching.
func removeFromProfiles(line, comment string, candidates []string) ([]Result, error) {
	var results []Result
	var errs []error

	for _, path := range candidates {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			results = append(results, Result{Path: path, Updated: false})
			continue
		}

		rawLines := strings.Split(string(data), "\n")
		out := make([]string, 0, len(rawLines))
		changed := false

		for _, l := range rawLines {
			if strings.TrimRight(l, "\r") == line {
				// Remove this hook line and its immediately preceding comment.
				if len(out) > 0 && strings.TrimRight(out[len(out)-1], "\r") == comment {
					out = out[:len(out)-1]
				}
				changed = true
				continue
			}
			out = append(out, l)
		}

		if !changed {
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0644); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		results = append(results, Result{Path: path, Updated: true})
	}

	return results, errors.Join(errs...)
}
```

- [ ] **Step 3: Build and vet**

```bash
go build ./internal/shellhook/...
go vet ./internal/shellhook/...
```

Expected: no output, exit 0.

- [ ] **Step 4: Commit**

```bash
git add internal/shellhook/shellhook.go internal/shellhook/shellhook_test.go
git commit -m "feat: implement shellhook.Remove with full-line matching"
```

---

### Task 4: `tip install` and `tip uninstall` subcommands

**Files:**
- Modify: `cmd/tip.go`

- [ ] **Step 1: Add the `shellhook` import to `cmd/tip.go`**

The current imports in `cmd/tip.go`:

```go
import (
	"fmt"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/i18n"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)
```

Add `"github.com/SAP-samples/sap-devs-cli/internal/shellhook"` to the internal imports group.

- [ ] **Step 2: Define `tipInstallCmd` and `tipUninstallCmd` in `cmd/tip.go`**

Insert before `func init()`:

```go
var tipInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Add sap-devs tip to your shell profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := shellhook.Add("sap-devs tip", "# SAP developer tips")
		if err != nil && len(results) == 0 {
			// No profiles found — print manual fallback, not an error exit.
			fmt.Fprintln(cmd.OutOrStdout(), "No shell profile found. Add this line to your shell profile manually:")
			fmt.Fprintln(cmd.OutOrStdout(), "  sap-devs tip")
			return nil
		}
		for _, r := range results {
			if r.Updated {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Updated %s\n", r.Path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s — already configured\n", r.Path)
			}
		}
		return err
	},
}

var tipUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove sap-devs tip from your shell profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := shellhook.Remove("sap-devs tip", "# SAP developer tips")
		anyRemoved := false
		for _, r := range results {
			if r.Updated {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Removed from %s\n", r.Path)
				anyRemoved = true
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s — not configured\n", r.Path)
			}
		}
		if !anyRemoved && len(results) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "'sap-devs tip' was not found in any shell profile.")
		}
		return err
	},
}
```

- [ ] **Step 3: Register subcommands in `init()`**

Replace the existing `func init()` in `cmd/tip.go`:

```go
func init() {
	tipCmd.AddCommand(tipInstallCmd)
	tipCmd.AddCommand(tipUninstallCmd)
	rootCmd.AddCommand(tipCmd)
}
```

- [ ] **Step 4: Build and smoke-test**

```bash
go build ./...
go vet ./...
go run . tip --help
```

Expected: `install` and `uninstall` appear in the subcommand list:

```text
Available Commands:
  install     Add sap-devs tip to your shell profile
  uninstall   Remove sap-devs tip from your shell profile
```

- [ ] **Step 5: Commit**

```bash
git add cmd/tip.go
git commit -m "feat: add tip install and tip uninstall subcommands"
```

---

### Task 5: Refactor `cmd/init.go` to use `shellhook.Add`

**Files:**
- Modify: `cmd/init.go`

The existing `addShellHook()` (lines 131–150) stops at the **first** matching profile. After this change `init` writes to **all** existing profiles — this is intentional (see spec).

- [ ] **Step 1: Add the `shellhook` import to `cmd/init.go`**

Add `"github.com/SAP-samples/sap-devs-cli/internal/shellhook"` to the import block. The `"os"` import must stay — it is still used for `os.Stdin.Fd()`.

- [ ] **Step 2: Replace the `addShellHook()` call site**

Find this block (around line 105–111):

```go
if strings.ToLower(strings.TrimSpace(readLine())) == "y" {
    if err := addShellHook(); err != nil {
        fmt.Fprintf(cmd.OutOrStdout(), "  Could not auto-add hook: %v\n  Add 'sap-devs tip' to your shell profile manually.\n", err)
    } else {
        fmt.Fprintln(cmd.OutOrStdout(), "  Added. Restart your terminal to see your first tip.")
    }
}
```

Replace with:

```go
if strings.ToLower(strings.TrimSpace(readLine())) == "y" {
    results, hookErr := shellhook.Add("sap-devs tip", "# SAP developer tips")
    if hookErr != nil && len(results) == 0 {
        fmt.Fprintf(cmd.OutOrStdout(), "  Could not add hook: %v\n  Add 'sap-devs tip' to your shell profile manually.\n", hookErr)
    } else {
        for _, r := range results {
            if r.Updated {
                fmt.Fprintf(cmd.OutOrStdout(), "  Added to %s.\n", r.Path)
            }
        }
        fmt.Fprintln(cmd.OutOrStdout(), "  Restart your terminal to see your first tip.")
    }
}
```

- [ ] **Step 3: Delete `addShellHook()` from `cmd/init.go`**

Remove the entire function (currently lines 131–150):

```go
func addShellHook() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	candidates := []string{".zshrc", ".bashrc", ".bash_profile"}
	for _, rc := range candidates {
		path := home + "/" + rc
		if _, err := os.Stat(path); err == nil {
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			_, err = f.WriteString("\n# SAP developer tips\nsap-devs tip\n")
			f.Close()
			return err
		}
	}
	return fmt.Errorf("no shell rc file found (.zshrc, .bashrc, .bash_profile)")
}
```

- [ ] **Step 4: Build and vet**

```bash
go build ./...
go vet ./...
```

Expected: no output, exit 0.

- [ ] **Step 5: Commit**

```bash
git add cmd/init.go
git commit -m "refactor: replace addShellHook in init.go with shellhook.Add"
```

---

### Task 6: Update user guide

**Files:**
- Modify: `docs/user/user-guide.md`

Find the `#### Adding a daily tip to your terminal startup` subsection and replace its entire content (from the `####` heading down to, but not including, the next `---` separator) with:

```markdown
#### Adding a daily tip to your terminal startup

During `sap-devs init` you can opt in automatically. You can also manage it at any time:

```bash
sap-devs tip install    # add to all detected shell profiles
sap-devs tip uninstall  # remove from all shell profiles
```

Both commands show which profiles were updated:

```text
✓ Updated ~/.zshrc
✓ Updated ~/.bash_profile
  ~/.bashrc — already configured
```

If no profiles are detected, add the line manually:

```bash
# bash — ~/.bashrc or ~/.bash_profile
echo -e '\n# SAP developer tips\nsap-devs tip' >> ~/.bashrc

# zsh — ~/.zshrc
echo -e '\n# SAP developer tips\nsap-devs tip' >> ~/.zshrc
```

PowerShell — add to your `$PROFILE`:

```powershell
Add-Content $PROFILE "`n# SAP developer tips`nsap-devs tip"
```
```

- [ ] **Step 2: Build to confirm no regressions**

```bash
go build ./...
go vet ./...
```

- [ ] **Step 3: Commit**

```bash
git add docs/user/user-guide.md
git commit -m "docs: update tip section with install/uninstall commands"
```
