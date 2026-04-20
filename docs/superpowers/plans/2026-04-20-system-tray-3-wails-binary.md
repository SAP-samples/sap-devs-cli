# Wails v3 Tray Binary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `sap-devs-tray` binary — a Wails v3 application that provides a system tray icon with right-click menu and a Fiori-themed webview dashboard panel.

**Architecture:** A separate Go module at `cmd/sap-devs-tray/` with its own `go.mod` importing Wails v3. The app registers a system tray, starts an embedded HTTP server (bound to `127.0.0.1`, session-token-protected), and opens a webview popup panel on left-click. Frontend uses SAP Fundamental Styles with `sap_horizon`/`sap_horizon_dark` themes, auto-switching via `prefers-color-scheme`.

**Tech Stack:** Wails v3 (alpha), SAP Fundamental Styles CSS, Go `embed.FS`, `net/http`

**Spec:** `docs/superpowers/specs/2026-04-20-system-tray-design.md`

**Prerequisites:** Plans 1 (OS scheduler) and 2 (tray lifecycle) should be completed first so that `sap-devs service status` and shared state files exist.

**Important:** This plan builds a separate Go module. All `go` commands in this plan run from `cmd/sap-devs-tray/`, not the repo root.

---

### Task 1: Initialize the Wails v3 module

**Files:**
- Create: `cmd/sap-devs-tray/go.mod`
- Create: `cmd/sap-devs-tray/main.go`

- [ ] **Step 1: Create the module directory**

```bash
mkdir -p cmd/sap-devs-tray
```

- [ ] **Step 2: Initialize go.mod**

```bash
cd cmd/sap-devs-tray
go mod init github.com/SAP-samples/sap-devs-cli/cmd/sap-devs-tray
```

- [ ] **Step 3: Add Wails v3 dependency**

```bash
cd cmd/sap-devs-tray
go get github.com/wailsapp/wails/v3@latest
```

Note: Pin to a specific alpha tag once confirmed working (e.g., `@v3.0.0-alpha.77`). Update the tag in `go.mod` after testing.

- [ ] **Step 4: Write minimal main.go**

Create `cmd/sap-devs-tray/main.go`:

```go
package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("sap-devs-tray %s\n", version)
		os.Exit(0)
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Wails app initialization will go here
	fmt.Println("sap-devs-tray starting... (skeleton)")
	select {} // block forever for now
}
```

- [ ] **Step 5: Verify it builds**

```bash
cd cmd/sap-devs-tray && go build -o sap-devs-tray .
./sap-devs-tray --version
```

Expected: `sap-devs-tray dev`

- [ ] **Step 6: Commit**

```bash
git add cmd/sap-devs-tray/go.mod cmd/sap-devs-tray/go.sum cmd/sap-devs-tray/main.go
git commit -m "feat(tray-binary): initialize Wails v3 module with skeleton main"
```

---

### Task 2: Shared state reader

**Files:**
- Create: `cmd/sap-devs-tray/state.go`

- [ ] **Step 1: Write the state reader**

