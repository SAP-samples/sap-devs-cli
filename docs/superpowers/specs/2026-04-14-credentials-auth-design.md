# Credentials & Authentication Design

**Date:** 2026-04-14  
**Status:** Approved  
**Feature:** Secure token storage and authenticated sync for github.tools.sap

---

## Problem

`sap-devs sync` fetches content from `github.tools.sap`, which requires authentication. Unauthenticated requests receive a 302 redirect to the login page. `http.Get` follows the redirect, receives an HTML page with HTTP 200, and `zip.NewReader` then fails with "not a valid zip file" — a confusing error that does not indicate the root cause.

No authentication mechanism currently exists in the CLI.

---

## Goals

- Allow users to authenticate with github.tools.sap for content sync
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
// Returns ("", nil) if no token is stored.
func Load(configDir string) (string, error)

// Delete removes the stored token from the keychain or credentials file.
func Delete(configDir string) error

// Resolve returns the best available token using the full priority chain:
// env vars → keychain → credentials file → "".
// Never returns an error; missing credentials resolve to empty string.
func Resolve(configDir string) string
```

**Keychain backend:** `zalando/go-keyring`
- macOS: Keychain
- Windows: Windows Credential Manager
- Linux: Secret Service via D-Bus; falls back to credentials file when unavailable

**Credentials file fallback:**
- Path: `<xdg.ConfigDir>/credentials` (e.g., `~/.config/sap-devs/credentials` on Linux)
- Format: plain text, token only, single line
- Permissions: 0600 (owner read/write only), set on create
- Used only when keychain is unavailable; a log-level note is printed (not a warning) indicating file storage is in use

**Security properties:**
- Token is never interpolated into error strings
- Token is never appended to URLs
- The credentials file is separate from `config.yaml` to avoid accidental inclusion in dotfile repos
- `config show` masks the token: displays first 4 characters followed by `****`, or `(not set)`

---

### 2. `FetchArchive` signature change

**Before:** `FetchArchive(url, destDir string) error`  
**After:** `FetchArchive(url, destDir, token string) error`

When `token` is non-empty, the HTTP request includes:
```
Authorization: token <tok>
```

**Auth redirect detection:** After reading the response body and before calling `zip.NewReader`, check whether the final response URL contains `/login`. If it does, return:
```
authentication required for <host> — set GITHUB_TOOLS_SAP_TOKEN or run 'sap-devs config token'
```

This replaces the current "not a valid zip file" error with an actionable message.

All callers of `FetchArchive` (in `cmd/sync.go`) resolve the token via `credentials.Resolve()` before calling.

---

### 3. `sap-devs config token` command

New subcommand under `config`:

```
sap-devs config token [value] [--delete]
```

**Interactive mode (no argument):**
```
Enter GitHub token (input hidden, will not appear in shell history):
Token stored securely.
```
Input is read with echo disabled (e.g., `golang.org/x/term` `ReadPassword`).

**Argument mode:**
```
Warning: token passed as argument may be saved in shell history.
Consider using 'sap-devs config token' without arguments for interactive entry.
Token stored securely.
```

**Delete mode (`--delete`):**
```
sap-devs config token --delete
```
Removes the stored token from the keychain or credentials file.

**Explanatory text** shown in `--help` and during `init`:
> Only required when syncing content from a private GitHub Enterprise instance (github.tools.sap). Not needed if you are outside the SAP network or already have GITHUB_TOOLS_SAP_TOKEN set in your environment.

---

### 4. `init` wizard update

The token prompt becomes **Step 1 of 5**, before the sync step. All subsequent steps shift up by one.

```
Step 1/5: GitHub authentication (optional)

  sap-devs syncs content from github.tools.sap, which requires a Personal
  Access Token if you are inside the SAP corporate network. If you are
  outside SAP or already have GITHUB_TOOLS_SAP_TOKEN set in your
  environment, press Enter to skip.

  GitHub token (press Enter to skip):
```

Behaviour:
- If a token is entered: stored via `credentials.Store()`, then sync proceeds with it
- If Enter is pressed: sync proceeds using whatever `credentials.Resolve()` finds (env var or nothing)
- If sync fails with an auth error despite a token: prints the actionable error message and continues with any cached content (same graceful-degradation behaviour as today)

---

## Documentation Updates

### `docs/user/user-guide.md`

Add a new **Authentication** section near the sync documentation covering:
- When a token is needed (private github.tools.sap, SAP corporate network)
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
|---|---|
| `zalando/go-keyring` | OS keychain abstraction (macOS Keychain, Windows Credential Manager, Linux Secret Service) |
| `golang.org/x/term` | Read password input without echo (already a transitive dep of many Go tools) |

---

## Testing

- Unit tests for `credentials.Resolve()` covering all priority levels (env var wins, keychain wins, file fallback, empty)
- Unit tests for `FetchArchive` auth redirect detection using an `httptest` server that returns a redirect to `/login`
- `Store`/`Load`/`Delete` tested against the file fallback path (keychain skipped in CI via interface injection or build tag)
- `init` and `config token` command tests verify token is not echoed and masked in output
