# OS-Native Scheduler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs service install/uninstall/status` commands that register an OS-native scheduled task to run `sap-devs sync && sap-devs inject --no-sync` on a configurable interval.

**Architecture:** A `Scheduler` interface in `internal/service/` with three platform implementations behind build tags. Each implementation generates a platform-native config file and shells out to OS tools (`schtasks`, `launchctl`, `systemctl`). CLI commands in `cmd/service.go` wire the interface to cobra.

**Tech Stack:** Go stdlib (`os/exec`, `text/template`, `runtime`), cobra, build tags

**Spec:** `docs/superpowers/specs/2026-04-20-system-tray-design.md`

**Windows note:** `go test` fails locally due to Windows Defender. Use `go build ./...` and `go vet ./...` locally; CI (ubuntu-latest) is the authoritative test runner.

---

### Task 1: Config — Add `service_interval` key

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestServiceConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 6*time.Hour, cfg.Service.Interval)
}

func TestServiceConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Service.Interval = 12 * time.Hour
	require.NoError(t, cfg.Save(dir))

	loaded, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 12*time.Hour, loaded.Service.Interval)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... && go vet ./...`
Expected: Compilation failure — `Config` has no `Service` field

- [ ] **Step 3: Write minimal implementation**

In `internal/config/config.go`, add the struct and field.

Note: The spec shows a flat `service_interval: 6h` for brevity, but we use a nested struct (`service.interval`) to match the existing pattern for `SyncConfig`, `TipConfig`, etc. This is an intentional, superior organization — all config sub-structs follow this nested pattern.

```go
// ServiceConfig controls the OS-native background scheduler.
type ServiceConfig struct {
	Interval time.Duration `yaml:"interval"`
}
```

Add to the `Config` struct:

```go
Service ServiceConfig `yaml:"service,omitempty"`
```

Add to `Default()`:

```go
Service: ServiceConfig{
	Interval: 6 * time.Hour,
},
```

- [ ] **Step 4: Run build to verify it compiles**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(service): add service_interval config key with 6h default"
```

---

### Task 2: Scheduler interface and Status type

**Files:**
- Create: `internal/service/scheduler.go`

- [ ] **Step 1: Create the interface file**

Create `internal/service/scheduler.go`:

```go
package service

import (
	"fmt"
	"time"
)

// Status describes the current state of the OS scheduler entry.
type Status struct {
	Installed bool
	Interval  time.Duration
	LastRun   time.Time // zero value if unknown or never run
	NextRun   time.Time // zero value if unknown
}

// Scheduler manages an OS-native scheduled task for background sync+inject.
type Scheduler interface {
	Install(interval time.Duration, binaryPath string) error
	Uninstall() error
	Status() (*Status, error)
}

// New returns the platform-appropriate Scheduler for the given cache directory.
// cacheDir is used for the daemon log path.
func New(cacheDir string) Scheduler {
	return newPlatformScheduler(cacheDir)
}

// ErrNotInstalled is returned when querying status of an uninstalled scheduler.
var ErrNotInstalled = fmt.Errorf("scheduler is not installed")
```

- [ ] **Step 2: Run build to verify it compiles**

Run: `go build ./...`
Expected: Fails because `newPlatformScheduler` is not defined yet (expected — platform files come next)

- [ ] **Step 3: Commit**

```bash
git add internal/service/scheduler.go
git commit -m "feat(service): add Scheduler interface and Status type"
```

---

### Task 3: Windows scheduler implementation

**Files:**
- Create: `internal/service/scheduler_windows.go`
- Create: `internal/service/scheduler_windows_test.go`

- [ ] **Step 1: Write the test**

Create `internal/service/scheduler_windows_test.go`:

```go
//go:build windows

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindowsTaskName(t *testing.T) {
	s := &windowsScheduler{cacheDir: t.TempDir()}
	assert.Equal(t, "sap-devs-sync", s.taskName())
}

func TestWindowsIntervalMinutes(t *testing.T) {
	assert.Equal(t, "360", intervalMinutes(6*time.Hour))
	assert.Equal(t, "60", intervalMinutes(1*time.Hour))
	assert.Equal(t, "1440", intervalMinutes(24*time.Hour))
}
```

