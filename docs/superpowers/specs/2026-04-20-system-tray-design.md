# System Tray & GUI Companion Design

**Date:** 2026-04-20
**Status:** Approved
**Depends on:** OS-native scheduler (independent), existing sync/inject commands

## Overview

Add an optional GUI companion to `sap-devs` — a system tray icon with a Fiori-themed webview dashboard panel. The tray binary is separate from the main CLI, downloaded on demand via `sap-devs tray install`, and built with Wails v3. An independent OS-native scheduler handles background sync/inject regardless of whether the tray is installed.

## Key Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Framework | Wails v3 (alpha) | Integrated tray + webview + window management; superior visual experience; potential to evolve into a full desktop app |
| Binary strategy | Separate tray binary, downloaded from GitHub Releases | Main CLI stays CGO-free; tray is opt-in; clean distribution |
| Scheduler | OS-native (systemd/launchd/Task Scheduler) | Works without tray; robust; survives reboots |
| Panel UI | Embedded webview with SAP Fundamental Styles | Authentic Fiori look; offline via embed.FS; light/dark theme |
| Theming | `sap_horizon` (light) + `sap_horizon_dark` (dark) | Auto-detects OS preference via `prefers-color-scheme`; high-contrast variants available |
| Tray ↔ CLI communication | Shared filesystem (JSON state files) | Simple, no IPC sockets needed for v1 |
| Future GUI | Tray as GUI host process | Additional webview windows launched from tray menu or CLI |

## Architecture

Four independent components that compose but never depend on each other:

```
┌──────────────────────────────────────────────────────────┐
│  1. sap-devs (main CLI)           CGO_ENABLED=0          │
│     - All existing commands                               │
│     - NEW: tray install/uninstall/start/stop/status       │
│     - NEW: service install/uninstall/status               │
│     - Downloads tray binary from GitHub Releases          │
│     - Registers/unregisters OS scheduler entries          │
└───────────────┬──────────────────────┬───────────────────┘
                │ downloads            │ registers
                ▼                      ▼
┌───────────────────────────┐  ┌──────────────────────────┐
│ 2. sap-devs-tray          │  │ 3. OS Scheduler          │
│    (Wails v3, optional)   │  │    (independent)         │
│    - System tray icon     │  │    - systemd timer (Lin) │
│    - Webview panel/popup  │  │    - launchd plist (mac) │
│    - Embedded HTTP server │  │    - Task Scheduler (Win)│
│    - Future GUI windows   │  │    - Runs: sap-devs sync │
│    - Reads shared state   │  │      + sap-devs inject   │
│    CGO per-platform       │  │    - Configurable interval│
└───────────────────────────┘  └──────────────────────────┘
                │ both read/write
                ▼
┌──────────────────────────────────────────────────────────┐
│ 4. Shared State (filesystem)                              │
│    ~/.cache/sap-devs/sync-state.json  (sync timestamps)   │
│    ~/.cache/sap-devs/tray-state.json  (tray status)       │
│    ~/.cache/sap-devs/daemon.log       (scheduler log)     │
│    ~/.config/sap-devs/config.yaml     (all settings)      │
└──────────────────────────────────────────────────────────┘
```

**Key invariant:** Remove component 2 or 3, and the others still work perfectly. The CLI is always the source of truth. The tray is a viewer. The scheduler is a cron job.

**Communication:** The tray binary watches state files for changes (filesystem polling or fsnotify). When the CLI runs sync/inject (whether manually or via scheduler), it updates `sync-state.json`. The tray reads this to display status. No IPC sockets needed for v1.

## CLI Commands

### `sap-devs tray` — manage the optional GUI companion

| Subcommand | Purpose |
|---|---|
| `tray install` | Download tray binary, verify, optionally register OS autostart |
| `tray uninstall` | Remove autostart, stop tray, delete binary |
| `tray start` | Launch the tray process (if not already running) |
| `tray stop` | Stop the running tray process |
| `tray status` | Show: installed? running? autostart enabled? version? |

