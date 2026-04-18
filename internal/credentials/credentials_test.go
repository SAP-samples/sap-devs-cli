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

func TestDelete_KeychainUnavailableNoFile(t *testing.T) {
	keyringBackend = unavailableKeyring{err: errors.New("permission denied")}
	dir := t.TempDir()
	err := Delete(dir)
	// Keychain unavailable + no file = nothing stored anywhere → ErrNotFound.
	// Matches the Store/Load fallback pattern: warn to stderr, don't surface the
	// keychain error as a failure when the token was likely stored in the file.
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCredFile_IsRestrictedPermissions(t *testing.T) {
	keyringBackend = unavailableKeyring{err: errors.New("no keychain")}
	dir := t.TempDir()
	require.NoError(t, Store(dir, "tok"))
	info, err := os.Stat(filepath.Join(dir, "credentials"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestStoreLoadService_KeychainRoundtrip(t *testing.T) {
	kb := &fakeKeyring{}
	keyringBackend = kb
	dir := t.TempDir()
	require.NoError(t, StoreService(dir, "youtube", "yt-key"))
	tok, err := LoadService(dir, "youtube")
	require.NoError(t, err)
	assert.Equal(t, "yt-key", tok)
}

func TestStoreLoadService_FileRoundtrip(t *testing.T) {
	keyringBackend = unavailableKeyring{err: errors.New("no keychain")}
	dir := t.TempDir()
	require.NoError(t, StoreService(dir, "youtube", "yt-key"))
	tok, err := LoadService(dir, "youtube")
	require.NoError(t, err)
	assert.Equal(t, "yt-key", tok)
	// Verify service-specific file name
	_, statErr := os.Stat(filepath.Join(dir, "credentials-youtube"))
	assert.NoError(t, statErr)
}

func TestDeleteService_RemovesToken(t *testing.T) {
	kb := &fakeKeyring{token: "yt-key"}
	keyringBackend = kb
	dir := t.TempDir()
	require.NoError(t, DeleteService(dir, "youtube"))
	assert.Equal(t, "", kb.token)
}

func TestResolveService_EnvVarWins(t *testing.T) {
	keyringBackend = notFoundKeyring{}
	dir := t.TempDir()
	t.Setenv("YOUTUBE_API_KEY", "env-key")
	assert.Equal(t, "env-key", ResolveService(dir, "youtube", []string{"YOUTUBE_API_KEY"}))
}

func TestResolveService_EmptyWhenNothing(t *testing.T) {
	keyringBackend = notFoundKeyring{}
	dir := t.TempDir()
	t.Setenv("YOUTUBE_API_KEY", "")
	assert.Equal(t, "", ResolveService(dir, "youtube", []string{"YOUTUBE_API_KEY"}))
}

func TestExistingStore_StillWorks(t *testing.T) {
	kb := &fakeKeyring{}
	keyringBackend = kb
	dir := t.TempDir()
	require.NoError(t, Store(dir, "gh-token"))
	tok, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "gh-token", tok)
}
