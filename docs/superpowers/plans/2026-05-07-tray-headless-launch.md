# Tray Headless Launch & App Shortcuts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate the terminal window when the tray binary launches on Windows, and provide native app shortcuts for easy manual launch on all platforms.

**Architecture:** Build-tagged platform files split process launching (`startProcess`) and shortcut management (`CreateShortcuts`/`RemoveShortcuts`) into per-OS implementations. The release pipeline packages icons alongside the binary and sets the Windows PE subsystem to GUI. The existing `Install()`/`Uninstall()` methods in `manager.go` gain shortcut lifecycle calls.

**Tech Stack:** Go build tags, `syscall.SysProcAttr` (Windows), PowerShell COM (`.lnk`), macOS `.app` bundle, freedesktop `.desktop` files, GitHub Actions workflow YAML.

**Spec:** `docs/superpowers/specs/2026-05-07-tray-headless-launch-design.md`

---

### Task 1: Platform-specific `startProcess` — Windows

**Files:**
- Create: `internal/trayctl/start_windows.go`

- [ ] **Step 1: Create the build-tagged Windows start implementation**

```go
//go:build windows

package trayctl

import (
	"os/exec"
	"syscall"
)

func startProcess(binaryPath string) error {
	cmd := exec.Command(binaryPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008 | 0x00000010, // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
	}
	return cmd.Start()
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/trayctl/`
Expected: clean build (no output)

- [ ] **Step 3: Commit**

```bash
git add internal/trayctl/start_windows.go
git commit -m "feat(tray): add Windows startProcess with DETACHED_PROCESS flags"
```

---

### Task 2: Platform-specific `startProcess` — Unix default

**Files:**
- Create: `internal/trayctl/start_other.go`

- [ ] **Step 1: Create the build-tagged non-Windows implementation**

```go
//go:build !windows

package trayctl

import "os/exec"

func startProcess(binaryPath string) error {
	cmd := exec.Command(binaryPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/trayctl/`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/trayctl/start_other.go
git commit -m "feat(tray): add Unix startProcess with process release"
```

---

### Task 3: Refactor `Start()` to delegate to `startProcess`

**Files:**
- Modify: `internal/trayctl/manager.go:132-143`

- [ ] **Step 1: Replace the `Start()` method body**

Change the current `Start()` method (lines 132-143) to:

```go
func (m *Manager) Start() error {
	if !m.IsInstalled() {
		return fmt.Errorf("tray is not installed — run `sap-devs tray install` first")
	}
	return startProcess(m.BinaryPath())
}
```

- [ ] **Step 2: Remove unused import `"os/exec"` if it's now only used elsewhere**

Check if `os/exec` is still needed by other methods in `manager.go` (`Stop()`, `Verify()`, `IsRunning()` — yes, they use it). No change needed.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/trayctl/`
Expected: clean build

- [ ] **Step 4: Run vet**

Run: `go vet ./internal/trayctl/`
Expected: no issues

- [ ] **Step 5: Commit**

```bash
git add internal/trayctl/manager.go
git commit -m "refactor(tray): delegate Start() to platform-specific startProcess"
```

---

### Task 4: Extract assets — replace `extractBinary` with `extractAssets`

**Files:**
- Modify: `internal/trayctl/extract.go`
- Modify: `internal/trayctl/manager.go:100-112`

- [ ] **Step 1: Add `extractAllFromTarGz` and `extractAllFromZip` functions to `extract.go`**

Add below the existing functions in `extract.go`:

```go
func extractAllFromTarGz(data []byte) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	files := make(map[string][]byte)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(tr, maxDownloadBytes))
		if err != nil {
			return nil, err
		}
		files[filepath.Base(hdr.Name)] = content
	}
	return files, nil
}

func extractAllFromZip(data []byte) (map[string][]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(rc, maxDownloadBytes))
		rc.Close()
		if err != nil {
			return nil, err
		}
		files[filepath.Base(f.Name)] = content
	}
	return files, nil
}

func extractAssets(data []byte, assetFileName string) (map[string][]byte, error) {
	if strings.HasSuffix(assetFileName, ".zip") {
		return extractAllFromZip(data)
	}
	return extractAllFromTarGz(data)
}
```

- [ ] **Step 2: Update imports in `extract.go` to add `"strings"`**

Replace the import block in `extract.go` with:

```go
import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)
```

- [ ] **Step 3: Update `Install()` in `manager.go` to use `extractAssets`**

Replace lines 100-112 of `manager.go` with:

```go
	assets, err := extractAssets(archive, asset)
	if err != nil {
		return fmt.Errorf("could not extract assets: %w", err)
	}

	if err := os.MkdirAll(m.binDir(), 0755); err != nil {
		return err
	}
	for name, content := range assets {
		perm := os.FileMode(0644)
		if name == binaryName() {
			perm = 0755
		}
		if err := os.WriteFile(filepath.Join(m.binDir(), name), content, perm); err != nil {
			return err
		}
	}
```