**`tray install` flow:**

1. Determine CLI version; if dev build (no version tag), print "Tray install requires a release build of sap-devs" and exit
2. Download correct tray binary from GitHub Releases (by OS/arch/version)
3. Verify checksum (from `checksums.txt` in release)
4. Run `sap-devs-tray --version` health check
5. Prompt: "Would you like sap-devs-tray to start automatically when you log in?" (Y/n)
6. If yes, register OS autostart entry

**`tray uninstall` flow:**
1. Remove OS autostart entry
2. Stop running tray process
3. Delete the tray binary

### `sap-devs service` — manage the OS-native scheduler

| Subcommand | Purpose |
|---|---|
| `service install` | Register OS scheduler entry |
| `service uninstall` | Remove the scheduler entry |
| `service status` | Show: installed? last run? next run? interval? |

### Config keys (in `config.yaml`)

```yaml
tray_autostart: true          # written by `tray install` when user opts in
service_interval: 6h          # sync+inject frequency, used by `service install`
```

Note: there is no `service_enabled` key. Whether the scheduler is active is determined by whether the OS entry exists (queried by `service status`). `service install` creates it; `service uninstall` removes it. The config only stores preferences that feed into those commands.

## Tray Binary Architecture (Wails v3)

### Directory structure

```
cmd/sap-devs-tray/
├── go.mod                    # Separate module: imports Wails v3
├── go.sum
├── main.go                   # Wails v3 app entry point
├── app.go                    # App struct (tray setup, menu, event handlers)
├── state.go                  # Read shared state files (sync-state, config)
├── server.go                 # Embedded HTTP server for webview content
└── frontend/
    ├── index.html            # Panel dashboard
    ├── config.html           # Future: config editor
    ├── content.html          # Future: content browser
    ├── css/
    │   ├── fundamental-styles.min.css
    │   ├── sap_horizon.css          # Light theme variables
    │   └── sap_horizon_dark.css     # Dark theme variables
    └── js/
        └── app.js            # Panel logic, state polling, actions
```

### Wails v3 app lifecycle

1. `main.go` creates the Wails application with `application.New()`
2. Registers the system tray with icon, tooltip, and menu
3. Starts an embedded HTTP server bound to `127.0.0.1` only (not `0.0.0.0`) on a random port, serving `frontend/` via `embed.FS`. A per-session random token is generated at startup; all API requests must include it as a `Bearer` token or query parameter. This prevents other local processes or malicious web pages from calling the tray's API.
4. Left-click on tray icon → opens a webview window pointed at `http://127.0.0.1:<port>/?token=<session-token>`
5. Webview window sized as a popup panel (~400×550px), positioned near tray icon
6. State refresh: frontend JS polls `/api/state` every 30 seconds (includes session token)

### Tray menu (right-click)

```
sap-devs v1.2.3
─────────────────
✓ Up to date (synced 2h ago)
Profile: CAP Developer
─────────────────
↻ Sync Now
⟳ Inject Now
─────────────────
Open Dashboard...        (opens panel)
Open Terminal...         (launches platform default: PowerShell on Windows, Terminal.app on macOS, $TERMINAL or xterm on Linux)
─────────────────
Quit
```

### Panel dashboard (left-click webview)

The panel displays:
- **Header:** sap-devs branding with version
- **Sync Status card:** Last synced time, next sync time, active pack count
- **Active Profile card:** Current profile name and pack list
- **Injected Tools card:** Per-tool injection status (green = injected, gray = not detected)
- **Action buttons:** Sync Now, Inject Now

### Theming

Uses SAP Fundamental Styles (`fundamental-styles` npm package) with `@sap-theming/theming-base-content` CSS variables, bundled into the binary via `embed.FS`.