- [ ] **Step 2: Write the implementation**

Create `internal/service/scheduler_windows.go`:

```go
//go:build windows

package service

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type windowsScheduler struct {
	cacheDir string
}

func newPlatformScheduler(cacheDir string) Scheduler {
	return &windowsScheduler{cacheDir: cacheDir}
}

func (s *windowsScheduler) taskName() string { return "sap-devs-sync" }

func (s *windowsScheduler) logPath() string {
	return filepath.Join(s.cacheDir, "daemon.log")
}

func (s *windowsScheduler) Install(interval time.Duration, binaryPath string) error {
	script := fmt.Sprintf(`%s sync > "%s" 2>&1 && %s inject --no-sync >> "%s" 2>&1`,
		binaryPath, s.logPath(), binaryPath, s.logPath())

	cmd := exec.Command("schtasks", "/create",
		"/tn", s.taskName(),
		"/tr", fmt.Sprintf(`cmd /c "%s"`, script),
		"/sc", "minute",
		"/mo", intervalMinutes(interval),
		"/f",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks create failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *windowsScheduler) Uninstall() error {
	cmd := exec.Command("schtasks", "/delete", "/tn", s.taskName(), "/f")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks delete failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *windowsScheduler) Status() (*Status, error) {
	cmd := exec.Command("schtasks", "/query", "/tn", s.taskName(), "/fo", "csv", "/nh")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &Status{Installed: false}, nil
	}
	fields := strings.Split(strings.TrimSpace(string(out)), ",")
	st := &Status{Installed: true}
	if len(fields) >= 3 {
		if t, err := time.Parse("1/2/2006 3:04:05 PM", strings.Trim(fields[2], "\"")); err == nil {
			st.NextRun = t
		}
	}
	return st, nil
}

func intervalMinutes(d time.Duration) string {
	mins := int(d.Minutes())
	if mins < 1 {
		mins = 1
	}
	return strconv.Itoa(mins)
}
```

- [ ] **Step 3: Run build to verify it compiles**

Run: `go build ./...`
Expected: On Windows, should compile. On other platforms, this file is skipped by build tag.

- [ ] **Step 4: Commit**

```bash
git add internal/service/scheduler_windows.go internal/service/scheduler_windows_test.go
git commit -m "feat(service): add Windows Task Scheduler implementation"
```

---

### Task 4: macOS scheduler implementation

**Files:**
- Create: `internal/service/scheduler_darwin.go`
- Create: `internal/service/scheduler_darwin_test.go`

- [ ] **Step 1: Write the test**

Create `internal/service/scheduler_darwin_test.go`:

```go
//go:build darwin

package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDarwinPlistGeneration(t *testing.T) {
	dir := t.TempDir()
	s := &darwinScheduler{cacheDir: dir}
	plist := s.generatePlist(6*time.Hour, "/usr/local/bin/sap-devs")
	assert.Contains(t, plist, "<integer>21600</integer>")
	assert.Contains(t, plist, "/usr/local/bin/sap-devs")
	assert.Contains(t, plist, "com.sap-devs.sync")
}

func TestDarwinPlistPath(t *testing.T) {
	s := &darwinScheduler{cacheDir: t.TempDir()}
	path := s.plistPath()
	assert.Contains(t, path, "LaunchAgents")
	assert.Contains(t, path, "com.sap-devs.sync.plist")
}
```

- [ ] **Step 2: Write the implementation**

Create `internal/service/scheduler_darwin.go`:

```go
//go:build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type darwinScheduler struct {
	cacheDir string
}

func newPlatformScheduler(cacheDir string) Scheduler {
	return &darwinScheduler{cacheDir: cacheDir}
}

const plistLabel = "com.sap-devs.sync"

func (s *darwinScheduler) plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist")
}

func (s *darwinScheduler) logPath() string {
	return filepath.Join(s.cacheDir, "daemon.log")
}

func (s *darwinScheduler) generatePlist(interval time.Duration, binaryPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>/bin/sh</string>
		<string>-c</string>
		<string>%s sync &amp;&amp; %s inject --no-sync</string>
	</array>
	<key>StartInterval</key>
	<integer>%d</integer>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>`, plistLabel, binaryPath, binaryPath, int(interval.Seconds()), s.logPath(), s.logPath())
}

