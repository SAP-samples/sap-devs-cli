# Credentials & Authentication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the silent "not a valid zip file" error during `sap-devs sync` by adding OS-keychain-backed token storage and authenticated HTTP requests to github.com/SAP-samples.

**Architecture:** A new `internal/credentials` package wraps `zalando/go-keyring` with a file fallback (`<configDir>/credentials`, 0600). `FetchArchive` gains a `token` parameter and auth-redirect detection. A new `sap-devs config token` command manages the stored token, and `init` prompts for a token before syncing.

**Tech Stack:** `zalando/go-keyring` (OS keychain), `golang.org/x/term` (hidden input), `net/http` + `httptest` for redirect detection tests.

**Spec:** `docs/superpowers/specs/2026-04-14-credentials-auth-design.md`

---

## File Map

| Action | Path | Responsibility |
| --- | --- | --- |
| Create | `internal/credentials/credentials.go` | `Store`, `Load`, `Delete`, `Resolve`, `ErrNotFound`, keyring interface |
| Create | `internal/credentials/credentials_test.go` | Unit tests (internal package — accesses unexported `keyringBackend`) |
| Modify | `internal/sync/fetcher.go` | Rename `url` → `rawURL`, add `token` param, auth header, redirect detection |
| Modify | `internal/sync/fetcher_test.go` | Update call sites to pass `""` token; add redirect detection tests |
| Modify | `cmd/sync.go` | `credentials.Resolve()` at top of `RunE`, pass token to both `FetchArchive` calls |
| Modify | `cmd/config.go` | Add `configTokenCmd`, update `configShowCmd` to show masked token |
| Modify | `cmd/init.go` | Move `xdg.New()` to top, add Step 1 (auth prompt), renumber steps 1→5 |
| Modify | `docs/user/user-guide.md` | Add Authentication section |
| Modify | `docs/developer/developer-guide.md` | Add Credentials section, update Sync section |
| Modify | `CLAUDE.md` | Add `internal/credentials/` to architecture table |

---

## Task 1: Add `zalando/go-keyring` dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the dependency**

```bash
cd d:/projects/sap-devs-cli
go get github.com/zalando/go-keyring
go mod tidy
```

- [ ] **Step 2: Verify the build still compiles**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add zalando/go-keyring dependency"
```

---

## Task 2: Create `internal/credentials` package

**Files:**
- Create: `internal/credentials/credentials.go`
- Create: `internal/credentials/credentials_test.go`

The tests use `package credentials` (not `package credentials_test`) so they can replace the unexported `keyringBackend` variable with a fake.

- [ ] **Step 1: Write the failing tests**

Create `internal/credentials/credentials_test.go`:

```go
package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeKeyring simulates a working in-memory keychain.
type fakeKeyring struct{ token string }

func (f *fakeKeyring) Get(_, _ string) (string, error) {
	if f.token == "" {
		return "", errKeyringNotFound
	}
	return f.token, nil
}
func (f *fakeKeyring) Set(_, _, password string) error { f.token = password; return nil }
func (f *fakeKeyring) Delete(_, _ string) error {
	if f.token == "" {
		return errKeyringNotFound
	}
	f.token = ""
	return nil
}

// unavailableKeyring simulates a keychain that is always inaccessible.
type unavailableKeyring struct{ err error }

func (u unavailableKeyring) Get(_, _ string) (string, error)    { return "", u.err }
func (u unavailableKeyring) Set(_, _, _ string) error            { return u.err }
func (u unavailableKeyring) Delete(_, _ string) error            { return u.err }

// notFoundKeyring simulates an empty keychain (no token stored).
type notFoundKeyring struct{}

func (notFoundKeyring) Get(_, _ string) (string, error)    { return "", errKeyringNotFound }
func (notFoundKeyring) Set(_, _, _ string) error            { return nil }
func (notFoundKeyring) Delete(_, _ string) error            { return errKeyringNotFound }

func TestLoad_ErrNotFoundWhenNothing(t *testing.T) {
	keyringBackend = notFoundKeyring{}
	dir := t.TempDir()
	_, err := Load(dir)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestStoreLoad_FileRoundtrip(t *testing.T) {
	keyringBackend = unavailableKeyring{err: errors.New("no keychain")}
	dir := t.TempDir()
	require.NoError(t, Store(dir, "mytoken"))
	tok, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "mytoken", tok)
}

func TestStoreLoad_KeychainRoundtrip(t *testing.T) {
	kb := &fakeKeyring{}
	keyringBackend = kb
	dir := t.TempDir()
	require.NoError(t, Store(dir, "keychain-token"))
	tok, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "keychain-token", tok)
	// Nothing written to file
	_, statErr := os.Stat(filepath.Join(dir, "credentials"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestLoad_KeychainErrorFallsBackToFile(t *testing.T) {
	// First store to file
	keyringBackend = unavailableKeyring{err: errors.New("no keychain")}
	dir := t.TempDir()
	require.NoError(t, Store(dir, "file-token"))

	// Now simulate keychain access error (not "not found")
	accessErr := errors.New("access denied")
	keyringBackend = unavailableKeyring{err: accessErr}

	// Capture stderr
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	tok, err := Load(dir)

	w.Close()
	os.Stderr = old
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])

	require.NoError(t, err)
	assert.Equal(t, "file-token", tok)
	assert.Contains(t, stderr, "keychain unavailable")
	assert.NotContains(t, stderr, "file-token") // token never in warning
}