This reads the JSON state files written by the main CLI. Create `cmd/sap-devs-tray/state.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// DashboardState is the combined state served to the frontend via /api/state.
type DashboardState struct {
	Version     string       `json:"version"`
	Profile     ProfileState `json:"profile"`
	Sync        SyncState    `json:"sync"`
	Tools       []ToolState  `json:"tools"`
	ServiceUp   bool         `json:"serviceUp"`
}

type ProfileState struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Packs []string `json:"packs"`
}

type SyncState struct {
	LastSynced time.Time `json:"lastSynced"`
	NextSync   time.Time `json:"nextSync"`
	PackCount  int       `json:"packCount"`
	Status     string    `json:"status"` // "up_to_date", "stale", "syncing", "error"
}

type ToolState struct {
	Name     string `json:"name"`
	Injected bool   `json:"injected"`
}

// ReadState assembles the dashboard state from shared filesystem state.
func ReadState(configDir, cacheDir string) *DashboardState {
	state := &DashboardState{
		Version: version,
		Sync:    readSyncState(cacheDir),
		Profile: readProfile(configDir),
	}
	return state
}

func readSyncState(cacheDir string) SyncState {
	path := filepath.Join(cacheDir, "sync-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return SyncState{Status: "unknown"}
	}
	var raw map[string]time.Time
	if err := json.Unmarshal(data, &raw); err != nil {
		return SyncState{Status: "unknown"}
	}
	var latest time.Time
	count := 0
	for _, t := range raw {
		count++
		if t.After(latest) {
			latest = t
		}
	}
	st := SyncState{
		LastSynced: latest,
		PackCount:  count,
		Status:     "up_to_date",
	}
	if time.Since(latest) > 12*time.Hour {
		st.Status = "stale"
	}
	return st
}

func readProfile(configDir string) ProfileState {
	path := filepath.Join(configDir, "profile.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return ProfileState{ID: "unknown"}
	}
	var p struct {
		ID string `yaml:"id"`
	}
	if err := yaml.Unmarshal(data, &p); err != nil || p.ID == "" {
		return ProfileState{ID: "unknown"}
	}
	return ProfileState{ID: p.ID, Name: profileDisplayName(p.ID)}
}

func profileDisplayName(id string) string {
	names := map[string]string{
		"cap-developer":  "CAP Developer",
		"abap-developer": "ABAP Developer",
		"btp-developer":  "BTP Developer",
		"all":            "All Packs",
		"minimal":        "Minimal",
	}
	if name, ok := names[id]; ok {
		return name
	}
	return id
}

func defaultConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "sap-devs")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "sap-devs")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "sap-devs")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "sap-devs")
	}
}

func defaultCacheDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "sap-devs", "cache")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Caches", "sap-devs")
	default:
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			return filepath.Join(xdg, "sap-devs")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".cache", "sap-devs")
	}
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd cmd/sap-devs-tray && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add cmd/sap-devs-tray/state.go
git commit -m "feat(tray-binary): add shared state reader for dashboard"
```

---

### Task 3: Embedded HTTP server with session token

**Files:**
- Create: `cmd/sap-devs-tray/server.go`

- [ ] **Step 1: Write the server**

Create `cmd/sap-devs-tray/server.go`:

```go
package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
)

//go:embed frontend
var frontendFS embed.FS

// Server is the embedded HTTP server that serves the frontend and API.
type Server struct {
	Token     string
	ConfigDir string
	CacheDir  string
	listener  net.Listener
	mux       *http.ServeMux
}

func NewServer(configDir, cacheDir string) (*Server, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	s := &Server{
		Token:     token,
		ConfigDir: configDir,
		CacheDir:  cacheDir,
		mux:       http.NewServeMux(),
	}

	frontendContent, _ := fs.Sub(frontendFS, "frontend")
	s.mux.Handle("/", http.FileServer(http.FS(frontendContent)))
	s.mux.HandleFunc("/api/state", s.requireToken(s.handleState))
	s.mux.HandleFunc("/api/sync", s.requireToken(s.handleSync))
	s.mux.HandleFunc("/api/inject", s.requireToken(s.handleInject))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s.listener = listener
	return s, nil
}

func (s *Server) Port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *Server) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", s.Port())
}

func (s *Server) PanelURL() string {
	return fmt.Sprintf("%s/?token=%s", s.URL(), s.Token)
}

func (s *Server) Start() error {
	return http.Serve(s.listener, s.mux)
}

func (s *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if token != s.Token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	state := ReadState(s.ConfigDir, s.CacheDir)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	go func() {
		cmd := exec.Command(sapDevsBinary(), "sync")
		cmd.Stdout = nil
		cmd.Stderr = nil
		_ = cmd.Run()
	}()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *Server) handleInject(w http.ResponseWriter, r *http.Request) {
	go func() {
		cmd := exec.Command(sapDevsBinary(), "inject", "--no-sync")
		cmd.Stdout = nil
		cmd.Stderr = nil
		_ = cmd.Run()
	}()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func sapDevsBinary() string {
	name := "sap-devs"
	if runtime.GOOS == "windows" {
		name = "sap-devs.exe"
	}
	// Look in PATH first; the CLI should be installed already
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return name
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd cmd/sap-devs-tray && go build ./...
```

