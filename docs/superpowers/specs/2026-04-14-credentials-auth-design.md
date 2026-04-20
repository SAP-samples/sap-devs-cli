# Credentials & Authentication Design

**Date:** 2026-04-14  
**Status:** Approved  
**Feature:** Secure token storage and authenticated sync for github.com/SAP-samples

---

## Problem

`sap-devs sync` fetches content from `github.com/SAP-samples`, which requires authentication. Unauthenticated requests receive a 302 redirect to the login page. `http.Get` follows the redirect, receives an HTML page with HTTP 200, and `zip.NewReader` then fails with "not a valid zip file" — a confusing error that does not indicate the root cause.

No authentication mechanism currently exists in the CLI.

---

## Goals

- Allow users to authenticate with github.com/SAP-samples for content sync
- Store tokens securely (OS keychain preferred, credentials file fallback)
- Support CI/scripted usage via environment variables
- Surface a clear, actionable error when authentication is missing or fails
- Never expose tokens in URLs, error messages, logs, or `config show` output
- Update end-user and developer documentation

---

## Non-Goals

- OAuth flows or browser-based login
- Per-command token scoping
- Encrypting the credentials file fallback (0600 permissions are sufficient)

---

## Architecture

### Token Resolution Order

When a token is needed (e.g., before `FetchArchive`), the CLI resolves it in this priority order:

1. `GITHUB_TOOLS_SAP_TOKEN` environment variable
2. `GH_TOKEN` environment variable
3. `GITHUB_TOKEN` environment variable
4. OS keychain entry (service: `"sap-devs"`, username: `"github-token"`)
5. Credentials file: `~/.config/sap-devs/credentials` (0600)
6. Empty string (unauthenticated)

`Resolve()` never returns an error; a missing token resolves to empty string.

---

## Components

### 1. `internal/credentials` package

New package with four exported functions:

```go
// Store saves the token to the OS keychain, falling back to the credentials
// file if the keychain is unavailable (e.g., headless Linux, CI containers).
func Store(configDir, token string) error

// Load retrieves the token from the OS keychain or credentials file.
// Returns ErrNotFound if no token is stored.
func Load(configDir string) (string, error)

// Delete removes the stored token from the keychain or credentials file.
// Returns ErrNotFound if no token was stored (idempotent, not a hard error).
func Delete(configDir string) error

// Resolve returns the best available token using the full priority chain:
// env vars → keychain → credentials file → "".
// Never returns an error; missing credentials resolve to empty string.
func Resolve(configDir string) string

// ErrNotFound is returned by Delete (and Load) when no token exists.
var ErrNotFound = errors.New("credentials: no token stored")
```

The `config token --delete` command checks the return value: if `err == nil` print `"Token removed."`; if `errors.Is(err, credentials.ErrNotFound)` print `"No token was stored."`; any other error is a hard failure.

**Keychain backend:** `zalando/go-keyring`

- macOS: Keychain
- Windows: Windows Credential Manager
- Linux: Secret Service via D-Bus; falls back to credentials file when unavailable

**Credentials file fallback:**

- Path: `<xdg.ConfigDir>/credentials`
  - Linux: `~/.config/sap-devs/credentials`
  - macOS: `~/Library/Application Support/sap-devs/credentials`
  - Windows: `%APPDATA%/sap-devs/credentials`
- Format: plain text, token only, single line
- Permissions: 0600 (owner read/write only), set on create
- Used only when keychain is unavailable; print to stderr: `"keychain unavailable: <reason>; token stored in credentials file"` (informational, not an error)

**Keychain error handling:** If `Load()` returns a non-nil error that is not "not found" (e.g., the user denied keychain access on macOS), `Resolve()` prints a stderr warning — `keychain unavailable: <reason>; falling back to credentials file` — then attempts the credentials file. Token exposure risk is contained: the warning never includes the token value.

**Security properties:**