func TestDelete_RemovesFromKeychain(t *testing.T) {
	kb := &fakeKeyring{token: "tok"}
	keyringBackend = kb
	dir := t.TempDir()
	require.NoError(t, Delete(dir))
	assert.Equal(t, "", kb.token)
}

func TestDelete_RemovesFromFile(t *testing.T) {
	keyringBackend = notFoundKeyring{}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "credentials"), []byte("tok"), 0600))
	require.NoError(t, Delete(dir))
	_, err := os.Stat(filepath.Join(dir, "credentials"))
	assert.True(t, os.IsNotExist(err))
}

func TestDelete_ErrNotFoundWhenNothingStored(t *testing.T) {
	keyringBackend = notFoundKeyring{}
	dir := t.TempDir()
	err := Delete(dir)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestResolve_EnvVarWins(t *testing.T) {
	keyringBackend = notFoundKeyring{}
	dir := t.TempDir()
	t.Setenv("GITHUB_TOOLS_SAP_TOKEN", "env-token")
	assert.Equal(t, "env-token", Resolve(dir))
}

func TestResolve_GHTokenFallback(t *testing.T) {
	keyringBackend = notFoundKeyring{}
	dir := t.TempDir()
	t.Setenv("GITHUB_TOOLS_SAP_TOKEN", "")
	t.Setenv("GH_TOKEN", "gh-token")
	assert.Equal(t, "gh-token", Resolve(dir))
}

func TestResolve_GitHubTokenFallback(t *testing.T) {
	keyringBackend = notFoundKeyring{}
	dir := t.TempDir()
	t.Setenv("GITHUB_TOOLS_SAP_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "github-token")
	assert.Equal(t, "github-token", Resolve(dir))
}

func TestResolve_FileWhenKeyringUnavailable(t *testing.T) {
	keyringBackend = unavailableKeyring{err: errors.New("no keychain")}
	dir := t.TempDir()
	t.Setenv("GITHUB_TOOLS_SAP_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "credentials"), []byte("file-tok"), 0600))
	assert.Equal(t, "file-tok", Resolve(dir))
}

func TestResolve_EmptyWhenNothing(t *testing.T) {
	keyringBackend = notFoundKeyring{}
	dir := t.TempDir()
	t.Setenv("GITHUB_TOOLS_SAP_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	assert.Equal(t, "", Resolve(dir))
}

func TestResolve_KeychainErrorWarnsAndFallsBack(t *testing.T) {
	accessErr := errors.New("permission denied")
	keyringBackend = unavailableKeyring{err: accessErr}
	dir := t.TempDir()
	t.Setenv("GITHUB_TOOLS_SAP_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "credentials"), []byte("file-tok"), 0600))

	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	tok := Resolve(dir)

	w.Close()
	os.Stderr = old
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])

	assert.Equal(t, "file-tok", tok)
	assert.Contains(t, stderr, "keychain unavailable")
	assert.True(t, strings.Contains(stderr, "permission denied"))
	assert.NotContains(t, stderr, "file-tok") // token never in warning
}

func TestCredFile_IsRestrictedPermissions(t *testing.T) {
	keyringBackend = unavailableKeyring{err: errors.New("no keychain")}
	dir := t.TempDir()
	require.NoError(t, Store(dir, "tok"))
	info, err := os.Stat(filepath.Join(dir, "credentials"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/credentials/...
```

Expected: compile error — package does not exist yet.

- [ ] **Step 3: Write the implementation**

Create `internal/credentials/credentials.go`:

```go
package credentials

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goKeyring "github.com/zalando/go-keyring"
)

const (
	keyringSvc  = "sap-devs"
	keyringUser = "github-token"
)

// ErrNotFound is returned by Load and Delete when no token is stored.
var ErrNotFound = errors.New("credentials: no token stored")

// errKeyringNotFound matches zalando/go-keyring's "not found" sentinel.
// We keep a local reference so tests can use it without importing go-keyring.
var errKeyringNotFound = goKeyring.ErrNotFound

// keyring is the interface used by Store/Load/Delete.
// The package-level keyringBackend variable is replaced in tests.
type keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

// osKeyring delegates to the real OS keychain via zalando/go-keyring.
type osKeyring struct{}

func (osKeyring) Get(s, u string) (string, error)    { return goKeyring.Get(s, u) }
func (osKeyring) Set(s, u, p string) error            { return goKeyring.Set(s, u, p) }
func (osKeyring) Delete(s, u string) error            { return goKeyring.Delete(s, u) }

var keyringBackend keyring = osKeyring{}

// Store saves the token to the OS keychain.
// Falls back to <configDir>/credentials (0600) if the keychain is unavailable.
func Store(configDir, token string) error {
	if err := keyringBackend.Set(keyringSvc, keyringUser, token); err == nil {
		return nil
	} else {
		fmt.Fprintf(os.Stderr, "keychain unavailable: %v; token stored in credentials file\n", err)
	}
	return writeCredFile(configDir, token)
}

// Load retrieves the token from the OS keychain or credentials file.
// Returns ErrNotFound if no token is stored.
func Load(configDir string) (string, error) {
	tok, err := keyringBackend.Get(keyringSvc, keyringUser)
	if err == nil {
		return tok, nil
	}
	if errors.Is(err, goKeyring.ErrNotFound) {
		// Not in keychain — try file
		return readCredFile(configDir)
	}
	// Keychain access error — warn and fall back to file
	fmt.Fprintf(os.Stderr, "keychain unavailable: %v; falling back to credentials file\n", err)
	return readCredFile(configDir)
}

// Delete removes the stored token from the keychain or credentials file.
// Returns ErrNotFound if no token was stored anywhere.
func Delete(configDir string) error {
	keychainErr := keyringBackend.Delete(keyringSvc, keyringUser)
	if keychainErr == nil {
		return nil
	}
	// Keychain either had "not found" or an access error — try the file.
	path := credFile(configDir)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

// Resolve returns the best available token using the full priority chain:
// GITHUB_TOOLS_SAP_TOKEN → GH_TOKEN → GITHUB_TOKEN → keychain/file → "".
// Never returns an error.
func Resolve(configDir string) string {
	for _, env := range []string{"GITHUB_TOOLS_SAP_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	tok, err := Load(configDir)
	if err == nil {
		return tok
	}
	return ""
}

func credFile(configDir string) string {
	return filepath.Join(configDir, "credentials")
}

func writeCredFile(configDir, token string) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(credFile(configDir), []byte(token), 0600)
}

func readCredFile(configDir string) (string, error) {
	data, err := os.ReadFile(credFile(configDir))
	if os.IsNotExist(err) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(data))
	if tok == "" {
		return "", ErrNotFound
	}
	return tok, nil
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/credentials/...
```

Expected: all tests pass.

- [ ] **Step 5: Verify build**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/credentials/
git commit -m "feat: add internal/credentials package with keychain and file fallback"
```

---

## Task 3: Update `FetchArchive` — signature, auth header, redirect detection

**Files:**
- Modify: `internal/sync/fetcher.go`
- Modify: `internal/sync/fetcher_test.go`

- [ ] **Step 1: Update existing tests to pass empty token (third argument)**

In `internal/sync/fetcher_test.go`, update the two existing `FetchArchive` calls:

```go
// In TestFetcher_DownloadsAndExtractsZip:
err := sapSync.FetchArchive(srv.URL, dest, "")

// In TestFetcher_BlocksZipSlip:
err := sapSync.FetchArchive(srv.URL, dest, "")
```

- [ ] **Step 2: Add redirect detection and auth header tests**

Append to `internal/sync/fetcher_test.go`:

```go
func TestFetcher_AuthRedirectReturnsActionableError(t *testing.T) {
	// Server that redirects to /login on the same host (simulates GHE auth wall)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>Login page</html>"))
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()

	dest := t.TempDir()
	err := sapSync.FetchArchive(srv.URL+"/repo.zip", dest, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication required")
	assert.Contains(t, err.Error(), "GITHUB_TOOLS_SAP_TOKEN")
	assert.Contains(t, err.Error(), "sap-devs config token")
}

func TestFetcher_SendsAuthHeader(t *testing.T) {
	var gotAuth string
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("repo-main/content/pack.yaml")
	f.Write([]byte("id: test\n"))
	w.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	dest := t.TempDir()
	err := sapSync.FetchArchive(srv.URL, dest, "mytoken")
	require.NoError(t, err)
	assert.Equal(t, "token mytoken", gotAuth)
}

func TestFetcher_NoAuthHeaderWhenTokenEmpty(t *testing.T) {
	var gotAuth string
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("repo-main/content/pack.yaml")
	f.Write([]byte("id: test\n"))
	w.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/zip")
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	dest := t.TempDir()
	require.NoError(t, sapSync.FetchArchive(srv.URL, dest, ""))
	assert.Equal(t, "", gotAuth)
}
```

- [ ] **Step 3: Run tests to confirm new ones fail**

```bash
go test ./internal/sync/...
```

Expected: compile error (wrong number of args to `FetchArchive`).

- [ ] **Step 4: Update `FetchArchive` implementation**

Replace the contents of `internal/sync/fetcher.go`:

```go
package sync

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// FetchArchive downloads a zip archive from rawURL and extracts it to destDir.
// If token is non-empty it is sent as an Authorization header.
// Existing files are overwritten; directories are created as needed.
// GitHub/GitLab archives include a top-level directory prefix which is stripped.
func FetchArchive(rawURL, destDir, token string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil) //nolint:gosec // URL comes from user config, not untrusted input
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: HTTP %d", rawURL, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Auth redirect detection: if we ended up on the login page, surface a clear error.
	// Check resp.Request.URL (the final URL after redirects) for a /login path on the same host.
	if resp.Request.URL.Host == parsedURL.Host && strings.Contains(resp.Request.URL.Path, "/login") {
		return fmt.Errorf("authentication required for %s — set GITHUB_TOOLS_SAP_TOKEN or run 'sap-devs config token'", parsedURL.Host)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	// Strip one leading path component (GitHub archives include repo-name-sha/ prefix)
	strip := zipStripPrefix(zr)

	absBase, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("resolve destDir: %w", err)
	}

	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, strip)
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		dest := filepath.Join(absBase, filepath.FromSlash(name))
		// Zip slip guard: ensure destination is within destDir
		if !strings.HasPrefix(dest, absBase+string(os.PathSeparator)) {
			return fmt.Errorf("zip slip blocked: %q escapes destination", f.Name)
		}
		if err := extractFile(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func zipStripPrefix(zr *zip.Reader) string {
	for _, f := range zr.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) == 2 {
			return parts[0] + "/"
		}
	}
	return ""
}

func extractFile(f *zip.File, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("create dir for %s: %w", dest, err)
	}
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %s: %w", f.Name, err)
	}
	defer rc.Close()
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer out.Close()
	if _, err = io.Copy(out, rc); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	return nil
}
```

- [ ] **Step 5: Run tests to confirm they pass**

```bash
go test ./internal/sync/...
```

Expected: all tests pass.

> **Note:** `go build ./...` will now fail in `cmd/sync.go` because `FetchArchive` is called with 2 args. Do NOT commit yet — complete Task 4 first and commit both together.

---

## Task 4: Update `cmd/sync.go` — resolve and pass token

**Files:**
- Modify: `cmd/sync.go`

- [ ] **Step 1: Update `cmd/sync.go`**

Add the import for `credentials` and resolve the token once before both `FetchArchive` calls. Here is the full updated `syncCmd.RunE` body — replace only the relevant sections:

Add to imports:
```go
"github.com/SAP-samples/sap-devs-cli/internal/credentials"
```

At the top of `syncCmd.RunE`, after loading `cfg` and before the categories/TTL logic, add:
```go
token := credentials.Resolve(paths.ConfigDir)
```

Then update both `FetchArchive` calls:
```go
// Official repo fetch (was line 69):
if err := sapSync.FetchArchive(officialRepoArchive, officialCache, token); err != nil {

// Company repo fetch (was line 87):
if err := sapSync.FetchArchive(companyArchive, companyCache, token); err != nil {
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Commit Tasks 3 and 4 together**

```bash
git add internal/sync/fetcher.go internal/sync/fetcher_test.go cmd/sync.go
git commit -m "feat: update FetchArchive with auth token and detect auth redirects"
```

---

## Task 5: Add `config token` command and update `config show`

**Files:**
- Modify: `cmd/root.go` (add `RootCmd()` accessor)
- Modify: `cmd/config.go`
- Create: `cmd/config_token_test.go`

- [ ] **Step 1: Add `RootCmd()` accessor to `cmd/root.go`**

The tests are in `package cmd_test` (external) and cannot access the unexported `rootCmd` variable directly. Add this exported accessor at the bottom of `cmd/root.go`:

```go
// RootCmd returns the root cobra command. Used in tests.
func RootCmd() *cobra.Command { return rootCmd }
```

- [ ] **Step 2: Write the failing tests**

Create `cmd/config_token_test.go`:

```go
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeCommand runs a cobra command with given args, returns combined output and error.
func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	_, err := root.ExecuteC()
	return buf.String(), err
}

// skipOnWindows skips XDG-dependent tests on Windows (XDG_CONFIG_HOME is not honoured there).
// CI (ubuntu-latest) is the authoritative runner per project convention.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("XDG_CONFIG_HOME isolation not supported on Windows; run in CI")
	}
}

func TestConfigToken_StoreViaArg(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	out, err := executeCommand(RootCmd(), "config", "token", "ghp_testtoken")
	require.NoError(t, err)
	assert.Contains(t, out, "Warning: token passed as argument")
	assert.Contains(t, out, "Token stored securely.")
	assert.NotContains(t, out, "ghp_testtoken") // token must never appear in output
}

func TestConfigToken_NonTTYReturnsError(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// When called with no args in a non-TTY context (test stdin is not a TTY),
	// term.ReadPassword fails and the command should return a non-zero exit with
	// the fallback message — not block silently.
	_, err := executeCommand(RootCmd(), "config", "token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interactive input not available")
	assert.Contains(t, err.Error(), "sap-devs config token <value>")
}

func TestConfigToken_DeleteWhenStored(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	sapDir := filepath.Join(dir, "sap-devs")
	require.NoError(t, os.MkdirAll(sapDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sapDir, "credentials"), []byte("tok"), 0600))

	out, err := executeCommand(RootCmd(), "config", "token", "--delete")
	require.NoError(t, err)
	assert.Contains(t, out, "Token removed.")
}

func TestConfigToken_DeleteWhenNothingStored(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sap-devs"), 0755))

	out, err := executeCommand(RootCmd(), "config", "token", "--delete")
	require.NoError(t, err)
	assert.Contains(t, out, "No token was stored.")
}

func TestConfigToken_DeleteAndValueMutuallyExclusive(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := executeCommand(RootCmd(), "config", "token", "ghp_abc", "--delete")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use --delete with a token value")
}

func TestConfigShow_MasksToken(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	sapDir := filepath.Join(dir, "sap-devs")
	require.NoError(t, os.MkdirAll(sapDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sapDir, "credentials"), []byte("ghp_abcdefgh"), 0600))

	out, err := executeCommand(RootCmd(), "config", "show")
	require.NoError(t, err)
	assert.Contains(t, out, "ghp_****")
	assert.NotContains(t, out, "ghp_abcdefgh")
}

func TestConfigShow_NoTokenDisplaysNotSet(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sap-devs"), 0755))

	out, err := executeCommand(RootCmd(), "config", "show")
	require.NoError(t, err)
	assert.Contains(t, out, "(not set)")
}

func TestConfigToken_DeleteThenShowNotSet(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	sapDir := filepath.Join(dir, "sap-devs")
	require.NoError(t, os.MkdirAll(sapDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sapDir, "credentials"), []byte("tok"), 0600))

	_, err := executeCommand(RootCmd(), "config", "token", "--delete")
	require.NoError(t, err)

	out, err := executeCommand(RootCmd(), "config", "show")
	require.NoError(t, err)
	assert.Contains(t, out, "(not set)")
}
```

- [ ] **Step 3: Run tests to confirm they fail**

```bash
go test ./cmd/... -run TestConfigToken -v
go test ./cmd/... -run TestConfigShow -v
```

Expected: compile errors — `configTokenCmd` not defined, `RootCmd` not found.

- [ ] **Step 4: Update `cmd/config.go`**

Replace the full file content. **Note:** `i18n` is NOT imported — it was never used in `config.go`. The masking logic is extracted to an unexported `maskToken` helper so it can be tested in isolation:

```go
package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/credentials"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage sap-devs configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "company_repo:    %s\n", cfg.CompanyRepo)
		fmt.Fprintf(cmd.OutOrStdout(), "language:        %s\n", cfg.Language)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.tips:       %s\n", cfg.Sync.Tips)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.tools:      %s\n", cfg.Sync.Tools)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.advocates:  %s\n", cfg.Sync.Advocates)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.resources:  %s\n", cfg.Sync.Resources)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.context:    %s\n", cfg.Sync.Context)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.mcp:        %s\n", cfg.Sync.MCP)
		fmt.Fprintf(cmd.OutOrStdout(), "sync.disabled:   %v\n", cfg.Sync.Disabled)

		// Show token status (masked — never show the full value)
		tok, loadErr := credentials.Load(paths.ConfigDir)
		fmt.Fprintf(cmd.OutOrStdout(), "github_token:    %s\n", maskedToken(tok, loadErr))
		return nil
	},
}