**Themes:**
- `sap_horizon` — light mode
- `sap_horizon_dark` — dark mode
- `sap_horizon_hcb` / `sap_horizon_hcw` — high contrast (accessibility, opt-in via settings)

**OS detection:** `@media (prefers-color-scheme: dark)` auto-detects OS preference. Theme switches live when OS setting changes.

**Components:** Use Fundamental Styles classes (`fd-button`, `fd-card`, `fd-list`, `fd-object-status`, `fd-toolbar`, etc.) for authentic Fiori appearance.

## OS-Native Scheduler

### Per-platform implementation

| Platform | Mechanism | File/entry |
|---|---|---|
| Windows | Task Scheduler | Task: `sap-devs-sync` (via `schtasks`) |
| macOS | launchd | `~/Library/LaunchAgents/com.sap-devs.sync.plist` |
| Linux | systemd user timer | `~/.config/systemd/user/sap-devs-sync.{service,timer}` |

### Implementation

New package `internal/service/` with platform interface:

```go
type Scheduler interface {
    Install(interval time.Duration, binaryPath string) error
    Uninstall() error
    Status() (*Status, error)
}
```

Three implementations behind build tags: `scheduler_windows.go`, `scheduler_darwin.go`, `scheduler_linux.go`. All use `os/exec` to call OS tools — no CGO, no new dependencies.

Scheduler logs output to `~/.cache/sap-devs/daemon.log` (truncated on each run — each scheduled invocation overwrites the log file so it stays bounded without needing a rotation library).

## Build & Distribution

### GoReleaser configuration

The main CLI build is unchanged (`CGO_ENABLED=0`, cross-compiled from any runner). The tray binary requires CGO and platform-specific toolchains, so it **cannot be cross-compiled** via a single GoReleaser run.

**Strategy: per-platform CI matrix.** GitHub Actions builds the tray binary natively on each OS:

```yaml
# .github/workflows/release.yml (simplified)
jobs:
  release-cli:
    # Existing: GoReleaser builds sap-devs (CGO_ENABLED=0, all platforms)
    runs-on: ubuntu-latest
    steps:
      - uses: goreleaser/goreleaser-action@v6

  release-tray:
    strategy:
      matrix:
        include:
          - os: ubuntu-latest
            goos: linux
            goarch: amd64
          - os: ubuntu-latest     # cross-compile arm64 via apt crossbuild-essential
            goos: linux
            goarch: arm64
          - os: macos-latest      # has Xcode
            goos: darwin
            goarch: arm64
          - os: macos-13          # Intel runner
            goos: darwin
            goarch: amd64
          - os: windows-latest    # has MSVC
            goos: windows
            goarch: amd64
    runs-on: ${{ matrix.os }}
    steps:
      - name: Install deps (Linux)
        if: runner.os == 'Linux'
        run: sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
      - name: Build tray binary
        run: CGO_ENABLED=1 go build -o sap-devs-tray ./cmd/sap-devs-tray/
      - name: Upload release asset
        # Attach to the same GitHub Release created by release-cli
```

The main CLI `.goreleaser.yml` is unchanged — only `sap-devs` is built there. The tray binaries are uploaded as additional release assets by the matrix job. `windows/arm64` is excluded (matching the main CLI).

### Release assets

Each GitHub Release includes both binaries per platform:

```
sap-devs_1.2.3_windows_amd64.zip          (CLI, ~28MB)
sap-devs-tray_1.2.3_windows_amd64.zip     (tray, ~30-40MB)
...per OS/arch...
```

### Download flow (`tray install`)

1. Determine current OS/arch and CLI version
2. Construct asset URL from GitHub Releases
3. Download to `~/.cache/sap-devs/bin/sap-devs-tray` (or `%LOCALAPPDATA%/sap-devs/bin/` on Windows)
4. Verify checksum from `checksums.txt`
5. Set executable permission
6. Run `sap-devs-tray --version` to verify
7. Prompt for autostart registration