func (s *darwinScheduler) Install(interval time.Duration, binaryPath string) error {
	plist := s.generatePlist(interval, binaryPath)
	path := s.plistPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(plist), 0644); err != nil {
		return err
	}
	cmd := exec.Command("launchctl", "load", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *darwinScheduler) Uninstall() error {
	path := s.plistPath()
	cmd := exec.Command("launchctl", "unload", path)
	_ = cmd.Run() // ignore error if not loaded
	return os.Remove(path)
}

func (s *darwinScheduler) Status() (*Status, error) {
	path := s.plistPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Status{Installed: false}, nil
	}
	cmd := exec.Command("launchctl", "list", plistLabel)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &Status{Installed: true}, nil
	}
	_ = out // launchctl list output is informational
	return &Status{Installed: true}, nil
}
```

- [ ] **Step 3: Run build to verify**

Run: `go build ./...`
Expected: Compiles (darwin build tag active on macOS, skipped elsewhere)

- [ ] **Step 4: Commit**

```bash
git add internal/service/scheduler_darwin.go internal/service/scheduler_darwin_test.go
git commit -m "feat(service): add macOS launchd scheduler implementation"
```

---

### Task 5: Linux scheduler implementation

**Files:**
- Create: `internal/service/scheduler_linux.go`
- Create: `internal/service/scheduler_linux_test.go`

- [ ] **Step 1: Write the test**

Create `internal/service/scheduler_linux_test.go`:

```go
//go:build linux

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLinuxServiceUnit(t *testing.T) {
	s := &linuxScheduler{cacheDir: "/tmp/test"}
	unit := s.generateServiceUnit("/usr/local/bin/sap-devs")
	assert.Contains(t, unit, "ExecStart=/bin/sh")
	assert.Contains(t, unit, "/usr/local/bin/sap-devs sync")
	assert.Contains(t, unit, "sap-devs background sync")
}

func TestLinuxTimerUnit(t *testing.T) {
	s := &linuxScheduler{cacheDir: "/tmp/test"}
	timer := s.generateTimerUnit(6 * time.Hour)
	assert.Contains(t, timer, "OnUnitActiveSec=6h0m0s")
	assert.Contains(t, timer, "OnBootSec=5min")
}
```

- [ ] **Step 2: Write the implementation**

Create `internal/service/scheduler_linux.go`:

```go
//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type linuxScheduler struct {
	cacheDir string
}

func newPlatformScheduler(cacheDir string) Scheduler {
	return &linuxScheduler{cacheDir: cacheDir}
}

const unitName = "sap-devs-sync"

func (s *linuxScheduler) unitDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user")
}

func (s *linuxScheduler) logPath() string {
	return filepath.Join(s.cacheDir, "daemon.log")
}

func (s *linuxScheduler) generateServiceUnit(binaryPath string) string {
	return fmt.Sprintf(`[Unit]
Description=sap-devs background sync

[Service]
Type=oneshot
ExecStart=/bin/sh -c '%s sync && %s inject --no-sync'
StandardOutput=file:%s
StandardError=file:%s
`, binaryPath, binaryPath, s.logPath(), s.logPath())
}

func (s *linuxScheduler) generateTimerUnit(interval time.Duration) string {
	return fmt.Sprintf(`[Unit]
Description=sap-devs background sync timer

[Timer]
OnBootSec=5min
OnUnitActiveSec=%s
Persistent=true

[Install]
WantedBy=timers.target
`, interval)
}