- Token is never interpolated into error strings
- Token is never appended to URLs
- The credentials file is separate from `config.yaml` to avoid accidental inclusion in dotfile repos
- `config show` masks the token: if shorter than 4 characters displays `(set)`; otherwise displays `<first4chars>****`; if not stored displays `(not set)`

---

### 2. `FetchArchive` signature change

**Before:** `FetchArchive(url, destDir string) error`  
**After:** `FetchArchive(rawURL, destDir, token string) error`

Rename the `url` parameter to `rawURL` to avoid shadowing the `net/url` package. Parse it at the start of the function: `parsedURL, err := url.Parse(rawURL)` — `parsedURL` is used for the redirect check below and for extracting the host for the error message.

When `token` is non-empty, the HTTP request includes:

```http
Authorization: token <tok>
```

**Auth redirect detection:** After reading the response body and before calling `zip.NewReader`, check whether the final redirected-to URL matches the host-specific login path. In Go, the correct field is `resp.Request.URL` (the URL of the last request in the redirect chain). Check using `resp.Request.URL.Host == parsedURL.Host && strings.Contains(resp.Request.URL.Path, "/login")` — scoped to the same host as the original request to avoid false positives from content paths that happen to contain `/login`. If matched, return:

```text
authentication required for <host> — set GITHUB_TOOLS_SAP_TOKEN or run 'sap-devs config token'
```

