# GUI Config Editor Design

## Goal

Add a graphical config editor to the `sap-devs-tray` system tray companion, accessible from the tray context menu and from the dashboard panel. This is Phase 3 of the Content Editing UI (TODO.md), scoped to config editing only.

## Context

The TUI config editor (`sap-devs config edit`) already exists with 4 form groups and ~20 fields. The GUI version mirrors this functionality in a webview window using the same embedded HTTP server + Fiori-styled frontend approach as the existing dashboard panel.

## Architecture

### Window Behavior

A separate webview window (not embedded in the dashboard panel):

- **Size:** ~520×700 pixels, resizable
- **Title bar:** Standard OS window chrome (not frameless)
- **Close behavior:** Standard close button, no hide-on-focus-loss
- **Opened from:** "Config" item in tray context menu, or a link/button in the dashboard panel
- **URL:** `config.html?token=TOKEN` served by the existing embedded HTTP server

### Backend (Go)

New file `cmd/sap-devs-tray/config.go` with API handlers. All endpoints use the existing `requireToken` middleware.

**Config read/write:**

| Endpoint | Method | Purpose |
| --- | --- | --- |
| `/api/config` | GET | Returns current config as JSON |
| `/api/config` | POST | Validates and saves config; returns field-level errors or `{status: "ok"}` |

**Input assistance:**

| Endpoint | Method | Purpose |
| --- | --- | --- |
| `/api/cities?q=<prefix>` | GET | Returns top 10 city matches from cities.json (case-insensitive prefix on city name) |
| `/api/languages` | GET | Returns supported language codes (discovered from i18n catalogs) |
| `/api/detect-location` | POST | Calls ip-api.com/json, returns `{city, country}` or error |

**Service & tray actions:**

| Endpoint | Method | Purpose |
| --- | --- | --- |
| `/api/service-status` | GET | Returns `{scheduler: {installed, lastRun, nextRun}, autostart: {installed}}` |
| `/api/service-install` | POST | Runs `sap-devs service install`, returns result |
| `/api/service-uninstall` | POST | Runs `sap-devs service uninstall`, returns result |
| `/api/autostart-install` | POST | Registers tray autostart via trayctl, returns result |
| `/api/autostart-uninstall` | POST | Unregisters tray autostart, returns result |

**Validation on POST /api/config:**

The backend validates all fields before writing to disk. On failure, returns HTTP 400 with:

```json
{
  "errors": {
    "company_repo": "Must be a valid URL (https://...)",
    "sync.tips": "Invalid duration format"
  }
}
```

On success, writes `config.yaml` and returns HTTP 200 with `{"status": "ok"}`.

### Frontend

New files in `cmd/sap-devs-tray/frontend/`:

- `config.html` — config editor page
- `js/config.js` — form logic, typeahead, validation, save
- Existing `css/app.css` extended with config-specific styles

Uses SAP Fundamental Styles components (`fd-panel`, `fd-form-item`, `fd-input`, `fd-select`, `fd-switch`, `fd-button`) with `sap_horizon` / `sap_horizon_dark` themes, auto-switching via `prefers-color-scheme` media queries.

### Embedding cities.json

The tray binary lives in a separate Go module and cannot import `internal/geo`. A copy of `cities.json` is embedded in the tray binary (e.g. `cmd/sap-devs-tray/data/cities.json` with `//go:embed`). The build script (`build.ps1`) or CI copies the file from `internal/geo/cities.json` at build time. CI can verify the copy stays in sync.

## UI Layout

Single scrolling form with 5 collapsible Fiori panels, all expanded by default. Sticky save bar at the bottom.

### Panel 1: General

| Field | Input Type | Validation |
| --- | --- | --- |
| Language | Select dropdown | Options from `/api/languages` + "(auto-detect from OS)" |
| Location | Text input with typeahead + Detect button | Autocomplete from `/api/cities`; soft warning if no match in city database |
| Experience Level | Select dropdown | `(not set)`, `beginner`, `intermediate`, `advanced` |
| Company Repo | Text input | Must be valid URL (`https://...`) or empty |

**Location typeahead:** On keyup (debounced 200ms), calls `GET /api/cities?q=<input>`. Backend returns top 10 matches. Frontend renders a dropdown list; clicking an entry fills the input as `"City, Country"`.

**Detect button:** Calls `POST /api/detect-location`. On success, populates the input with `"City, Country"`. On failure, shows inline error.

**Soft warning:** If the entered location text doesn't match any city in the database (checked client-side against typeahead results or via a validation response), show: "Location not found in city database — event filtering may not work." This does not block save.

### Panel 2: Preferences

| Field | Input Type | Validation |
| --- | --- | --- |
| Tip Rotation | Select dropdown | `Daily`, `Hourly`, `Session` |
| Interactive Tutorials | Fiori switch (fd-switch) | Boolean toggle |

### Panel 3: Events

Fields laid out in two-column rows where they pair naturally.