- [ ] **Step 4: Delete the now-dead `extractBinary` function from `manager.go`**

Remove lines 206-212 of `manager.go` (the old single-file extractor is replaced by `extractAssets`):

```go
// DELETE this entire function:
func extractBinary(data []byte, assetFileName string) ([]byte, error) {
	name := binaryName()
	if strings.HasSuffix(assetFileName, ".zip") {
		return extractFromZip(data, name)
	}
	return extractFromTarGz(data, name)
}
```

- [ ] **Step 5: Verify it compiles**

Run: `go build ./internal/trayctl/`
Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add internal/trayctl/extract.go internal/trayctl/manager.go
git commit -m "feat(tray): extract all archive assets (binary + icon) on install"
```

---

### Task 5: Windows shortcuts — `CreateShortcuts` / `RemoveShortcuts`

**Files:**
- Create: `internal/trayctl/shortcut_windows.go`

- [ ] **Step 1: Create the Windows shortcut implementation**

```go
//go:build windows

package trayctl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (m *Manager) CreateShortcuts() error {
	target := m.BinaryPath()
	workDir := m.binDir()
	iconPath := filepath.Join(m.binDir(), "sap-devs-tray.ico")

	startMenuDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs")
	if err := createLnk(filepath.Join(startMenuDir, "SAP Devs Tray.lnk"), target, workDir, iconPath); err != nil {
		return fmt.Errorf("start menu shortcut: %w", err)
	}

	desktopPath, err := resolveDesktopPath()
	if err != nil {
		return fmt.Errorf("resolve desktop path: %w", err)
	}
	if err := createLnk(filepath.Join(desktopPath, "SAP Devs Tray.lnk"), target, workDir, iconPath); err != nil {
		return fmt.Errorf("desktop shortcut: %w", err)
	}
	return nil
}

func (m *Manager) RemoveShortcuts() error {
	startMenuLink := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "SAP Devs Tray.lnk")
	_ = os.Remove(startMenuLink)

	desktopPath, _ := resolveDesktopPath()
	if desktopPath != "" {
		_ = os.Remove(filepath.Join(desktopPath, "SAP Devs Tray.lnk"))
	}

	_ = os.Remove(filepath.Join(m.binDir(), "sap-devs-tray.ico"))
	return nil
}

func resolveDesktopPath() (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"[Environment]::GetFolderPath('Desktop')")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("could not resolve Desktop folder")
	}
	return path, nil
}

func createLnk(lnkPath, target, workDir, iconPath string) error {
	script := fmt.Sprintf(`
$ws = New-Object -ComObject WScript.Shell
$s = $ws.CreateShortcut('%s')
$s.TargetPath = '%s'
$s.WorkingDirectory = '%s'
$s.IconLocation = '%s,0'
$s.Save()
`, lnkPath, target, workDir, iconPath)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
```

- [ ] **Step 2: Add `"strings"` to imports**

Already present in the code above — verify the import block includes `"strings"`.

- [ ] **Step 3: Verify it compiles**

Run: `GOOS=windows go build ./internal/trayctl/`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/trayctl/shortcut_windows.go
git commit -m "feat(tray): Windows .lnk shortcut creation via PowerShell COM"
```

---

### Task 6: macOS shortcuts — `.app` bundle

**Files:**
- Create: `internal/trayctl/shortcut_darwin.go`

- [ ] **Step 1: Create the macOS shortcut implementation**

```go
//go:build darwin

package trayctl

import (
	"fmt"
	"os"
	"path/filepath"
)

func (m *Manager) CreateShortcuts() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	appDir := filepath.Join(home, "Applications", "SAP Devs Tray.app", "Contents")
	macosDir := filepath.Join(appDir, "MacOS")
	resDir := filepath.Join(appDir, "Resources")

	for _, d := range []string{macosDir, resDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	symlinkPath := filepath.Join(macosDir, "sap-devs-tray")
	_ = os.Remove(symlinkPath)
	if err := os.Symlink(m.BinaryPath(), symlinkPath); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}

	icnsSource := filepath.Join(m.binDir(), "icon.icns")
	icnsDest := filepath.Join(resDir, "AppIcon.icns")
	if data, err := os.ReadFile(icnsSource); err == nil {
		_ = os.WriteFile(icnsDest, data, 0644)
	}

	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>SAP Devs Tray</string>
	<key>CFBundleIdentifier</key>
	<string>com.sap-devs.tray</string>
	<key>CFBundleExecutable</key>
	<string>sap-devs-tray</string>
	<key>CFBundleIconFile</key>
	<string>AppIcon</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>LSUIElement</key>
	<true/>
	<key>LSBackgroundOnly</key>
	<false/>
</dict>
</plist>`

	return os.WriteFile(filepath.Join(appDir, "Info.plist"), []byte(plist), 0644)
}

