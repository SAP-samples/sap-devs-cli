# Security Review

Last reviewed: 2026-04-20

## Scope

Full codebase review covering:

- Embedded HTTP server with session token auth (tray binary)
- OS-native scheduler registration (Windows/Linux/macOS)
- ZIP archive extraction from GitHub
- Credential handling (keychain + file fallback)
- Content loading from multiple directory layers
- Static file serving via embedded filesystem
- External API interactions (Discovery Center, GitHub, YouTube RSS, ip-api.com)
- Autostart registration (LaunchAgent plist, .desktop files, Windows Registry)
- Subprocess execution across the CLI

## Methodology

Three-phase analysis:

1. Broad vulnerability scan across all security-critical areas
2. Per-finding deep-dive validation by independent reviewers
3. False-positive filtering against confidence threshold (>=80%)

Categories examined: input validation, authentication/authorization, crypto/secrets management, injection/code execution, data exposure, path traversal, and deserialization.

## Findings

No vulnerabilities met the high-confidence reporting threshold.

## Areas Investigated

### ZIP Extraction (`internal/sync/fetcher.go`)

**Check:** Zip slip attack via `../` entries in downloaded archives.

**Result:** Protected. The extraction code normalizes the destination with `filepath.Abs`, cleans entry paths with `filepath.Join` + `filepath.FromSlash`, and validates with a `strings.HasPrefix` check including the path separator. A dedicated test (`TestFetcher_BlocksZipSlip`) verifies the guard.

### Tray Binary Extraction (`internal/trayctl/extract.go`)

**Check:** Path traversal during tar.gz/zip extraction of the tray companion binary.

**Result:** Safe by design. Archive contents are extracted into memory only (via `io.ReadAll`), then written to a single controlled path returned by `Manager.BinaryPath()`. No intermediate filesystem operations are exposed.

### HTTP Static File Server (`cmd/sap-devs-tray/server.go`)

**Check:** Directory traversal via requests like `../../etc/passwd`.

**Result:** Safe. Static assets are served from a Go `embed.FS`, which is fundamentally bounded to the files embedded at compile time. There is no access to the real filesystem.

### Session Token Auth (`cmd/sap-devs-tray/server.go`)

**Check:** Token leakage via URL query parameters, Referrer headers, or browser history.

**Result:** Acceptable for the threat model. The token is passed in query parameters within a Wails v3 embedded webview (not a regular browser). Mitigating factors:

- Wails webview cannot navigate to external URLs — no Referrer leakage
- All resources are embedded — no CDN or external font/image loads
- Server binds to `127.0.0.1` only — no network exposure
- Token is 128-bit, generated via `crypto/rand` — cryptographically secure
- Token is per-process and regenerated on each app restart

### OS Scheduler Registration (`internal/service/scheduler_*.go`)

**Check:** Shell command injection via the binary path embedded in scheduled task definitions.

**Result:** Not exploitable. The binary path originates exclusively from `os.Executable()` (resolved through `filepath.EvalSymlinks`), which is system state — not user input. Exploitation would require an attacker to control where the binary is installed, which already grants direct code execution without needing shell injection. Platform-specific notes:

- **Windows:** Double-quoted paths in `cmd /c` cannot be broken by valid Windows filenames (which prohibit `"`)
- **Linux:** Path in double quotes inside single quotes in the systemd unit; breaking out requires a `'` in the path, but an attacker with that level of filesystem control can replace the binary directly
- **macOS:** Similar to Linux; plist passes the command string to `/bin/sh -c`

### Autostart Registration (`internal/trayctl/autostart.go`)

**Check:** XML injection (macOS plist) and desktop entry injection (Linux `.desktop` file) via unescaped binary path.

**Result:** Not exploitable. Binary path comes from `os.Executable()`. Additionally:

- macOS: launchd's XML parser treats `<string>` values as literal data, not evaluated for entities
- Linux: The `Exec=` field in desktop entries is parsed by the desktop environment via direct exec, not shell-evaluated per the freedesktop specification

### Content Loader (`internal/content/loader.go`)

**Check:** Path traversal via pack IDs or content paths, including the project layer (`.sap-devs/`).

**Result:** Safe. `os.ReadDir` returns only immediate child entries (names cannot contain path separators on any filesystem). The project directory is hardcoded to `.sap-devs/` joined with the current working directory — no user-controllable path components.

### Credentials (`internal/credentials/credentials.go`)

**Check:** Token exposure in logs, error messages, or insecure file storage.

**Result:** Safe across all layers of the credential lifecycle.

**Storage:** Primary storage uses the OS keychain via `zalando/go-keyring` (macOS Keychain, Windows Credential Manager, Linux Secret Service). When the keychain is unavailable, the fallback writes to `<configDir>/credentials` with `0600` permissions (owner read/write only).

**No token leakage in logs.** The three `fmt.Fprintf(os.Stderr, ...)` calls in the credentials package only log the keychain error message (e.g., "keychain unavailable: ..."), never the token value itself.

**Token masked in CLI output.** The `config show` command uses a `maskedToken()` helper (`cmd/config.go:71-83`) that displays "set" or "not set" — the actual token value is never printed.

**Shell history warning.** When the user passes a token as a CLI argument (`sap-devs config token <value>`), the code explicitly warns about shell history exposure and suggests using the interactive prompt instead (`cmd/config.go:191-192`).

**Resolution chain is clean.** `Resolve()` checks environment variables (`GH_TOKEN`, `GITHUB_TOKEN`, `GITHUB_TOOLS_SAP_TOKEN`) then keychain/file — the resolved token is never logged or included in error messages at any point in the chain.

**Credential file naming.** The `credFileForUser` function builds the fallback filename from the `service` parameter (e.g., `credentials-youtube`). This value comes from the `--service` CLI flag, which is trusted input. `filepath.Join` normalizes the path, preventing traversal.

### Environment Variable Execution (`TERMINAL`, `PAGER`)

**Check:** Command injection via `$TERMINAL` or `$PAGER` environment variables.

**Result:** Excluded from scope. Environment variables are trusted values in this security model — an attacker who can modify them already has equivalent access to the user's session.

## Positive Security Observations

| Area | Implementation |
|------|---------------|
| Zip slip protection | Explicit `HasPrefix` check with path separator, backed by unit test |
| Session tokens | 128-bit via `crypto/rand.Read` + `hex.EncodeToString` |
| HTTP server binding | Hardcoded to `127.0.0.1:0` (loopback, random port) |
| Static asset serving | Go `embed.FS` eliminates directory traversal by design |
| Binary path resolution | `filepath.EvalSymlinks` before use in scheduler/autostart |
| Subprocess arguments | `exec.Command` with separate argument strings (not shell-concatenated) |
| Archive size limits | `io.LimitReader` applied during tray binary extraction |
| Credential file permissions | `0600` on fallback credential file |