| Field | Input Type | Validation |
| --- | --- | --- |
| Local Radius | Number input + "km" suffix | Integer, > 0 |
| Regional Radius | Number input + "km" suffix | Integer, > 0 |
| Notify Days | Number input | Integer, > 0 |
| Notify Method | Select dropdown | `Hook`, `OS notification`, `Both` |

### Panel 4: Sync TTLs

Two-column grid layout to save vertical space. Help text at the bottom: "Go duration format: e.g. 24h, 168h, 4h30m".

| Field | Input Type | Validation |
| --- | --- | --- |
| Disable All Sync | Fiori switch | Boolean toggle |
| Tips TTL | Text input | Valid Go duration |
| Tools TTL | Text input | Valid Go duration |
| Resources TTL | Text input | Valid Go duration |
| Context TTL | Text input | Valid Go duration |
| Events TTL | Text input | Valid Go duration |
| YouTube TTL | Text input | Valid Go duration |
| Discovery TTL | Text input | Valid Go duration |
| Tutorials TTL | Text input | Valid Go duration |
| Advocates TTL | Text input | Valid Go duration |
| MCP TTL | Text input | Valid Go duration |
| Learning TTL | Text input | Valid Go duration |

### Panel 5: Service & Tray

Two sub-sections separated by a divider.

**Background Scheduler:**

- Status badge: "Installed" (green) or "Not Installed" (grey)
- When installed: shows Interval field (text input, Go duration validation) + "Uninstall Scheduler" button (red outline)
- When not installed: shows "Install Scheduler" button (primary blue)
- Install/uninstall are immediate actions (not part of Save flow)

**Tray Autostart:**

- Status badge: "Installed" (green) or "Not Installed" (grey)
- When installed: shows "Uninstall Autostart" button (red outline)
- When not installed: shows "Install Autostart" button (primary blue)
- Install/uninstall are immediate actions (not part of Save flow)

## Data Flow

### Load

1. User clicks "Config" in tray menu or dashboard link
2. Wails opens config window at `config.html?token=TOKEN`
3. Frontend calls `GET /api/config`, `GET /api/languages`, `GET /api/service-status` in parallel
4. Form populated with current values; Service & Tray panel renders conditionally based on install state

### Save

1. User clicks Save
2. Frontend runs client-side validation on all fields
3. If validation errors: inline error messages shown (red text below field, red border on input), save blocked
4. If clean: `POST /api/config` with full config JSON body
5. Backend validates all fields (duration parsing, URL format, integer ranges)
6. If backend validation errors: returns `{errors: {...}}` → frontend maps to field-level inline errors
7. If success: writes `config.yaml`, returns `{status: "ok"}` → frontend shows success message/toast

### Service/Tray Actions

These are immediate — not part of the Save flow:

1. User clicks install/uninstall button
2. Button shows loading state
3. Frontend calls appropriate `POST /api/service-*` or `POST /api/autostart-*` endpoint
4. Backend executes `sap-devs service install/uninstall` (subprocess) or trayctl autostart register/unregister
5. On success: frontend refreshes service status, updates badge and conditional fields
6. On failure: inline error message shown

### Location Detect

1. User clicks Detect button → button shows loading state
2. `POST /api/detect-location` → backend calls `ip-api.com/json` (3s timeout)
3. On success: returns `{city, country}` → frontend populates input as "City, Country"
4. On failure: inline message "Could not detect location"

## File Changes

| File | Change |
| --- | --- |
| `cmd/sap-devs-tray/config.go` | **New** — config API handlers: read, write, validate, detect-location, cities search, languages list, service/autostart status and actions |
| `cmd/sap-devs-tray/server.go` | Register new routes on the existing mux |
| `cmd/sap-devs-tray/app.go` | Add config webview window (520×700, titled, resizable); add "Config" menu item; wire `srv.configWindowFunc` |
| `cmd/sap-devs-tray/data/cities.json` | **New** — copy of `internal/geo/cities.json`, embedded via `//go:embed` |
| `cmd/sap-devs-tray/frontend/config.html` | **New** — config editor page with Fiori panels |
| `cmd/sap-devs-tray/frontend/js/config.js` | **New** — form population, typeahead, validation, save, service actions |
| `cmd/sap-devs-tray/frontend/css/app.css` | Add styles for config panels, form validation states, typeahead dropdown |
| `cmd/sap-devs-tray/frontend/index.html` | Add "Config" button in dashboard |
| `cmd/sap-devs-tray/frontend/js/app.js` | Add click handler to open config window |
| `build.ps1` | Add cities.json copy step before tray build |

## Constraints

- The tray binary is a separate Go module — it cannot import from the main CLI's `internal/` packages
- Service install/uninstall run `sap-devs` as a subprocess (same pattern as sync/inject)
- Wails v3 is alpha — keep the implementation straightforward
- Cities.json is copied at build time; CI should verify the copy matches the source
- The config editor does not handle profile switching (separate concern)