func (m *Manager) RemoveShortcuts() error {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	_ = os.RemoveAll(filepath.Join(home, "Applications", "SAP Devs Tray.app"))
	_ = os.Remove(filepath.Join(m.binDir(), "icon.icns"))
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `GOOS=darwin go build ./internal/trayctl/`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/trayctl/shortcut_darwin.go
git commit -m "feat(tray): macOS .app bundle creation in ~/Applications/"
```

---

### Task 7: Linux shortcuts — `.desktop` files

**Files:**
- Create: `internal/trayctl/shortcut_linux.go`

- [ ] **Step 1: Create the Linux shortcut implementation**

```go
//go:build linux

package trayctl

import (
	"fmt"
	"os"
	"path/filepath"
)

func (m *Manager) CreateShortcuts() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	iconPath := filepath.Join(m.binDir(), "sap-devs-tray.png")
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=SAP Devs Tray
Comment=SAP developer tools system tray companion
Exec=%s
Icon=%s
Terminal=false
Categories=Development;
StartupNotify=false
`, m.BinaryPath(), iconPath)

	appsDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(appsDir, "sap-devs-tray.desktop"), []byte(entry), 0644); err != nil {
		return err
	}

	desktopDir := filepath.Join(home, "Desktop")
	if info, err := os.Stat(desktopDir); err == nil && info.IsDir() {
		desktopFile := filepath.Join(desktopDir, "sap-devs-tray.desktop")
		if err := os.WriteFile(desktopFile, []byte(entry), 0755); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) RemoveShortcuts() error {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	_ = os.Remove(filepath.Join(home, ".local", "share", "applications", "sap-devs-tray.desktop"))
	_ = os.Remove(filepath.Join(home, "Desktop", "sap-devs-tray.desktop"))
	_ = os.Remove(filepath.Join(m.binDir(), "sap-devs-tray.png"))
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `GOOS=linux go build ./internal/trayctl/`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/trayctl/shortcut_linux.go
git commit -m "feat(tray): Linux .desktop file creation for app launchers"
```

---

### Task 8: Wire shortcuts into `Install()`

**Files:**
- Modify: `internal/trayctl/manager.go` (Install method)

- [ ] **Step 1: Add `CreateShortcuts()` call at the end of `Install()` in `manager.go`**

After the asset extraction loop (the new code from Task 4), add before the closing `return nil`:

```go
	if err := m.CreateShortcuts(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create shortcuts: %v\n", err)
	}
	return nil
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/trayctl/`
Expected: clean build

- [ ] **Step 3: Verify the full CLI builds**

Run: `go build ./...`
Expected: clean build

- [ ] **Step 4: Commit**

```bash
git add internal/trayctl/manager.go
git commit -m "feat(tray): wire CreateShortcuts into Install"
```

---

### Task 9: Icon assets

**Files:**
- Create: `cmd/sap-devs-tray/assets/icon.png` (placeholder — real asset needs design)
- Create: `cmd/sap-devs-tray/assets/icon.ico` (placeholder)
- Create: `cmd/sap-devs-tray/assets/icon.icns` (placeholder)

- [ ] **Step 1: Create the assets directory**

```bash
mkdir -p cmd/sap-devs-tray/assets
```

- [ ] **Step 2: Create placeholder icon files**

For now, create minimal valid placeholder files. Real icons will be provided by design. A 1x1 PNG is sufficient to validate the pipeline:

```bash
# Create a minimal 1x1 transparent PNG (67 bytes)
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\nIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01\r\n\xb4\x00\x00\x00\x00IEND\xaeB`\x82' > cmd/sap-devs-tray/assets/icon.png
```

For `.ico` and `.icns`, create empty files as placeholders (the pipeline will work, shortcuts will just show default icons):

```bash
touch cmd/sap-devs-tray/assets/icon.ico
touch cmd/sap-devs-tray/assets/icon.icns
```

- [ ] **Step 3: Add a README in the assets folder**

Create `cmd/sap-devs-tray/assets/README.md`:

```markdown
# Tray Icon Assets

- `icon.png` — 1024x1024 master PNG (source of truth)
- `icon.ico` — Windows ICO (multi-resolution: 16/32/48/256)
- `icon.icns` — macOS ICNS

Current files are placeholders. Replace with real assets before release.
```

- [ ] **Step 4: Commit**

```bash
git add cmd/sap-devs-tray/assets/
git commit -m "chore(tray): add placeholder icon assets for release pipeline"
```

---

### Task 10: Release pipeline — Windows GUI subsystem + icon packaging

**Files:**
- Modify: `.github/workflows/release-tray.yml`

- [ ] **Step 1: Add `-H windowsgui` to the Windows build step**

In the Build step (line 73), change the `go build` command to conditionally include the ldflag:

Replace the entire Build step `run:` block with:

```bash
EXT=""
EXTRA_LDFLAGS=""
if [ "${{ matrix.goos }}" = "windows" ]; then
  EXT=".exe"
  EXTRA_LDFLAGS="-H windowsgui"
fi
go build -ldflags "-X main.version=${VERSION} ${EXTRA_LDFLAGS}" -o "sap-devs-tray${EXT}" .
```

- [ ] **Step 2: Add a step to copy platform-specific icon into build output**

Add after the "Prepare tray build assets" step and before "Build":

```yaml
      - name: Copy platform icon
        shell: bash
        run: |
          if [ "${{ matrix.goos }}" = "windows" ]; then
            cp cmd/sap-devs-tray/assets/icon.ico cmd/sap-devs-tray/sap-devs-tray.ico
          elif [ "${{ matrix.goos }}" = "darwin" ]; then
            cp cmd/sap-devs-tray/assets/icon.icns cmd/sap-devs-tray/icon.icns
          else
            cp cmd/sap-devs-tray/assets/icon.png cmd/sap-devs-tray/sap-devs-tray.png
          fi
```

- [ ] **Step 3: Update the Package step to include icon files**

Replace the Package step `run:` block:

```bash
ASSET="sap-devs-tray_${VERSION}_${{ matrix.goos }}_${{ matrix.goarch }}"
if [ "${{ matrix.goos }}" = "windows" ]; then
  7z a "${ASSET}.zip" ./cmd/sap-devs-tray/sap-devs-tray.exe ./cmd/sap-devs-tray/sap-devs-tray.ico
elif [ "${{ matrix.goos }}" = "darwin" ]; then
  tar czf "${ASSET}.tar.gz" -C cmd/sap-devs-tray sap-devs-tray icon.icns
else
  tar czf "${ASSET}.tar.gz" -C cmd/sap-devs-tray sap-devs-tray sap-devs-tray.png
fi
```

- [ ] **Step 4: Verify YAML is valid**

Run: `yq '.' .github/workflows/release-tray.yml > /dev/null`
Expected: no error

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/release-tray.yml
git commit -m "ci(tray): add -H windowsgui for Windows, package icons in release archives"
```

---

### Task 11: Update `Uninstall()` to remove all bin dir assets

**Files:**
- Modify: `internal/trayctl/manager.go` (Uninstall method)

- [ ] **Step 1: Change Uninstall to remove the entire binDir instead of just the binary**

The current `Uninstall()` does `os.Remove(m.BinaryPath())`. Since the bin dir now contains binary + icon, and the tray owns that directory, remove the whole directory:

```go
func (m *Manager) Uninstall() error {
	_ = m.RemoveShortcuts()
	_ = m.Stop()
	return os.RemoveAll(m.binDir())
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/trayctl/`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/trayctl/manager.go
git commit -m "fix(tray): Uninstall removes entire bin dir (binary + icons)"
```

---

### Task 12: Final integration verification

- [ ] **Step 1: Full build check**

Run: `go build ./...`
Expected: clean build of all packages

- [ ] **Step 2: Vet check**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 3: Cross-compilation check (Windows from any platform)**

Run: `GOOS=windows GOARCH=amd64 go build ./internal/trayctl/`
Expected: clean build

- [ ] **Step 4: Cross-compilation check (Darwin)**

Run: `GOOS=darwin GOARCH=arm64 go build ./internal/trayctl/`
Expected: clean build

- [ ] **Step 5: Cross-compilation check (Linux)**

Run: `GOOS=linux GOARCH=amd64 go build ./internal/trayctl/`
Expected: clean build

- [ ] **Step 6: Commit any remaining fixes**

If any compilation issues were found and fixed, commit them.

---

### Task 13: Documentation updates

**Files:**
- Modify: `CLAUDE.md` (Tray Companion section)

- [ ] **Step 1: Update the Tray Companion section in CLAUDE.md**

In the `### Tray Companion (Experimental)` section, update to mention shortcut management:

Add after the sentence ending with `Config key: config.Tray.Autostart.`:

```
`shortcut_windows.go` / `shortcut_darwin.go` / `shortcut_linux.go` handle native app shortcuts (Windows `.lnk`, macOS `.app` bundle, Linux `.desktop` files) — created during install, removed during uninstall. The release pipeline ships platform-specific icons alongside the binary and sets the Windows PE subsystem to GUI (`-H windowsgui`) to prevent terminal window allocation.
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with tray shortcut and headless launch details"
```