Expected: May fail if `frontend/` directory doesn't exist yet — create a placeholder:

```bash
mkdir -p cmd/sap-devs-tray/frontend
echo "<html><body>placeholder</body></html>" > cmd/sap-devs-tray/frontend/index.html
```

- [ ] **Step 3: Commit**

```bash
git add cmd/sap-devs-tray/server.go cmd/sap-devs-tray/frontend/index.html
git commit -m "feat(tray-binary): add embedded HTTP server with session token auth"
```

---

### Task 4: Frontend — Fiori dashboard with Fundamental Styles

**Files:**
- Create: `cmd/sap-devs-tray/frontend/index.html`
- Create: `cmd/sap-devs-tray/frontend/css/app.css`
- Create: `cmd/sap-devs-tray/frontend/js/app.js`

- [ ] **Step 1: Download Fundamental Styles CSS**

```bash
cd cmd/sap-devs-tray/frontend/css
# Download fundamental-styles and theming CSS variables
# These will be embedded in the binary — no CDN at runtime
curl -o fundamental-styles.min.css "https://unpkg.com/fundamental-styles@latest/dist/fundamental-styles.css"
curl -o sap_horizon.css "https://unpkg.com/@sap-theming/theming-base-content/content/Base/baseLib/sap_horizon/css_variables.css"
curl -o sap_horizon_dark.css "https://unpkg.com/@sap-theming/theming-base-content/content/Base/baseLib/sap_horizon_dark/css_variables.css"
```

- [ ] **Step 2: Write the dashboard HTML**

Replace `cmd/sap-devs-tray/frontend/index.html` with the full Fiori dashboard. This should use `fd-*` Fundamental Styles classes for buttons, cards, object-status, etc. Include:

- `<link>` to the CSS files (relative paths, served by embedded FS)
- A `<script>` block or `js/app.js` reference that:
  - Reads the `token` from the URL query parameter
  - Polls `/api/state?token=<token>` every 30 seconds
  - Renders sync status, profile, injected tools
  - Handles Sync Now / Inject Now button clicks
- `@media (prefers-color-scheme: dark)` to switch between `sap_horizon` and `sap_horizon_dark` variable imports

The HTML structure should match the mockup approved during brainstorming (header, sync status card, profile card, tools card, action buttons).

- [ ] **Step 3: Write app.css**

Create `cmd/sap-devs-tray/frontend/css/app.css` with panel-specific overrides:

```css
:root {
    --panel-width: 400px;
    --panel-max-height: 550px;
}

body {
    margin: 0;
    padding: 0;
    width: var(--panel-width);
    max-height: var(--panel-max-height);
    overflow-y: auto;
    font-family: '72', 'Segoe UI', system-ui, sans-serif;
}

.panel-header {
    padding: 16px 20px;
    display: flex;
    align-items: center;
    gap: 12px;
}

.status-card, .profile-card, .tools-card {
    margin: 0 16px 12px;
    padding: 14px;
    border-radius: 12px;
}

.action-buttons {
    margin: 0 16px 16px;
    display: flex;
    gap: 8px;
}

.action-buttons .fd-button {
    flex: 1;
}
```

- [ ] **Step 4: Write app.js**

Create `cmd/sap-devs-tray/frontend/js/app.js`:

```javascript
(function() {
    const params = new URLSearchParams(window.location.search);
    const token = params.get('token');

    async function fetchState() {
        try {
            const resp = await fetch(`/api/state?token=${token}`);
            if (!resp.ok) return;
            const state = await resp.json();
            renderState(state);
        } catch (e) {
            console.error('Failed to fetch state:', e);
        }
    }

    function renderState(state) {
        // Update sync status
        const lastSynced = state.sync.lastSynced
            ? timeAgo(new Date(state.sync.lastSynced))
            : 'Never';
        document.getElementById('last-synced').textContent = lastSynced;
        document.getElementById('pack-count').textContent = state.sync.packCount || '—';
        document.getElementById('sync-status-badge').textContent =
            state.sync.status === 'up_to_date' ? 'Up to Date' : 'Stale';

        // Update profile
        document.getElementById('profile-name').textContent = state.profile.name || state.profile.id;
        document.getElementById('profile-packs').textContent =
            (state.profile.packs || []).join(', ') || '—';

        // Update version
        document.getElementById('version').textContent = 'v' + state.version;
    }

    function timeAgo(date) {
        const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
        if (seconds < 60) return 'just now';
        const minutes = Math.floor(seconds / 60);
        if (minutes < 60) return minutes + 'm ago';
        const hours = Math.floor(minutes / 60);
        if (hours < 24) return hours + 'h ago';
        const days = Math.floor(hours / 24);
        return days + 'd ago';
    }

    // Action handlers
    document.addEventListener('DOMContentLoaded', () => {
        document.getElementById('btn-sync')?.addEventListener('click', async () => {
            await fetch(`/api/sync?token=${token}`, { method: 'POST' });
            setTimeout(fetchState, 2000);
        });
        document.getElementById('btn-inject')?.addEventListener('click', async () => {
            await fetch(`/api/inject?token=${token}`, { method: 'POST' });
            setTimeout(fetchState, 2000);
        });

        fetchState();
        setInterval(fetchState, 30000);
    });
})();
```

- [ ] **Step 5: Verify it builds**

```bash
cd cmd/sap-devs-tray && go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add cmd/sap-devs-tray/frontend/
git commit -m "feat(tray-binary): add Fiori dashboard with Fundamental Styles theming"
```

---

### Task 5: Wails v3 app — tray icon and webview panel

**Files:**
- Create: `cmd/sap-devs-tray/app.go`
- Modify: `cmd/sap-devs-tray/main.go`

- [ ] **Step 1: Write app.go — Wails v3 tray setup**

Create `cmd/sap-devs-tray/app.go`. Based on the Wails v3 systray-menu example (`v3/examples/systray-menu/main.go`), the API uses `app.SystemTray.New()`, `app.Window.NewWithOptions()`, and `app.NewMenu()`. Pin the Wails v3 alpha version after confirming this compiles.

```go
package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed frontend/icon.png
var trayIcon []byte

func startApp(srv *Server) error {
	app := application.New(application.Options{
		Name:        "sap-devs",
		Description: "SAP Developer Companion",
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	// Create the webview panel window (hidden by default)
	panel := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "sap-devs Dashboard",
		Width:         400,
		Height:        550,
		URL:           srv.PanelURL(),
		Frameless:     true,
		AlwaysOnTop:   true,
		Hidden:        true,
		DisableResize: true,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
		},
	})

	// Hide instead of closing the panel window
	panel.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		panel.Hide()
		e.Cancel()
	})

	// Create system tray
	systemTray := app.SystemTray.New()
	systemTray.SetIcon(trayIcon)
	systemTray.SetTooltip("sap-devs")

	// Build tray menu
	menu := app.NewMenu()
	menu.Add(fmt.Sprintf("sap-devs %s", version)).SetEnabled(false)
	menu.AddSeparator()

	menu.Add("Sync Now").OnClick(func(ctx *application.Context) {
		go func() {
			cmd := exec.Command(sapDevsBinary(), "sync")
			_ = cmd.Run()
		}()
	})
	menu.Add("Inject Now").OnClick(func(ctx *application.Context) {
		go func() {
			cmd := exec.Command(sapDevsBinary(), "inject", "--no-sync")
			_ = cmd.Run()
		}()
	})

	menu.AddSeparator()
	menu.Add("Open Dashboard...").OnClick(func(ctx *application.Context) {
		panel.Show()
		panel.Focus()
	})
	menu.Add("Open Terminal...").OnClick(func(ctx *application.Context) {
		openTerminal()
	})

	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(ctx *application.Context) {
		app.Quit()
	})

	systemTray.SetMenu(menu)
	systemTray.AttachWindow(panel).WindowOffset(2)

	return app.Run()
}

func openTerminal() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("powershell", "-NoExit")
	case "darwin":
		cmd = exec.Command("open", "-a", "Terminal")
	default:
		// Try $TERMINAL, then x-terminal-emulator, then xterm
		if term := envOr("TERMINAL", ""); term != "" {
			cmd = exec.Command(term)
		} else if path, err := exec.LookPath("x-terminal-emulator"); err == nil {
			cmd = exec.Command(path)
		} else {
			cmd = exec.Command("xterm")
		}
	}
	_ = cmd.Start()
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
```