### CI requirements

- macOS: Xcode (GitHub Actions `macos-latest` has it)
- Linux: GTK/WebKit dev headers for Wails webview (install via `apt-get`)
- Windows: MSVC (GitHub Actions `windows-latest` has it)
- Main CLI build is unaffected — same `CGO_ENABLED=0` as today

### Version coupling

Tray binary version must match CLI version. `sap-devs update` checks both; if the tray is installed, it re-runs the `tray install` download/verify flow for the new version after updating the CLI. `tray status` warns on version mismatch.

## Alpha Dependency Guardrails

### Isolation guarantees

1. **Zero import coupling.** The main `go.mod` never imports Wails. The tray binary has its own `go.mod` under `cmd/sap-devs-tray/`. If Wails v3 breaks, the CLI is untouched.

2. **Feature completeness without tray.** Every feature the tray surfaces (sync status, profile info, inject state) is already available via CLI commands. The tray is a viewer, never a gatekeeper.

3. **Scheduler independence.** `sap-devs service install` works on day one without the tray binary. Background sync/inject is an OS-level scheduled task.

4. **Graceful failure modes:**
   - `tray install` on unsupported platform → clear message, no crash
   - `tray start` when not installed → "Run `sap-devs tray install` first"
   - Tray binary crashes → CLI and scheduler continue. Next tray start recovers.
   - Wails v3 API breaks between alphas → tray may fail to build; CI catches this; CLI release proceeds independently

5. **Version gating.** `tray install` prints:
   ```
   Note: The sap-devs tray companion uses Wails v3 (currently in alpha).
   This is an optional enhancement — all CLI features work without it.
   If you encounter issues, run `sap-devs tray uninstall` to remove it.
   ```

### Documentation requirements

- **CLAUDE.md:** Architecture section notes tray binary is Wails v3 alpha, separate go.mod, optional
- **README:** "Optional GUI Companion" section with install instructions and alpha disclaimer
- **`sap-devs tray --help`:** Alpha notice in long description
- **Release notes:** Tray assets marked as experimental

### Upgrade path

When Wails v3 reaches stable, remove alpha disclaimers. Architecture doesn't change.

## File Structure (new/modified files)

### New files in main CLI

```
cmd/tray.go                         # tray install/uninstall/start/stop/status commands
cmd/service.go                      # service install/uninstall/status commands
internal/service/scheduler.go       # Scheduler interface + Status type
internal/service/scheduler_windows.go
internal/service/scheduler_darwin.go
internal/service/scheduler_linux.go
internal/trayctl/manager.go         # Tray lifecycle (download, verify, start, stop)
```

### New files in tray binary

```
cmd/sap-devs-tray/
├── go.mod
├── go.sum
├── main.go
├── app.go
├── state.go
├── server.go
└── frontend/
    ├── index.html
    ├── css/
    │   ├── fundamental-styles.min.css
    │   ├── sap_horizon.css
    │   └── sap_horizon_dark.css
    └── js/
        └── app.js
```

### Modified files

```
.goreleaser.yml                     # Add sap-devs-tray build target
internal/config/config.go           # Add tray_autostart, service_interval keys
CLAUDE.md                           # Document tray architecture
```

## Future Extensions

The tray binary is designed as the GUI host for future graphical features:

- **Config editor:** Webview window with form-based config editing (mirrors TUI `config edit`)
- **Content browser:** Webview window to browse/edit pack content (mirrors TUI `content edit`)
- **Notification center:** Desktop notifications on sync completion, new content, or errors
- **Mini dashboard:** Expanded panel with learning progress, recent tips, upcoming events
- **Full desktop app:** If demand warrants it, the tray evolves into an SAP developer desktop companion with navigation, multiple views, etc.

Each future feature is a new HTML page served by the tray's embedded HTTP server, launched as a webview window from the tray menu.
