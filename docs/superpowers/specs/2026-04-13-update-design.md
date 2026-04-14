# sap-devs update — Design Specification

## Goal

Add `sap-devs update` so developers can self-update the binary from GitHub Releases, and add a weekly background check that appends a one-line hint when a newer version is available.

## Commands

```sh
sap-devs update   # check for a newer release and install it if found
```

## Scope

This feature has two parts that ship together:

1. **Release pipeline** — goreleaser config + GitHub Actions release workflow that builds cross-platform binaries and publishes them to GitHub Releases on every `v*` tag push
2. **Update command + background check** — `cmd/update.go` for the manual flow, `internal/update/` for the core logic, and a background hint wired into the root command

## Release Pipeline

### goreleaser (`.goreleaser.yml`)

- Builds binaries for: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
- Sets version via `ldflags`: `-X github.tools.sap/developer-relations/sap-devs-cli/cmd.Version={{ .Version }}`
- Generates `checksums.txt` (SHA256) alongside the release assets
- Asset naming (goreleaser default): `sap-devs_<version>_<OS>_<arch>.tar.gz` (`.zip` for Windows)
- Archives contain the binary only (no README or LICENSE in archive)
- Changelog generated from git commits

### GitHub Actions release workflow (`.github/workflows/release.yml`)

- Trigger: `push` to tags matching `v*`
- Runner: `ubuntu-latest` (goreleaser cross-compiles via CGO_ENABLED=0)
- Steps: checkout (full history for changelog) → setup Go → run goreleaser
- Requires `GITHUB_TOKEN` (provided automatically by Actions)

## Architecture

### `internal/update/` — three files

#### `checker.go`

```go
type Release struct {
    Version string // e.g. "1.2.0" (no leading "v")
    TagName string // e.g. "v1.2.0"
}

// CheckLatest fetches the latest GitHub release and returns it along with
// whether it is newer than currentVersion. Returns nil, false, nil if the
// request fails (callers treat failure as "no update available").
func CheckLatest(repoURL, currentVersion string) (*Release, bool, error)
```

- Calls `https://api.github.com/repos/<owner>/<repo>/releases/latest`
- `repoURL` is the full repo URL (e.g. `https://github.tools.sap/developer-relations/sap-devs-cli`) — owner/repo are parsed from it
- Compares versions with `semver` using Go's `golang.org/x/mod/semver` (already a transitive dependency via Go toolchain) — strips leading `v` before comparing
- Returns `nil, false, nil` on any network or parse error (fail-open for background use)

#### `installer.go`

```go
// Install downloads the release asset for the current OS/arch, verifies its
// SHA256 checksum against checksums.txt, and atomically replaces the running binary.
func Install(repoURL string, release *Release) error
```

- Constructs asset name: `sap-devs_v<version>_<GOOS>_<GOARCH>.tar.gz` (`.zip` on Windows)
- Downloads asset and `checksums.txt` from the GitHub Releases download URL
- Verifies SHA256 of the downloaded archive against the matching line in `checksums.txt`
- On mismatch: deletes temp file, returns error `"checksum mismatch — download may be corrupt"`
- Extracts the binary from the archive into a temp file alongside the current executable
- Calls `os.Rename` (atomic on the same filesystem) to replace the current binary
- Uses `os.Executable()` to locate the current binary path; errors clearly if it cannot be resolved

#### `check_cache.go`

```go
// ShouldCheck returns true if enough time has passed since the last update check.
// Returns true if the cache file is missing or unreadable (fail-open).
func ShouldCheck(cacheDir string, ttl time.Duration) bool

// RecordCheck writes the current time to the cache file.
func RecordCheck(cacheDir string) error
```

- Cache file: `<cacheDir>/update_check.json` containing `{"last_check": "<RFC3339 timestamp>"}`
- Missing or corrupt file → returns `true` (fail-open, so first run always checks)
- `RecordCheck` is called regardless of whether an update was found

### `cmd/update.go`

```
sap-devs update
```

Manual flow:
1. `CheckLatest(repoURL, Version)` — print `Checking for updates...`
2. Already up to date → `sap-devs v1.2.0 is already up to date.`
3. Newer found → `Updating sap-devs v1.0.0 → v1.2.0...` then `Install`
4. Success → `✓ Updated to v1.2.0. Restart your shell if needed.`
5. Error → surface directly (network failure, checksum mismatch, permission denied)

