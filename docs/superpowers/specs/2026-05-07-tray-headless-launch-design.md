# Tray Headless Launch & App Shortcuts

**Date:** 2026-05-07
**Status:** Approved

## Problem

The `sap-devs-tray` binary on Windows opens a terminal window when launched at OS startup (via registry Run key) or manually from the CLI. Closing that terminal kills the tray. Additionally, there's no easy way to launch the tray manually on any platform without opening a terminal.

## Solution

A build-time + runtime fix to eliminate the terminal window, combined with native app shortcuts on all three platforms for manual launch.

## Design

### 1. Eliminating the Terminal Window (Windows)

**Build-time fix:**
- Add `-H windowsgui` to Go build ldflags in `release-tray.yml` for the Windows matrix entry
- Changes the PE header from `IMAGE_SUBSYSTEM_WINDOWS_CUI` (console) to `IMAGE_SUBSYSTEM_WINDOWS_GUI`
- Windows will never allocate a console regardless of how the binary is launched

**Runtime fix (`Start()` method):**
- New `internal/trayctl/start_windows.go` with build-tagged `startProcess()` function
- Uses `syscall.SysProcAttr{CreationFlags: 0x00000008 | 0x00000010}` (`DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP`)
- Ensures `sap-devs tray start` from a terminal doesn't inherit the parent console

