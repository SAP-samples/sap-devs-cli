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
- Asset naming: goreleaser default strips the leading `v` from the version in archive names. The name template produces: `sap-devs_<version>_<OS>_<arch>.tar.gz` where `<version>` has no `v` prefix (e.g. `sap-devs_1.2.0_linux_amd64.tar.gz`). Windows uses `.zip`.
- Archives contain the binary only (no README or LICENSE in archive)
- Changelog generated from git commits
- `CGO_ENABLED=0` set in the goreleaser build config to enable cross-compilation from the Linux CI runner

### GitHub Actions release workflow (`.github/workflows/release.yml`)

- Trigger: `push` to tags matching `v*`
- Runner: `ubuntu-latest`
- Steps: checkout with full history (for changelog) → setup Go → run goreleaser
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
// whether it is newer than currentVersion.
// Returns a real error on failure — callers decide whether to surface or swallow it.
func CheckLatest(repoURL, currentVersion string) (*Release, bool, error)
```

- Calls `https://api.github.com/repos/<owner>/<repo>/releases/latest` with header `Accept: application/vnd.github+json`
- `repoURL` is the full repo URL (e.g. `https://github.tools.sap/developer-relations/sap-devs-cli`) — owner/repo are parsed from it by splitting the path
- Version comparison: normalize both versions to `major.minor.patch` by trimming a leading `v`, split on `.`, compare as integers field by field. No external semver library — avoids adding a new dependency.
- Returns an error on network failure or unparseable response

#### `installer.go`

```go
// Install downloads the release asset for the current OS/arch, verifies its
// SHA256 checksum against checksums.txt, and replaces the running binary.
func Install(repoURL string, release *Release) error
```

- Constructs asset name: `sap-devs_<version>_<GOOS>_<GOARCH>.tar.gz` where `<version>` has no `v` prefix — matching the goreleaser output format exactly (e.g. `sap-devs_1.2.0_linux_amd64.tar.gz`). Windows uses `.zip`.
- Downloads asset and `checksums.txt` from the GitHub Releases download URL
- Verifies SHA256 of the downloaded archive against the matching line in `checksums.txt`
- On mismatch: deletes temp file, returns error `"checksum mismatch — download may be corrupt"`
- On unsupported platform (asset name not found in `checksums.txt`): returns error `"no release asset found for <GOOS>/<GOARCH>"`
- Extracts the binary from the archive into a temp file in the same directory as the current executable
- Uses `os.Executable()` to locate the current binary path; returns `"could not determine binary path"` error if it fails
- **Platform-specific binary replacement:**
  - Linux/macOS: `os.Rename(tmpFile, currentPath)` — atomic on same filesystem
  - Windows: `os.Remove(currentPath)` then `os.Rename(tmpFile, currentPath)` — non-atomic but necessary because Windows locks running executables; if `os.Remove` succeeds but `os.Rename` fails, the temp file is left in place alongside a missing original (documented limitation)

#### `check_cache.go`

```go
// ShouldCheck returns true if enough time has passed since the last update check.
// Returns true if the cache file is missing or unreadable (fail-open).
func ShouldCheck(cacheDir string, ttl time.Duration) bool

// RecordCheck writes the current time to the cache file.
// Only called after a successful response from CheckLatest (not on network errors),
// so a network failure does not suppress retries for the full TTL period.
func RecordCheck(cacheDir string) error
```

- Cache file: `<cacheDir>/update_check.json` containing `{"last_check": "<RFC3339 timestamp>"}`
- Missing or corrupt file → `ShouldCheck` returns `true` (fail-open, so first run always checks)

### `cmd/update.go`

Manual flow:

1. If `Version == "dev"`: print `cannot update a dev build` and return
2. Print `Checking for updates...`
3. Call `CheckLatest(repoURL, Version)` — surface any error directly
4. Already up to date → print `sap-devs v1.2.0 is already up to date.` and return
5. Newer found → print `Updating sap-devs v1.0.0 → v1.2.0...` then call `Install`
6. Success → print `✓ Updated to v1.2.0. Restart your shell if needed.`
7. Any error → surface directly (network failure, checksum mismatch, permission denied)

`repoURL` is a package-level constant defined in `cmd/update.go`, accessible to all files in `package cmd`:

```go
const repoURL = "https://github.tools.sap/developer-relations/sap-devs-cli"
```