// maskedToken returns a display-safe representation of a stored token.
// It is extracted here so tests can verify masking logic directly.
func maskedToken(tok string, err error) string {
	switch {
	case err == nil:
		if len(tok) < 4 {
			return "(set)"
		}
		return tok[:4] + "****"
	case errors.Is(err, credentials.ErrNotFound):
		return "(not set)"
	default:
		return "(unavailable)"
	}
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		switch args[0] {
		case "company_repo":
			cfg.CompanyRepo = args[1]
		case "language":
			cfg.Language = args[1]
		default:
			return fmt.Errorf("unknown config key: %s", args[0])
		}
		if err := cfg.Save(paths.ConfigDir); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", args[0], args[1])
		return nil
	},
}

var configCompanyCmd = &cobra.Command{
	Use:   "company <git-url>",
	Short: "Configure the company content repo URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := xdg.New()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.ConfigDir)
		if err != nil {
			return err
		}
		cfg.CompanyRepo = args[0]
		if err := cfg.Save(paths.ConfigDir); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Company repo set to: %s\n", args[0])
		return nil
	},
}

var tokenDeleteFlag bool

var configTokenCmd = &cobra.Command{
	Use:   "token [value]",
	Short: "Store a GitHub token for authenticating with github.com/SAP-samples",
	Long: `Store a Personal Access Token for authenticating with github.com/SAP-samples.

Only required when syncing content from a private GitHub Enterprise instance
(github.com/SAP-samples). Not needed if you are outside the SAP network or already
have GITHUB_TOOLS_SAP_TOKEN set in your environment.

The token is stored in the OS keychain (macOS Keychain, Windows Credential
Manager, Linux Secret Service). On systems without a keychain, it falls back
to a credentials file with restricted permissions.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if tokenDeleteFlag && len(args) > 0 {
			return fmt.Errorf("cannot use --delete with a token value")
		}

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		if tokenDeleteFlag {
			err := credentials.Delete(paths.ConfigDir)
			if errors.Is(err, credentials.ErrNotFound) {
				fmt.Fprintln(cmd.OutOrStdout(), "No token was stored.")
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Token removed.")
			return nil
		}

		var token string
		if len(args) == 1 {
			token = args[0]
			fmt.Fprintln(cmd.OutOrStdout(), "Warning: token passed as argument may be saved in shell history.")
			fmt.Fprintln(cmd.OutOrStdout(), "Consider using 'sap-devs config token' without arguments for interactive entry.")
		} else {
			fmt.Fprint(cmd.OutOrStdout(), "Enter GitHub token (input hidden, will not appear in shell history): ")
			raw, readErr := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(cmd.OutOrStdout()) // newline after hidden input
			if readErr != nil {
				return fmt.Errorf("interactive input not available — pass token as argument: sap-devs config token <value>")
			}
			token = strings.TrimSpace(string(raw))
			if token == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No token entered.")
				return nil
			}
		}

		if err := credentials.Store(paths.ConfigDir, token); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Token stored securely.")
		return nil
	},
}

func init() {
	configTokenCmd.Flags().BoolVar(&tokenDeleteFlag, "delete", false, "Remove the stored token")
	configCmd.AddCommand(configShowCmd, configSetCmd, configCompanyCmd, configTokenCmd)
	rootCmd.AddCommand(configCmd)
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./cmd/... -run TestConfigToken -v
go test ./cmd/... -run TestConfigShow -v
```

Expected: all pass on Linux/CI. Windows tests are skipped.

- [ ] **Step 6: Run `go mod tidy` to promote `golang.org/x/term` from indirect to direct**

`cmd/config.go` now directly imports `golang.org/x/term`. Run:

```bash
go mod tidy
```

Expected: `go.mod` now lists `golang.org/x/term` as a direct dependency (no `// indirect` annotation).

- [ ] **Step 7: Verify build**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add cmd/root.go cmd/config.go cmd/config_token_test.go go.mod go.sum
git commit -m "feat: add config token command and masked token display in config show"
```

---

## Task 6: Update `init` wizard — add Step 1 (auth prompt), renumber steps

**Files:**
- Modify: `cmd/init.go`

- [ ] **Step 1: Update `cmd/init.go`**

Replace the full file:

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
	"github.com/SAP-samples/sap-devs-cli/internal/credentials"
	"github.com/SAP-samples/sap-devs-cli/internal/xdg"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "First-time setup wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Welcome to sap-devs — your AI-first SAP developer toolkit.")
		fmt.Println()

		paths, err := xdg.New()
		if err != nil {
			return err
		}

		// Step 1: GitHub authentication (optional)
		fmt.Println("Step 1/5: GitHub authentication (optional)")
		fmt.Println()
		fmt.Println("  sap-devs syncs content from github.com/SAP-samples, which requires a Personal")
		fmt.Println("  Access Token if you are inside the SAP corporate network. If you are")
		fmt.Println("  outside SAP or already have GITHUB_TOOLS_SAP_TOKEN set in your")
		fmt.Println("  environment, press Enter to skip.")
		fmt.Println()
		fmt.Print("  GitHub token (press Enter to skip): ")
		raw, termErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // newline after hidden input
		if termErr != nil {
			fmt.Println("  Note: interactive token input unavailable. Run 'sap-devs config token <value>' after setup to authenticate.")
		} else {
			token := strings.TrimSpace(string(raw))
			if token != "" {
				if storeErr := credentials.Store(paths.ConfigDir, token); storeErr != nil {
					fmt.Printf("  Warning: could not store token (%v).\n", storeErr)
				} else {
					fmt.Println("  Token stored securely.")
				}
			}
		}

		// Step 2: Sync content
		fmt.Println("\nStep 2/5: Downloading SAP developer content...")
		if err := runSyncForce(); err != nil {
			fmt.Printf("Warning: content sync failed (%v). Continuing with any cached content.\n", err)
		}

		// Step 3: Choose profile
		fmt.Println("\nStep 3/5: What kind of SAP developer are you?")
		loader, err := newContentLoader()
		if err != nil {
			return err
		}
		profiles, err := loader.LoadProfiles()
		if err != nil {
			return err
		}
		if len(profiles) > 0 {
			for i, p := range profiles {
				fmt.Printf("  [%d] %-25s %s\n", i+1, p.ID, p.Description)
			}
			fmt.Print("\nEnter number (or press Enter to skip): ")
			choice := readLine()
			if choice != "" {
				idx := 0
				fmt.Sscanf(choice, "%d", &idx)
				if idx >= 1 && idx <= len(profiles) {
					chosen := profiles[idx-1]
					if err := config.SaveProfile(paths.ConfigDir, &config.Profile{ID: chosen.ID}); err != nil {
						return err
					}
					fmt.Printf("Profile set to: %s\n", chosen.Name)
				}
			}
		} else {
			fmt.Println("No profiles found. Run 'sap-devs sync' then 'sap-devs profile list'.")
		}

		// Step 4: Inject into AI tools
		fmt.Println("\nStep 4/5: Inject SAP context into your AI tools?")
		fmt.Println("  This writes SAP developer context to your AI tool configuration files.")
		fmt.Print("  Inject now? [Y/n]: ")
		if answer := strings.ToLower(strings.TrimSpace(readLine())); answer == "" || answer == "y" {
			if err := runInjectGlobal(); err != nil {
				fmt.Printf("  Warning: inject failed (%v). You can run 'sap-devs inject' manually.\n", err)
			} else {
				fmt.Println("  SAP context injected into your AI tools.")
			}
		}

		// Step 5: Shell profile hook
		fmt.Println("\nStep 5/5: Add SAP tip to your terminal startup?")
		fmt.Println("  This adds 'sap-devs tip' to your shell profile so you see a tip each time you open a terminal.")
		fmt.Print("  Add it? [y/N]: ")
		if strings.ToLower(strings.TrimSpace(readLine())) == "y" {
			if err := addShellHook(); err != nil {
				fmt.Printf("  Could not auto-add hook: %v\n  Add 'sap-devs tip' to your shell profile manually.\n", err)
			} else {
				fmt.Println("  Added. Restart your terminal to see your first tip.")
			}
		}

		fmt.Println("\nSetup complete! Run 'sap-devs --help' to explore all commands.")
		fmt.Println("Run 'sap-devs inject' to re-inject after syncing new content.")
		return nil
	},
}

func runSyncForce() error {
	syncForce = true
	defer func() { syncForce = false }()
	return syncCmd.RunE(syncCmd, nil)
}

func readLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func addShellHook() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	candidates := []string{".zshrc", ".bashrc", ".bash_profile"}
	for _, rc := range candidates {
		path := home + "/" + rc
		if _, err := os.Stat(path); err == nil {
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			_, err = f.WriteString("\n# SAP developer tips\nsap-devs tip\n")
			f.Close()
			return err
		}
	}
	return fmt.Errorf("no shell rc file found (.zshrc, .bashrc, .bash_profile)")
}

func runInjectGlobal() error {
	prevProject, prevDryRun, prevTool := injectProject, injectDryRun, injectTool
	injectProject = false
	injectDryRun = false
	injectTool = ""
	defer func() { injectProject, injectDryRun, injectTool = prevProject, prevDryRun, prevTool }()
	return injectCmd.RunE(injectCmd, nil)
}

func init() {
	rootCmd.AddCommand(initCmd)
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/init.go
git commit -m "feat: add auth token prompt as Step 1 of init wizard"
```

---

## Task 6a: Test `init` wizard non-TTY path

**Files:**
- Create: `cmd/init_test.go`

The `init` wizard calls `term.ReadPassword` in Step 1. When stdin is not a TTY (piped CI input, scripted usage), `ReadPassword` returns an error. The wizard must skip Step 1 gracefully, print the note, and continue to Step 2 rather than blocking or returning an error.

- [ ] **Step 1: Write the failing test**

Create `cmd/init_test.go`:

```go
package cmd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInit_NonTTYSkipsTokenPrompt verifies that when stdin is not a TTY
// (term.ReadPassword fails), initCmd.RunE does not error on the token step —
// it prints the fallback note and continues to Step 2.
//
// When run in a test environment stdin is not a TTY, so term.ReadPassword
// will always return an error here, exercising the non-TTY branch.
func TestInit_NonTTYSkipsTokenPrompt(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	out, err := executeCommand(RootCmd(), "init")
	// init may fail at sync or inject steps in a test environment — that is OK.
	// The important thing: if it fails, the failure must NOT be the non-TTY error
	// from the token step; the wizard must have continued past Step 1.
	if err != nil {
		assert.NotContains(t, err.Error(), "interactive input not available",
			"init must not fail due to non-TTY token prompt — Step 1 should be skipped gracefully")
	}
	// The non-TTY note must be printed, confirming Step 1 was handled not silently dropped.
	assert.Contains(t, out, "interactive token input unavailable",
		"init must print the non-TTY fallback note for the token step")
}
```

- [ ] **Step 2: Run test to confirm it compiles and exercises the branch**

```bash
go test ./cmd/... -run TestInit_NonTTYSkipsTokenPrompt -v
```

Expected: test passes (the non-TTY path is hit because test stdin is not a TTY). The test may log warnings from sync or inject steps — that is expected.

- [ ] **Step 3: Verify build**

```bash
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add cmd/init_test.go
git commit -m "test: verify init wizard non-TTY path skips token prompt gracefully"
```

---

## Task 7: Documentation updates

**Files:**
- Modify: `docs/user/user-guide.md`
- Modify: `docs/developer/developer-guide.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add Authentication section to `docs/user/user-guide.md`**

Find the `### sync` section (around line 141) and insert a new `### Authentication` section immediately before it:

```markdown
### Authentication

`sap-devs sync` fetches content from `github.com/SAP-samples`, which requires a Personal Access Token if you are inside the SAP corporate network.

**When you need a token:** Only when syncing from `github.com/SAP-samples` on the SAP corporate network. If you are outside SAP, no token is needed.

**Token resolution order** (first match wins):

1. `GITHUB_TOOLS_SAP_TOKEN` environment variable
2. `GH_TOKEN` environment variable
3. `GITHUB_TOKEN` environment variable
4. Token stored with `sap-devs config token`

**Storing a token (interactive — recommended for developer machines):**

```sh
sap-devs config token
# Prompts: Enter GitHub token (input hidden, will not appear in shell history):
```

**Storing a token (non-interactive — scripted or CI):**

```sh
sap-devs config token ghp_yourtoken
# Warning: token passed as argument may be saved in shell history.
```

For CI/CD, set `GITHUB_TOOLS_SAP_TOKEN` as a pipeline secret instead — no local storage needed.

**Where tokens are stored:** The OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service). On headless systems without a keychain, a credentials file at `~/.config/sap-devs/credentials` (Linux) with restricted permissions (owner read/write only). Tokens are **never** stored in `config.yaml`.

**Removing a stored token:**

```sh
sap-devs config token --delete
```

**Viewing token status:**

```sh
sap-devs config show
# github_token:    ghp_****
```
```

- [ ] **Step 2: Add Credentials section to `docs/developer/developer-guide.md`**

Find the `### Sync` section and:

a) Update the Sync description to mention the new token parameter:

```markdown
### Sync

`sap-devs sync` (`cmd/sync.go`) fetches the official repo as a `.zip` archive and extracts it to the cache. Per-category TTLs are tracked in `~/.cache/sap-devs/sync-state.json` via `sync.Engine` (`internal/sync/engine.go`). Use `--force` to ignore TTLs.

The auth token is resolved once at the top of `syncCmd.RunE` via `credentials.Resolve()` and passed to both `FetchArchive` calls (official + company repo). `FetchArchive` signature: `FetchArchive(rawURL, destDir, token string) error`.
```

b) Add a new `### Credentials` section after Sync:

```markdown
### Credentials

`internal/credentials/` manages token storage and resolution.

**Functions:**

| Function | Behaviour |
| --- | --- |
| `Store(configDir, token string) error` | Saves to OS keychain; falls back to `<configDir>/credentials` (0600) if keychain unavailable. Prints an informational stderr note on fallback. |
| `Load(configDir string) (string, error)` | Reads from keychain; falls back to file on keychain error (prints stderr warning). Returns `ErrNotFound` if no token anywhere. |
| `Delete(configDir string) error` | Removes from keychain; falls back to deleting the file. Returns `ErrNotFound` if nothing stored. |
| `Resolve(configDir string) string` | Full priority chain: `GITHUB_TOOLS_SAP_TOKEN` → `GH_TOKEN` → `GITHUB_TOKEN` → `Load()` → `""`. Never errors. |

**Keychain backend:** `zalando/go-keyring` — macOS Keychain, Windows Credential Manager, Linux Secret Service (D-Bus). Falls back to credentials file when unavailable (headless Linux, CI containers).

**Security properties:**
- Token only sent in `Authorization: token <tok>` header, never in URLs or error strings
- `config show` masks the token: `<first4>****` or `(not set)`
- Credentials file is separate from `config.yaml` to prevent accidental dotfile repo exposure

**Testing:** The package uses an unexported `keyringBackend` variable (`type keyring interface`). Tests (`package credentials`) replace it with `fakeKeyring`, `unavailableKeyring`, or `notFoundKeyring` structs to exercise all paths without a real keychain. No real OS keychain is touched in CI.

**Auth redirect detection in `FetchArchive`:** After reading the response body, `FetchArchive` checks `resp.Request.URL.Host == parsedURL.Host && strings.Contains(resp.Request.URL.Path, "/login")`. If matched, it returns: `authentication required for <host> — set GITHUB_TOOLS_SAP_TOKEN or run 'sap-devs config token'`. The host in the error is always from the original URL, not the redirect target.
```