**Default (`start_other.go`):**
- Non-Windows platforms keep current `cmd.Start()` + `cmd.Process.Release()` logic (Unix processes don't create terminal windows when spawned without a TTY)

**Registry entry unchanged:**
- Bare path to `sap-devs-tray.exe` in `HKCU\...\Run` continues to work once binary is GUI-subsystem

### 2. Desktop & Start Menu Shortcuts (Windows)

**Shortcut creation during `tray install`:**
- Creates `.lnk` files in two locations:
  - **Start Menu:** `%APPDATA%\Microsoft\Windows\Start Menu\Programs\SAP Devs Tray.lnk`
  - **Desktop:** resolved via `[Environment]::GetFolderPath('Desktop')` (handles OneDrive redirection)

**Implementation:**
- PowerShell `WScript.Shell` COM object creates `.lnk` files
- Desktop path resolved dynamically (not hard-coded `%USERPROFILE%\Desktop`) to handle enterprise OneDrive folder redirection
- Target: binary path, working directory: bin folder, icon: bundled `.ico`

**Icon:**
- `sap-devs-tray.ico` shipped in release archive, extracted to `~/.cache/sap-devs/bin/`
- `.lnk` references this icon path

**Cleanup on `tray uninstall`:**
- Remove both `.lnk` files and the `.ico`

### 3. macOS App Bundle & Launchpad Integration

**`.app` bundle structure created during `tray install`:**

```
~/Applications/SAP Devs Tray.app/
├── Contents/
│   ├── Info.plist
│   ├── MacOS/
│   │   └── sap-devs-tray   (symlink → ~/.cache/sap-devs/bin/sap-devs-tray)
│   └── Resources/
│       └── AppIcon.icns
```

**Info.plist key points:**
- `CFBundleName`: "SAP Devs Tray"
- `CFBundleIdentifier`: "com.sap-devs.tray"
- `CFBundleExecutable`: "sap-devs-tray"
- `LSUIElement`: `true` — hides from Dock while running (standard for menu-bar apps)
- `LSBackgroundOnly`: `false` — allows Launchpad visibility for manual launch

**Why `~/Applications/` (not `/Applications/`):**
- `/Applications` requires `sudo` or admin authentication on modern macOS
- `~/Applications/` is per-user, requires no elevation, still appears in Launchpad + Spotlight
- Consistent with Homebrew Cask per-user installs

**Integration:**
- `~/Applications/` placement gives automatic Launchpad + Spotlight visibility
- Users can drag to Dock for quick access

**Cleanup on `tray uninstall`:**
- Remove entire `SAP Devs Tray.app` bundle

### 4. Linux Desktop Integration

**Application launcher entry:**
- `~/.local/share/applications/sap-devs-tray.desktop`:

```ini
[Desktop Entry]
Type=Application
Name=SAP Devs Tray
Comment=SAP developer tools system tray companion
Exec=<binary-path>
Icon=<icon-path>
Terminal=false
Categories=Development;
StartupNotify=false
```

**Desktop shortcut:**
- Copy `.desktop` file to `~/Desktop/sap-devs-tray.desktop`
- `chmod +x` (required by some DEs)

**Relationship to autostart:**
- Existing `~/.config/autostart/sap-devs-tray.desktop` handles login startup (already correct)
- New `~/.local/share/applications/` entry is for app launcher visibility (GNOME Activities, KDE launcher, etc.)
- Two separate files per freedesktop spec

**Icon:**
- PNG (512x512) extracted to `~/.cache/sap-devs/bin/sap-devs-tray.png`

**Cleanup on `tray uninstall`:**
- Remove both `.desktop` files and the `.png`

### 5. Icon Asset Strategy & Release Pipeline

**Source assets (checked into repo at `cmd/sap-devs-tray/assets/`):**
- `icon.png` — 1024x1024 master PNG (source of truth)
- `icon.ico` — pre-built Windows ICO (multi-resolution: 16/32/48/256)
- `icon.icns` — pre-built macOS ICNS

**Why pre-build all formats:**
- Avoids runtime tool dependencies (`sips`, `convert`)
- Icons rarely change — committing all three is simpler than build-time conversion

**Release archive contents per platform:**
- Windows `.zip`: `sap-devs-tray.exe` + `sap-devs-tray.ico`
- macOS `.tar.gz`: `sap-devs-tray` + `icon.icns`
- Linux `.tar.gz`: `sap-devs-tray` + `sap-devs-tray.png`

**`release-tray.yml` changes:**
- Windows build: add `-H windowsgui` to ldflags
- Package step: include platform-specific icon
- Prepare step: copy icons from `assets/` into build output

**`Install()` changes in `manager.go`:**
- `extractBinary()` becomes `extractAssets()` — extracts all files in the archive to `m.binDir()` (archives are purpose-built, containing only binary + icon)
- After extraction: `CreateShortcuts()` (platform-dispatched, best-effort — errors logged as warnings, do not fail install)
- After shortcuts: `RegisterAutostart()` (existing)

**`Uninstall()` changes:**
- Call `RemoveShortcuts()` before removing binary (best-effort, errors logged)

### 6. File Organization

**Dispatch pattern rationale:**
- `start_windows.go` / `start_other.go` use **build tags** because `syscall.SysProcAttr` has platform-specific struct fields that don't compile cross-platform. This differs from `autostart.go` which uses `runtime.GOOS` switching — that works there because `exec.Command` and `os.WriteFile` are platform-neutral.
- `shortcut_windows.go` / `shortcut_darwin.go` / `shortcut_linux.go` use **build tags** because each platform's shortcut logic imports platform-specific packages or shells out to platform-specific tools with no shared code path.
- `shortcut.go` is unnecessary — each build-tagged file directly implements `CreateShortcuts()` and `RemoveShortcuts()` as methods on `*Manager`.

**New files:**

| File | Purpose |
|------|---------|
| `internal/trayctl/start_windows.go` | Build-tagged `startProcess()` with `DETACHED_PROCESS` flags |
| `internal/trayctl/start_other.go` | Build-tagged default `startProcess()` for Unix |
| `internal/trayctl/shortcut_windows.go` | `CreateShortcuts()` / `RemoveShortcuts()` — `.lnk` creation via PowerShell COM |
| `internal/trayctl/shortcut_darwin.go` | `CreateShortcuts()` / `RemoveShortcuts()` — `.app` bundle creation |
| `internal/trayctl/shortcut_linux.go` | `CreateShortcuts()` / `RemoveShortcuts()` — `.desktop` file in applications + Desktop |
| `cmd/sap-devs-tray/assets/icon.png` | Master icon (1024x1024) |
| `cmd/sap-devs-tray/assets/icon.ico` | Windows multi-res ICO |
| `cmd/sap-devs-tray/assets/icon.icns` | macOS ICNS |

**Modified files:**

| File | Change |
|------|--------|
| `internal/trayctl/manager.go` | `Start()` delegates to `startProcess()`. `Install()` calls `CreateShortcuts()`. `Uninstall()` calls `RemoveShortcuts()`. Extract logic handles icon files. |
| `.github/workflows/release-tray.yml` | `-H windowsgui` for Windows. Package icons alongside binary. |

**Method signatures:**

```go
// shortcut_{windows,darwin,linux}.go
func (m *Manager) CreateShortcuts() error
func (m *Manager) RemoveShortcuts() error

// start_windows.go / start_other.go
func startProcess(binaryPath string) error
```

**`Start()` simplification:**

```go
func (m *Manager) Start() error {
    if !m.IsInstalled() {
        return fmt.Errorf("tray is not installed — run `sap-devs tray install` first")
    }
    return startProcess(m.BinaryPath())
}
```