where `<host>` is `parsedURL.Host` (the **original** URL's host, not the redirect target), to avoid leaking internal redirect hostnames.

The login page HTML body is small and downloading it before the check fires is acceptable — no custom redirect policy is needed.

This replaces the current "not a valid zip file" error with an actionable message.

Resolve the token once at the top of `syncCmd.RunE` (before the official fetch) and pass the same value to both `FetchArchive` calls — the official repo fetch and the company repo fetch.

---

### 3. `sap-devs config token` command

New subcommand under `config`:

```text
sap-devs config token [value] [--delete]
```

**Interactive mode (no argument):**

```text
Enter GitHub token (input hidden, will not appear in shell history):
Token stored securely.
```

Input is read with echo disabled via `golang.org/x/term` `ReadPassword(int(os.Stdin.Fd()))`. This works correctly in interactive terminals on macOS, Windows (cmd.exe, PowerShell, Windows Terminal), and Linux. When stdin is not a TTY (e.g., piped input, Git Bash on Windows), `ReadPassword` will return an error — in that case the command should print a clear message ("interactive input not available — pass token as argument: `sap-devs config token <value>`") and exit with a non-zero code rather than blocking silently.

**Argument mode:**

```text
Warning: token passed as argument may be saved in shell history.
Consider using 'sap-devs config token' without arguments for interactive entry.
Token stored securely.
```

**Delete mode (`--delete`):**

```text
sap-devs config token --delete
```

Removes the stored token from the keychain or credentials file. `Delete()` is idempotent — returns `nil` when no token exists. The command prints `"Token removed."` on success or `"No token was stored."` if nothing was found. Supplying both a positional value and `--delete` is an error: the command should return `"cannot use --delete with a token value"` and exit non-zero.

**`config show` integration:** `configShowCmd` must call `credentials.Load(paths.ConfigDir)` and append a `github_token` line to its output. Token masking rule: if the token is shorter than 4 characters, display `(set)`; otherwise display `<first4chars>****`. If `Load()` returns `ErrNotFound`, display `(not set)`. If `Load()` returns any other error (e.g., keychain access denied), display `(unavailable)` — do not propagate the error or abort `config show`.

**Explanatory text** shown in `--help` and during `init`:

> Only required when syncing content from a private GitHub Enterprise instance (github.com/SAP-samples). Not needed if you are outside the SAP network or already have GITHUB_TOOLS_SAP_TOKEN set in your environment.

---

### 4. `init` wizard update

The token prompt becomes **Step 1 of 5**, before the sync step. The full updated wizard flow:

1. **GitHub authentication** (new — token prompt, skippable)
2. **Downloading SAP developer content** (was Step 1 — sync)
3. **Setting your developer profile** (was Step 2)
4. **Injecting context into AI tools** (was Step 3)
5. **Done / next steps** (was Step 4)

```text
Step 1/5: GitHub authentication (optional)

  sap-devs syncs content from github.com/SAP-samples, which requires a Personal
  Access Token if you are inside the SAP corporate network. If you are
  outside SAP or already have GITHUB_TOOLS_SAP_TOKEN set in your
  environment, press Enter to skip.

  GitHub token (press Enter to skip):
```

The token prompt in Step 1 uses `golang.org/x/term` `ReadPassword(int(os.Stdin.Fd()))` for hidden input, consistent with `config token` interactive mode. When stdin is not a TTY (e.g., piped CI input), `ReadPassword` will return an error — in that case the wizard should skip Step 1 gracefully, print `"Note: interactive token input unavailable. Run 'sap-devs config token <value>' after setup to authenticate."`, and continue to Step 2.

Behaviour:

- If a token is entered: call `credentials.Store(paths.ConfigDir, token)` first, then call `runSyncForce()`. The sync command internally calls `credentials.Resolve()`, which will pick up the freshly stored token. **`Store()` must complete before `runSyncForce()` is called** — do not pass the token directly to sync.
- If Enter is pressed: sync proceeds using whatever `credentials.Resolve()` finds (env var or nothing)
- If sync fails with an auth error despite a token: prints the actionable error message and continues with any cached content (same graceful-degradation behaviour as today)

---

## Documentation Updates

### `docs/user/user-guide.md`

Add a new **Authentication** section near the sync documentation covering:

- When a token is needed (private github.com/SAP-samples, SAP corporate network)
- The three env vars checked, in priority order
- How to store a token interactively: `sap-devs config token`
- How to store a token non-interactively (scripted/CI): pass as argument or set env var
- Where tokens are stored (OS keychain or 0600 credentials file — never in `config.yaml`)
- How to remove a stored token: `sap-devs config token --delete`
- CI usage: set `GITHUB_TOOLS_SAP_TOKEN` as a pipeline secret; no local storage needed

### `docs/developer/developer-guide.md`

Update the **Sync** section and add a **Credentials** section covering:

- The `internal/credentials` package: purpose, the four functions, and the keychain → file fallback
- `FetchArchive` signature change and auth redirect detection logic
- The `zalando/go-keyring` dependency: why it was chosen and what backends it uses
- Security properties: token only in Authorization header, never in URLs or error strings, masked in `config show`
- Testing notes: keychain calls should be abstracted behind an interface for unit tests

### `CLAUDE.md`

Add `internal/credentials/` to the architecture overview table with description:

> Secure token storage — OS keychain (zalando/go-keyring) with credentials file fallback; `Resolve()` implements env var → keychain → file priority chain.

---

## Dependencies

| Package | Purpose |
| --- | --- |
| `zalando/go-keyring` | OS keychain abstraction (macOS Keychain, Windows Credential Manager, Linux Secret Service) |
| `golang.org/x/term` | Read password input without echo. Already present as an indirect dependency; importing it promotes it to direct after `go mod tidy`. |

---

## Testing

- Unit tests for `credentials.Resolve()` covering all priority levels (env var wins, keychain wins, file fallback, empty)
- Unit test for `Resolve()` when `Load()` returns a non-"not found" error (e.g., keychain access denied): verifies stderr warning is printed, warning does not contain the token value, and fallback to credentials file proceeds
- Unit tests for `FetchArchive` auth redirect detection using an `httptest` server that redirects to `<host>/login`
- `Store`/`Load`/`Delete` round-trip tested against the file fallback path (keychain skipped in CI via interface injection or build tag); `Delete` test verifies a subsequent `Resolve()` returns empty string
- `init` and `config token` command tests verify token is not echoed and masked in output
- `config token --delete` test verifies stored token is removed and `config show` subsequently displays `(not set)`
- `config token --delete value` test verifies mutually exclusive flag returns a non-zero exit code
