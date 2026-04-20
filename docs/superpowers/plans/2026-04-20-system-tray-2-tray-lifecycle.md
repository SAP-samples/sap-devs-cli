# Tray Lifecycle & CLI Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `sap-devs tray install/uninstall/start/stop/status` commands that download and manage an optional tray companion binary from GitHub Releases.

**Architecture:** A `trayctl` package in `internal/trayctl/` handles downloading, verifying, starting, and stopping the tray binary. CLI commands in `cmd/tray.go` wire it to cobra. The tray binary path is `~/.cache/sap-devs/bin/sap-devs-tray` (platform-adjusted). The download reuses patterns from `internal/update/installer.go`.

**Tech Stack:** Go stdlib (`os/exec`, `crypto/sha256`, `net/http`), cobra, existing `internal/update` helpers, existing `internal/xdg` paths

**Spec:** `docs/superpowers/specs/2026-04-20-system-tray-design.md`

**Windows note:** `go test` fails locally due to Windows Defender. Use `go build ./...` and `go vet ./...` locally; CI (ubuntu-latest) is the authoritative test runner.

---

### Task 1: Config — Add `tray_autostart` key

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestTrayConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.False(t, cfg.Tray.Autostart)
}

func TestTrayConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Tray.Autostart = true
	require.NoError(t, cfg.Save(dir))

	loaded, err := config.Load(dir)
	require.NoError(t, err)
	assert.True(t, loaded.Tray.Autostart)
}
```

- [ ] **Step 2: Write minimal implementation**

In `internal/config/config.go`, add:

```go
// TrayConfig controls the optional GUI tray companion.
type TrayConfig struct {
	Autostart bool `yaml:"autostart,omitempty"`
}
```

Add to the `Config` struct:

```go
Tray TrayConfig `yaml:"tray,omitempty"`
```

No default override needed — `Autostart` defaults to `false`.

- [ ] **Step 3: Run build to verify**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(tray): add tray_autostart config key"
```

---

### Task 2: Tray control manager — download and verify

**Files:**
- Create: `internal/trayctl/manager.go`
- Create: `internal/trayctl/manager_test.go`

- [ ] **Step 1: Write tests**

Create `internal/trayctl/manager_test.go`:

```go
package trayctl

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBinaryName(t *testing.T) {
	if runtime.GOOS == "windows" {
		assert.Equal(t, "sap-devs-tray.exe", binaryName())
	} else {
		assert.Equal(t, "sap-devs-tray", binaryName())
	}
}

func TestAssetName(t *testing.T) {
	name := assetName("1.2.3")
	assert.Contains(t, name, "sap-devs-tray_1.2.3_")
	assert.Contains(t, name, runtime.GOOS)
	assert.Contains(t, name, runtime.GOARCH)
}

func TestBinDir(t *testing.T) {
	m := &Manager{CacheDir: "/tmp/cache"}
	assert.Equal(t, filepath.Join("/tmp/cache", "bin"), m.binDir())
}

func TestIsInstalled_NotInstalled(t *testing.T) {
	m := &Manager{CacheDir: t.TempDir()}
	assert.False(t, m.IsInstalled())
}
```

- [ ] **Step 2: Write the implementation**

Create `internal/trayctl/manager.go`:

```go
package trayctl

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	repoURL         = "https://github.com/SAP-samples/sap-devs-cli"
	maxDownloadBytes = 200 * 1024 * 1024 // 200 MB (tray binary is larger)
)

// Manager handles downloading, verifying, starting, and stopping the tray binary.
type Manager struct {
	CacheDir  string
	Token     string // optional GitHub token
	Version   string // CLI version — tray must match
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "sap-devs-tray.exe"
	}
	return "sap-devs-tray"
}

func assetName(version string) string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("sap-devs-tray_%s_%s_%s%s", version, runtime.GOOS, runtime.GOARCH, ext)
}

func (m *Manager) binDir() string {
	return filepath.Join(m.CacheDir, "bin")
}

// BinaryPath returns the full path to the tray binary.
func (m *Manager) BinaryPath() string {
	return filepath.Join(m.binDir(), binaryName())
}

// IsInstalled checks if the tray binary exists on disk.
func (m *Manager) IsInstalled() bool {
	_, err := os.Stat(m.BinaryPath())
	return err == nil
}

// Install downloads and verifies the tray binary from GitHub Releases.
func (m *Manager) Install() error {
	if m.Version == "" || m.Version == "dev" {
		return fmt.Errorf("tray install requires a release build of sap-devs (current: %s)", m.Version)
	}

	asset := assetName(m.Version)
	tagName := "v" + m.Version
	downloadURL := repoURL + "/releases/download/" + tagName + "/"

	checksumData, err := httpGet(downloadURL+"tray-checksums.txt", m.Token)
	if err != nil {
		return fmt.Errorf("could not download tray-checksums.txt: %w", err)
	}
	expectedHash, err := findChecksum(checksumData, asset)
	if err != nil {
		return fmt.Errorf("tray binary not available for %s/%s in this release", runtime.GOOS, runtime.GOARCH)
	}

	archive, err := httpGet(downloadURL+asset, m.Token)
	if err != nil {
		return fmt.Errorf("could not download %s: %w", asset, err)
	}

	actual := sha256.Sum256(archive)
	if fmt.Sprintf("%x", actual) != expectedHash {
		return fmt.Errorf("checksum mismatch — download may be corrupt")
	}

	binBytes, err := extractBinary(archive, asset)
	if err != nil {
		return fmt.Errorf("could not extract binary: %w", err)
	}

	if err := os.MkdirAll(m.binDir(), 0755); err != nil {
		return err
	}
	path := m.BinaryPath()
	if err := os.WriteFile(path, binBytes, 0755); err != nil {
		return err
	}
	return nil
}

// Verify runs `sap-devs-tray --version` and checks it executes successfully.
func (m *Manager) Verify() error {
	cmd := exec.Command(m.BinaryPath(), "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tray binary verification failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Uninstall stops the tray and removes the binary.
func (m *Manager) Uninstall() error {
	_ = m.Stop()
	return os.Remove(m.BinaryPath())
}

// Start launches the tray process in the background.
func (m *Manager) Start() error {
	if !m.IsInstalled() {
		return fmt.Errorf("tray is not installed — run `sap-devs tray install` first")
	}
	cmd := exec.Command(m.BinaryPath())
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

// Stop sends a signal to terminate the running tray process.
func (m *Manager) Stop() error {
	// Find and kill by process name. Platform-specific but works for v1.
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("taskkill", "/im", "sap-devs-tray.exe", "/f")
	default:
		cmd = exec.Command("pkill", "-f", "sap-devs-tray")
	}
	return cmd.Run()
}

// IsRunning checks whether the tray process is currently running.
func (m *Manager) IsRunning() bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("tasklist", "/fi", "imagename eq sap-devs-tray.exe", "/nh")
	default:
		cmd = exec.Command("pgrep", "-f", "sap-devs-tray")
	}
	return cmd.Run() == nil
}

func httpGet(url, token string) ([]byte, error) {
	client := &http.Client{Timeout: 300 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes))
}

func findChecksum(data []byte, assetName string) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("asset %s not found in checksums", assetName)
}

// extractBinary extracts the tray binary from an archive.
// Reuses the same tar.gz/zip extraction pattern as internal/update.
func extractBinary(data []byte, assetFileName string) ([]byte, error) {
	name := binaryName()
	if strings.HasSuffix(assetFileName, ".zip") {
		return extractFromZip(data, name)
	}
	return extractFromTarGz(data, name)
}
```

Note: `extractFromZip` and `extractFromTarGz` should be copied from `internal/update/installer.go` (lines 163-202) with the binary name changed from `"sap-devs"` to accept a parameter. The `httpGet` and `findChecksum` functions in `manager.go` are also near-identical copies of their counterparts in `internal/update/installer.go`. All four functions are deliberately duplicated to keep `trayctl` and `update` decoupled — neither package imports the other. If a shared `internal/archive` or `internal/httputil` package is desired later, that's a refactor, not a blocker.

- [ ] **Step 3: Add archive extraction helpers**

Copy `extractFromTarGz` and `extractFromZip` from `internal/update/installer.go` into a new file `internal/trayctl/extract.go`:

```go
package trayctl

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
)

func extractFromTarGz(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(hdr.Name) == name {
			return io.ReadAll(io.LimitReader(tr, maxDownloadBytes))
		}
	}
	return nil, fmt.Errorf("binary %q not found in archive", name)
}

func extractFromZip(data []byte, name string) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		if filepath.Base(f.Name) == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			result, err := io.ReadAll(io.LimitReader(rc, maxDownloadBytes))
			rc.Close()
			return result, err
		}
	}
	return nil, fmt.Errorf("binary %q not found in zip archive", name)
}
```

- [ ] **Step 4: Run build to verify**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 5: Commit**

```bash
git add internal/trayctl/manager.go internal/trayctl/manager_test.go internal/trayctl/extract.go
git commit -m "feat(tray): add trayctl manager for download, verify, start, stop"
```