- [ ] **Step 3: Add `internal/credentials/` to `CLAUDE.md`**

In the `CLAUDE.md` architecture overview, find the `ContentLoader` line and add after the existing architecture table entries:

```markdown
`internal/credentials` ([internal/credentials/credentials.go](internal/credentials/credentials.go)) provides secure token storage. `Store`/`Load`/`Delete` use the OS keychain via `zalando/go-keyring` with a `<configDir>/credentials` file fallback (0600). `Resolve()` implements the full priority chain: env vars (`GITHUB_TOOLS_SAP_TOKEN`, `GH_TOKEN`, `GITHUB_TOKEN`) → keychain → file → `""`. Used by `sync` and `config token`.
```

- [ ] **Step 4: Verify build**

```bash
go build ./...
go vet ./...
```

- [ ] **Step 5: Commit**

```bash
git add docs/user/user-guide.md docs/developer/developer-guide.md CLAUDE.md
git commit -m "docs: add authentication and credentials documentation"
```

---

## Task 8: Full verification

- [ ] **Step 1: Run all tests**

```bash
go test ./...
```

Expected: all pass (CI is the authoritative runner; Windows may skip some tests per project convention).

- [ ] **Step 2: Build the binary and smoke-test**

```bash
VERSION=$(git describe --tags --always --dirty)
go build -ldflags "-X github.com/SAP-samples/sap-devs-cli/cmd.Version=${VERSION}" -o sap-devs .
./sap-devs config show
./sap-devs config token --help
./sap-devs init --help
```

Expected: `config show` includes `github_token: (not set)`, `config token --help` shows the long description about when a token is needed.

- [ ] **Step 3: Final commit if any last fixes needed, then push for CI**

```bash
git log --oneline -8
```

Verify all feature commits are present before raising a PR.