Note: The exact Wails v3 API may shift between alpha releases. If `app.SystemTray.New()` or `app.Window.NewWithOptions()` signatures change, consult the Wails v3 examples at `github.com/wailsapp/wails/tree/v3-alpha/v3/examples/systray-menu/main.go` for the current API.

- [ ] **Step 2: Update main.go to wire everything**

Update `cmd/sap-devs-tray/main.go` `run()` function:

```go
func run() error {
	configDir := defaultConfigDir()
	cacheDir := defaultCacheDir()

	srv, err := NewServer(configDir, cacheDir)
	if err != nil {
		return fmt.Errorf("could not start server: %w", err)
	}
	go srv.Start()

	return startApp(srv)
}
```

Where `startApp(srv)` is the function in `app.go` that creates the Wails application with the tray and webview.

- [ ] **Step 3: Add a tray icon**

Create or download an SVG/PNG icon for the tray. Store at `cmd/sap-devs-tray/frontend/icon.png` (or embed directly). The icon should be the sap-devs logo, small enough for system tray (~16x16 or ~22x22 depending on platform).

- [ ] **Step 4: Verify it builds and runs**

```bash
cd cmd/sap-devs-tray && go build -o sap-devs-tray . && ./sap-devs-tray
```

Expected: Tray icon appears in system tray. Right-click shows menu. Left-click opens webview panel.

- [ ] **Step 5: Commit**

```bash
git add cmd/sap-devs-tray/app.go cmd/sap-devs-tray/main.go cmd/sap-devs-tray/frontend/icon.png
git commit -m "feat(tray-binary): wire Wails v3 system tray with webview panel"
```

---

### Task 6: CI build configuration

**Files:**
- Create: `.github/workflows/release-tray.yml`

- [ ] **Step 1: Write the CI workflow**

Create `.github/workflows/release-tray.yml`:

```yaml
name: Release Tray Binary

on:
  release:
    types: [published]

permissions:
  contents: write

jobs:
  build-tray:
    strategy:
      fail-fast: false
      matrix:
        include:
          - os: ubuntu-latest
            goos: linux
            goarch: amd64
            cc: gcc
          - os: ubuntu-latest
            goos: linux
            goarch: arm64
            cc: aarch64-linux-gnu-gcc
          - os: macos-latest
            goos: darwin
            goarch: arm64
            cc: ""
          - os: macos-13
            goos: darwin
            goarch: amd64
            cc: ""
          - os: windows-latest
            goos: windows
            goarch: amd64
            cc: ""
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: cmd/sap-devs-tray/go.mod

      - name: Install Linux amd64 deps
        if: matrix.goos == 'linux' && matrix.goarch == 'amd64'
        run: sudo apt-get update && sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev

      - name: Install Linux arm64 cross-compile deps
        if: matrix.goos == 'linux' && matrix.goarch == 'arm64'
        run: |
          sudo dpkg --add-architecture arm64
          sudo apt-get update
          sudo apt-get install -y crossbuild-essential-arm64 \
            libgtk-3-dev:arm64 libwebkit2gtk-4.1-dev:arm64

      - name: Build
        working-directory: cmd/sap-devs-tray
        env:
          CGO_ENABLED: "1"
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CC: ${{ matrix.cc }}
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          EXT=""
          if [ "${{ matrix.goos }}" = "windows" ]; then EXT=".exe"; fi
          go build -ldflags "-X main.version=${VERSION}" -o "sap-devs-tray${EXT}" .

      - name: Package
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          ASSET="sap-devs-tray_${VERSION}_${{ matrix.goos }}_${{ matrix.goarch }}"
          if [ "${{ matrix.goos }}" = "windows" ]; then
            zip "${ASSET}.zip" -j cmd/sap-devs-tray/sap-devs-tray.exe
          else
            tar czf "${ASSET}.tar.gz" -C cmd/sap-devs-tray sap-devs-tray
          fi

      - name: Generate checksum
        shell: bash
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          ASSET="sap-devs-tray_${VERSION}_${{ matrix.goos }}_${{ matrix.goarch }}"
          if [ "${{ matrix.goos }}" = "windows" ]; then
            FILE="${ASSET}.zip"
          else
            FILE="${ASSET}.tar.gz"
          fi
          if command -v sha256sum &>/dev/null; then
            sha256sum "${FILE}" > "${FILE}.sha256"
          else
            shasum -a 256 "${FILE}" > "${FILE}.sha256"
          fi

      - name: Upload to Release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          ASSET="sap-devs-tray_${VERSION}_${{ matrix.goos }}_${{ matrix.goarch }}"
          if [ "${{ matrix.goos }}" = "windows" ]; then
            gh release upload "${GITHUB_REF_NAME}" "${ASSET}.zip" "${ASSET}.zip.sha256" --clobber
          else
            gh release upload "${GITHUB_REF_NAME}" "${ASSET}.tar.gz" "${ASSET}.tar.gz.sha256" --clobber
          fi

  aggregate-checksums:
    needs: build-tray
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Download per-artifact checksums
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          mkdir -p checksums
          for suffix in linux_amd64.tar.gz linux_arm64.tar.gz darwin_arm64.tar.gz darwin_amd64.tar.gz windows_amd64.zip; do
            FILE="sap-devs-tray_${VERSION}_${suffix}.sha256"
            gh release download "${GITHUB_REF_NAME}" -p "${FILE}" -D checksums || true
          done

      - name: Aggregate into checksums.txt
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          cat checksums/*.sha256 > tray-checksums.txt
          gh release upload "${GITHUB_REF_NAME}" tray-checksums.txt --clobber
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/release-tray.yml
git commit -m "ci: add tray binary build and release workflow"
```

---

### Task 7: Documentation and CLAUDE.md update

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add tray binary section to CLAUDE.md Architecture Overview**

Add after the "### Sync" section:

```markdown
### Tray Companion (Optional)

`sap-devs-tray` is an optional GUI companion built with Wails v3 (alpha). It lives in `cmd/sap-devs-tray/` with its own `go.mod` — the main CLI never imports Wails. The tray binary provides a system tray icon and a Fiori-themed webview dashboard panel using SAP Fundamental Styles (`sap_horizon` / `sap_horizon_dark` themes, auto-switching via OS preference).

**Architecture:** The tray reads shared state files (`sync-state.json`, `config.yaml`) written by the main CLI. An embedded HTTP server bound to `127.0.0.1` (session-token-protected) serves the dashboard frontend. The main CLI manages the tray lifecycle via `internal/trayctl/` (download from GitHub Releases, start/stop, autostart registration).

**Alpha disclaimer:** Wails v3 is in alpha. The tray is strictly optional — all CLI features work without it. If Wails v3 breaks, only the tray binary is affected.
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add tray binary architecture to CLAUDE.md"
```