---

### Task 3: OS autostart registration

**Files:**
- Create: `internal/trayctl/autostart.go`
- Create: `internal/trayctl/autostart_test.go`

- [ ] **Step 1: Write tests**

Create `internal/trayctl/autostart_test.go`:

```go
package trayctl

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAutostartEntryName(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
		assert.Equal(t, "sap-devs-tray", autostartEntryName())
	case "darwin":
		assert.Equal(t, "com.sap-devs.tray", autostartEntryName())
	case "linux":
		assert.Equal(t, "sap-devs-tray.desktop", autostartEntryName())
	}
}
```

- [ ] **Step 2: Write the implementation**

Create `internal/trayctl/autostart.go`:

```go
package trayctl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func autostartEntryName() string {
	switch runtime.GOOS {
	case "windows":
		return "sap-devs-tray"
	case "darwin":
		return "com.sap-devs.tray"
	default:
		return "sap-devs-tray.desktop"
	}
}

// RegisterAutostart registers the tray binary to start on OS login.
func (m *Manager) RegisterAutostart() error {
	binaryPath := m.BinaryPath()
	switch runtime.GOOS {
	case "windows":
		return registerWindowsAutostart(binaryPath)
	case "darwin":
		return registerDarwinAutostart(binaryPath)
	case "linux":
		return registerLinuxAutostart(binaryPath)
	default:
		return fmt.Errorf("autostart not supported on %s", runtime.GOOS)
	}
}

// UnregisterAutostart removes the tray from OS login startup.
func (m *Manager) UnregisterAutostart() error {
	switch runtime.GOOS {
	case "windows":
		return unregisterWindowsAutostart()
	case "darwin":
		return unregisterDarwinAutostart()
	case "linux":
		return unregisterLinuxAutostart()
	default:
		return nil
	}
}

func registerWindowsAutostart(binaryPath string) error {
	cmd := exec.Command("reg", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", "sap-devs-tray",
		"/t", "REG_SZ",
		"/d", binaryPath,
		"/f",
	)
	return cmd.Run()
}

func unregisterWindowsAutostart() error {
	cmd := exec.Command("reg", "delete",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", "sap-devs-tray",
		"/f",
	)
	return cmd.Run()
}

func registerDarwinAutostart(binaryPath string) error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.sap-devs.tray</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>`, binaryPath)

	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "Library", "LaunchAgents", "com.sap-devs.tray.plist")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(plist), 0644)
}

func unregisterDarwinAutostart() error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "Library", "LaunchAgents", "com.sap-devs.tray.plist")
	_ = exec.Command("launchctl", "unload", path).Run()
	return os.Remove(path)
}

func registerLinuxAutostart(binaryPath string) error {
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=sap-devs Tray
Exec=%s
Terminal=false
StartupNotify=false
X-GNOME-Autostart-enabled=true
`, binaryPath)

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "autostart")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "sap-devs-tray.desktop"), []byte(entry), 0644)
}

func unregisterLinuxAutostart() error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".config", "autostart", "sap-devs-tray.desktop")
	return os.Remove(path)
}
```

- [ ] **Step 3: Run build to verify**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 4: Commit**

```bash
git add internal/trayctl/autostart.go internal/trayctl/autostart_test.go
git commit -m "feat(tray): add cross-platform autostart registration"
```

---

### Task 4: CLI commands — `sap-devs tray install/uninstall/start/stop/status`

**Files:**
- Create: `cmd/tray.go`

- [ ] **Step 1: Write the command file**

Create `cmd/tray.go`:

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/credentials"
	"github.com/SAP-samples/sap-devs-cli/internal/trayctl"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
	"github.com/spf13/cobra"
)

var trayCmd = &cobra.Command{
	Use:   "tray",
	Short: "Manage the optional GUI tray companion",
	Long: `Manage the sap-devs system tray companion — an optional graphical dashboard
that shows sync status, active profile, and injected tools at a glance.

Note: The tray companion uses Wails v3 (currently in alpha).
This is an optional enhancement — all CLI features work without it.`,
}

var trayInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Download and install the tray companion binary",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}

		mgr := &trayctl.Manager{
			CacheDir: paths.CacheDir,
			Token:    credentials.Resolve(paths.ConfigDir),
			Version:  Version,
		}

		out := cmd.OutOrStdout()
		fmt.Fprintln(out, "Downloading sap-devs-tray...")
		if err := mgr.Install(); err != nil {
			return err
		}

		fmt.Fprintln(out, "Verifying...")
		if err := mgr.Verify(); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}

		fmt.Fprintln(out, "Tray companion installed successfully.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Note: The sap-devs tray companion uses Wails v3 (currently in alpha).")
		fmt.Fprintln(out, "This is an optional enhancement — all CLI features work without it.")
		fmt.Fprintln(out, "If you encounter issues, run `sap-devs tray uninstall` to remove it.")
		fmt.Fprintln(out)

		fmt.Fprint(out, "Start tray automatically on login? [Y/n] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || answer == "y" || answer == "yes" {
			if err := mgr.RegisterAutostart(); err != nil {
				fmt.Fprintf(out, "Warning: could not register autostart: %v\n", err)
			} else {
				fmt.Fprintln(out, "Autostart registered.")
				cfg, _ := config.Load(paths.ConfigDir)
				cfg.Tray.Autostart = true
				_ = cfg.Save(paths.ConfigDir)
			}
		}

		return nil
	},
}

var trayUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the tray companion",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		mgr := &trayctl.Manager{CacheDir: paths.CacheDir, Version: Version}

		mgr.UnregisterAutostart()
		if err := mgr.Uninstall(); err != nil {
			return err
		}

		cfg, _ := config.Load(paths.ConfigDir)
		cfg.Tray.Autostart = false
		_ = cfg.Save(paths.ConfigDir)

		fmt.Fprintln(cmd.OutOrStdout(), "Tray companion uninstalled.")
		return nil
	},
}

var trayStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Launch the tray companion",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		mgr := &trayctl.Manager{CacheDir: paths.CacheDir}
		if err := mgr.Start(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Tray companion started.")
		return nil
	},
}

var trayStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running tray companion",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		mgr := &trayctl.Manager{CacheDir: paths.CacheDir}
		if err := mgr.Stop(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Tray companion stopped.")
		return nil
	},
}

var trayStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show tray companion status",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		mgr := &trayctl.Manager{CacheDir: paths.CacheDir, Version: Version}
		out := cmd.OutOrStdout()

		if !mgr.IsInstalled() {
			fmt.Fprintln(out, "Tray: not installed")
			fmt.Fprintln(out, "Run `sap-devs tray install` to download the tray companion.")
			return nil
		}

		running := "stopped"
		if mgr.IsRunning() {
			running = "running"
		}

		cfg, _ := config.Load(paths.ConfigDir)
		autostart := "disabled"
		if cfg.Tray.Autostart {
			autostart = "enabled"
		}

		fmt.Fprintf(out, "Tray:      installed (%s)\n", running)
		fmt.Fprintf(out, "Autostart: %s\n", autostart)
		fmt.Fprintf(out, "Binary:    %s\n", mgr.BinaryPath())
		return nil
	},
}

func init() {
	trayCmd.AddCommand(trayInstallCmd)
	trayCmd.AddCommand(trayUninstallCmd)
	trayCmd.AddCommand(trayStartCmd)
	trayCmd.AddCommand(trayStopCmd)
	trayCmd.AddCommand(trayStatusCmd)
	rootCmd.AddCommand(trayCmd)
}
```

- [ ] **Step 2: Run build to verify**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 3: Smoke test**

Run: `go run . tray status`
Expected: "Tray: not installed" message

Run: `go run . tray --help`
Expected: Help text with alpha notice in long description

- [ ] **Step 4: Commit**

```bash
git add cmd/tray.go
git commit -m "feat(tray): add tray install/uninstall/start/stop/status CLI commands"
```

---

### Task 5: Wire tray update into `sap-devs update`

**Files:**
- Modify: `cmd/update.go`

- [ ] **Step 1: Add tray update logic**

After the existing self-update succeeds in `cmd/update.go`, add tray binary update:

```go
// After successful CLI update, check if tray is installed and update it too
paths, err := xdg.New()
if err == nil {
	mgr := &trayctl.Manager{CacheDir: paths.CacheDir, Version: rel.Version, Token: token}
	if mgr.IsInstalled() {
		fmt.Fprintln(cmd.OutOrStdout(), "Updating tray companion...")
		if err := mgr.Install(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: tray update failed: %v\n", err)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Tray companion updated.")
		}
	}
}
```

- [ ] **Step 2: Run build to verify**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 3: Commit**

```bash
git add cmd/update.go
git commit -m "feat(tray): update tray binary during sap-devs update"
```

---

### Task 6: Update CLAUDE.md and docs

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add tray commands to CLI reference table in CLAUDE.md**

Add to the CLI Commands table:

```markdown
| `tray install/uninstall/start/stop/status` | Download and manage optional GUI tray companion (Wails v3, experimental) |
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add tray commands to CLI reference"
```