`Version` already exists as a `var` in `cmd/version.go` (`var Version = "dev"`). Do **not** redeclare it in `cmd/update.go` — it is set by `-ldflags` at build time and must remain a `var` for that injection to work.

### Background check (wired into `cmd/root.go`)

A buffered channel synchronizes the goroutine result with `PersistentPostRunE`. The channel approach avoids a data race and provides the 3-second timeout.

`updateHintCh` is a package-level `chan string` in `cmd/`. It must be reset to `nil` at the top of each `PersistentPreRunE` invocation (before the `ShouldCheck` guard) so that in-process test runs that invoke multiple commands do not observe a stale channel from a previous invocation.

```go
// PersistentPreRunE (skip entirely for the "update" command itself):
updateHintCh = nil  // reset before every invocation
if Version != "dev" && update.ShouldCheck(paths.CacheDir, 168*time.Hour) {
    updateHintCh = make(chan string, 1)
    go func() {
        release, newer, err := update.CheckLatest(repoURL, Version)
        if err == nil {
            update.RecordCheck(paths.CacheDir)
            if newer {
                updateHintCh <- "↻ sap-devs " + release.TagName + " available — run 'sap-devs update' to install"
            }
        }
        // on error: channel stays empty, hint is skipped, RecordCheck not called
    }()
}

// PersistentPostRunE:
if updateHintCh != nil {
    select {
    case hint := <-updateHintCh:
        fmt.Fprintln(os.Stderr, hint)
    case <-time.After(3 * time.Second):
        // goroutine too slow or found no update — skip hint silently
    }
}
```

The background check is skipped for the `update` command itself (checked by command name in `PersistentPreRunE`). It is also skipped for `dev` builds (`Version == "dev"`).

## Data Flow

```text
git tag v1.2.0 && git push --tags
  → GitHub Actions release.yml
    → goreleaser builds 5 binaries + checksums.txt
      → GitHub Releases: sap-devs_1.2.0_linux_amd64.tar.gz, ..., checksums.txt

sap-devs update
  → CheckLatest → GET api.github.com/.../releases/latest → { tag_name: "v1.2.0" }
  → newer than current? yes
  → Install → download sap-devs_1.2.0_<os>_<arch>.tar.gz + checksums.txt
            → verify SHA256
            → extract binary to temp file
            → replace running binary (os.Rename; os.Remove+os.Rename on Windows)
  → print success
```

## Error Handling

| Situation | Behavior |
| --- | --- |
| Network failure (manual `update`) | Surface error: `error: could not reach GitHub: <details>` |
| Network failure (background check) | Silently swallowed; `RecordCheck` not called (will retry next run) |
| Checksum mismatch | Delete temp file, error: `error: checksum mismatch — download may be corrupt` |
| Unsupported platform | Error: `no release asset found for <GOOS>/<GOARCH>` |
| `os.Executable()` fails | Error: `could not determine binary path` |
| Permission denied replacing binary | Surface OS error |
| Already up to date | `sap-devs vX.Y.Z is already up to date.` |
| Running `dev` build | Background check skipped; `update` prints `cannot update a dev build` |

## Testing

### `internal/update/checker_test.go`

- `TestCheckLatest_NewerAvailable` — httptest server returns `{"tag_name":"v9.9.9"}` → returns release, newer=true
- `TestCheckLatest_AlreadyLatest` — server returns current version tag → newer=false
- `TestCheckLatest_NetworkError` — server URL unreachable → returns error (not nil)
- `TestCheckLatest_MalformedJSON` — server returns 200 with bad body → returns error

### `internal/update/installer_test.go`

- `TestInstall_Success` — httptest serves valid tar.gz + matching checksums.txt → binary replaced
- `TestInstall_ChecksumMismatch` — checksums.txt has wrong hash → error contains "checksum mismatch", temp file deleted
- `TestInstall_UnsupportedPlatform` — asset name absent from checksums.txt → error contains "no release asset found"

### `internal/update/check_cache_test.go`

- `TestShouldCheck_MissingFile` → true
- `TestShouldCheck_RecentCheck` → false (timestamp within TTL)
- `TestShouldCheck_ExpiredCheck` → true (timestamp beyond TTL)
- `TestShouldCheck_CorruptFile` → true (fail-open)
- `TestRecordCheck_WritesTimestamp` → file created with valid RFC3339 timestamp

## Files

| File | Action | Purpose |
| --- | --- | --- |
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