func (s *linuxScheduler) Install(interval time.Duration, binaryPath string) error {
	dir := s.unitDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	svc := s.generateServiceUnit(binaryPath)
	if err := os.WriteFile(filepath.Join(dir, unitName+".service"), []byte(svc), 0644); err != nil {
		return err
	}
	tmr := s.generateTimerUnit(interval)
	if err := os.WriteFile(filepath.Join(dir, unitName+".timer"), []byte(tmr), 0644); err != nil {
		return err
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	cmd := exec.Command("systemctl", "--user", "enable", "--now", unitName+".timer")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl enable failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *linuxScheduler) Uninstall() error {
	_ = exec.Command("systemctl", "--user", "disable", "--now", unitName+".timer").Run()
	dir := s.unitDir()
	_ = os.Remove(filepath.Join(dir, unitName+".service"))
	_ = os.Remove(filepath.Join(dir, unitName+".timer"))
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func (s *linuxScheduler) Status() (*Status, error) {
	dir := s.unitDir()
	timerPath := filepath.Join(dir, unitName+".timer")
	if _, err := os.Stat(timerPath); os.IsNotExist(err) {
		return &Status{Installed: false}, nil
	}
	st := &Status{Installed: true}
	cmd := exec.Command("systemctl", "--user", "show", unitName+".timer",
		"--property=LastTriggerUSec,NextElapseUSecRealtime")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return st, nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", parts[1]); err == nil {
			switch parts[0] {
			case "LastTriggerUSec":
				st.LastRun = t
			case "NextElapseUSecRealtime":
				st.NextRun = t
			}
		}
	}
	return st, nil
}
```

- [ ] **Step 3: Run build to verify**

Run: `go build ./...`
Expected: Compiles on Linux; skipped on other platforms

- [ ] **Step 4: Commit**

```bash
git add internal/service/scheduler_linux.go internal/service/scheduler_linux_test.go
git commit -m "feat(service): add Linux systemd timer scheduler implementation"
```

---

### Task 6: CLI commands — `sap-devs service install/uninstall/status`

**Files:**
- Create: `cmd/service.go`

- [ ] **Step 1: Write the command file**

Create `cmd/service.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/service"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage background sync scheduler",
	Long:  "Register or remove an OS-native scheduled task that runs sap-devs sync + inject on a configurable interval.",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Register the OS scheduler entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}

		binaryPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("could not determine binary path: %w", err)
		}

		sched := service.New(paths.CacheDir)
		interval := cfg.Service.Interval
		if interval == 0 {
			interval = 6 * time.Hour
		}

		if err := sched.Install(interval, binaryPath); err != nil {
			return fmt.Errorf("could not install scheduler: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Background scheduler installed (every %s).\n", interval)
		fmt.Fprintf(cmd.OutOrStdout(), "Runs: %s sync && %s inject --no-sync\n", binaryPath, binaryPath)
		return nil
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the OS scheduler entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		sched := service.New(paths.CacheDir)
		if err := sched.Uninstall(); err != nil {
			return fmt.Errorf("could not uninstall scheduler: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Background scheduler removed.")
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show scheduler status",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		sched := service.New(paths.CacheDir)
		st, err := sched.Status()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if !st.Installed {
			fmt.Fprintln(out, "Scheduler: not installed")
			fmt.Fprintln(out, "Run `sap-devs service install` to enable background sync.")
			return nil
		}
		fmt.Fprintln(out, "Scheduler: installed")
		if !st.LastRun.IsZero() {
			fmt.Fprintf(out, "Last run:  %s\n", st.LastRun.Format(time.RFC3339))
		}
		if !st.NextRun.IsZero() {
			fmt.Fprintf(out, "Next run:  %s\n", st.NextRun.Format(time.RFC3339))
		}
		return nil
	},
}

func init() {
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	rootCmd.AddCommand(serviceCmd)
}
```

- [ ] **Step 2: Run build to verify**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 3: Smoke test**

Run: `go run . service status`
Expected: "Scheduler: not installed" message

- [ ] **Step 4: Commit**

```bash
git add cmd/service.go
git commit -m "feat(service): add service install/uninstall/status CLI commands"
```

---

### Task 7: Update CLAUDE.md and docs

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/developer-guide.md` (if exists)

- [ ] **Step 1: Add service commands to CLI reference table in CLAUDE.md**

Add to the CLI Commands table:

```markdown
| `service install/uninstall/status` | Manage OS-native background scheduler (systemd/launchd/Task Scheduler) |
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add service commands to CLI reference"
```