`repoURL` constant defined once in `cmd/update.go`:
```go
const repoURL = "https://github.tools.sap/developer-relations/sap-devs-cli"
```

### Background check (wired into `cmd/root.go`)

Added to root command's `PersistentPreRunE` / `PersistentPostRunE`:

```
PersistentPreRunE:
  paths, _ := xdg.New()
  if update.ShouldCheck(paths.CacheDir, 168*time.Hour) {
      go func() {
          release, newer, _ := update.CheckLatest(repoURL, Version)
          if newer { updateHint = "↻ sap-devs " + release.TagName + " available — run 'sap-devs update' to install" }
          update.RecordCheck(paths.CacheDir)
      }()
  }

PersistentPostRunE:
  if updateHint != "" { fmt.Fprintln(os.Stderr, updateHint) }
```

`updateHint` is a package-level `string` in `cmd/`. The goroutine has a short timeout (3 seconds) — if it hasn't resolved by the time `PersistentPostRunE` fires, the hint is skipped silently.

The background check is skipped for the `update` command itself (avoid recursive hint).

## Data Flow

```
git tag v1.2.0 && git push --tags
  → GitHub Actions release.yml
    → goreleaser builds 5 binaries + checksums.txt
      → GitHub Releases: sap-devs_v1.2.0_linux_amd64.tar.gz, ..., checksums.txt

sap-devs update
  → CheckLatest → GET api.github.com/.../releases/latest → { tag_name: "v1.2.0" }
  → newer than current? yes
  → Install → download asset + checksums.txt
            → verify SHA256
            → extract binary to temp file
            → os.Rename → replace running binary
  → print success
```

## Error Handling

| Situation | Behavior |
|-----------|----------|
| Network failure (manual `update`) | Surface error: `error: could not reach GitHub: <details>` |
| Network failure (background check) | Silently swallowed |
| Checksum mismatch | Delete temp file, error: `error: checksum mismatch — download may be corrupt` |
| `os.Executable()` fails | Error: `error: could not determine binary path` |
| Permission denied replacing binary | Surface OS error |
| Already up to date | `sap-devs vX.Y.Z is already up to date.` |
| Running `dev` build (no semver tag) | Skip background check; `update` prints `cannot update a dev build` |

## Testing

### `internal/update/checker_test.go`

- `TestCheckLatest_NewerAvailable` — httptest server returns `{"tag_name":"v9.9.9"}` → returns release, newer=true
- `TestCheckLatest_AlreadyLatest` — server returns current version tag → newer=false
- `TestCheckLatest_NetworkError` — server URL unreachable → returns nil, false, nil (not an error)
- `TestCheckLatest_MalformedJSON` — server returns 200 with bad body → nil, false, nil

### `internal/update/installer_test.go`

- `TestInstall_Success` — httptest serves valid tar.gz + matching checksums.txt → binary replaced
- `TestInstall_ChecksumMismatch` — checksums.txt has wrong hash → error contains "checksum mismatch", temp file deleted
- `TestInstall_WrongPlatformAsset` — verifies asset name constructed correctly for current GOOS/GOARCH

### `internal/update/check_cache_test.go`

- `TestShouldCheck_MissingFile` → true
- `TestShouldCheck_RecentCheck` → false (timestamp within TTL)
- `TestShouldCheck_ExpiredCheck` → true (timestamp beyond TTL)
- `TestShouldCheck_CorruptFile` → true (fail-open)
- `TestRecordCheck_WritesTimestamp` → file created with valid RFC3339 timestamp

## Files

| File | Action | Purpose |
|------|--------|---------|
| `.goreleaser.yml` | Create | Cross-platform release build config |
| `.github/workflows/release.yml` | Create | Tag-triggered release workflow |
| `internal/update/checker.go` | Create | `CheckLatest` — GitHub Releases API |
| `internal/update/checker_test.go` | Create | Unit tests for checker |
| `internal/update/installer.go` | Create | `Install` — download, verify, replace |
| `internal/update/installer_test.go` | Create | Unit tests for installer |
| `internal/update/check_cache.go` | Create | `ShouldCheck` / `RecordCheck` |
| `internal/update/check_cache_test.go` | Create | Unit tests for cache |
| `cmd/update.go` | Create | `update` Cobra command |
| `cmd/root.go` | Modify | Wire background check into persistent pre/post run |
