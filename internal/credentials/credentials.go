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

func (osKeyring) Get(s, u string) (string, error) { return goKeyring.Get(s, u) }
func (osKeyring) Set(s, u, p string) error         { return goKeyring.Set(s, u, p) }
func (osKeyring) Delete(s, u string) error          { return goKeyring.Delete(s, u) }

var keyringBackend keyring = osKeyring{}

// Store saves the token to the OS keychain.
// Falls back to <configDir>/credentials (0600) if the keychain is unavailable.
func Store(configDir, token string) error { return storeForUser(configDir, keyringUser, token) }

// Load retrieves the token from the OS keychain or credentials file.
// Returns ErrNotFound if no token is stored.
func Load(configDir string) (string, error) { return loadForUser(configDir, keyringUser) }

// Delete removes the stored token from the keychain or credentials file.
// Returns ErrNotFound if no token was stored anywhere.
func Delete(configDir string) error { return deleteForUser(configDir, keyringUser) }

// Resolve returns the best available token using the full priority chain:
// GITHUB_TOOLS_SAP_TOKEN -> GH_TOKEN -> GITHUB_TOKEN -> keychain/file -> "".
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

// StoreService saves a service-specific token to the OS keychain.
// Falls back to <configDir>/credentials-<service> (0600) if the keychain is unavailable.
func StoreService(configDir, service, token string) error {
	return storeForUser(configDir, service, token)
}

// LoadService retrieves a service-specific token from the OS keychain or credentials file.
// Returns ErrNotFound if no token is stored.
func LoadService(configDir, service string) (string, error) {
	return loadForUser(configDir, service)
}

// DeleteService removes a service-specific token from the keychain or credentials file.
// Returns ErrNotFound if no token was stored anywhere.
func DeleteService(configDir, service string) error {
	return deleteForUser(configDir, service)
}

// ResolveService returns the best available token for a service using the priority chain:
// env vars (in order) -> keychain/file -> "".
// Never returns an error.
func ResolveService(configDir, service string, envVars []string) string {
	for _, env := range envVars {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	tok, err := LoadService(configDir, service)
	if err == nil {
		return tok
	}
	return ""
}

// storeForUser saves a token keyed by user to the OS keychain,
// falling back to a credentials file if the keychain is unavailable.
func storeForUser(configDir, user, token string) error {
	err := keyringBackend.Set(keyringSvc, user, token)
	if err == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "keychain unavailable: %v; token stored in credentials file\n", err)
	return writeCredFileForUser(configDir, user, token)
}

// loadForUser retrieves a token keyed by user from the OS keychain or credentials file.
func loadForUser(configDir, user string) (string, error) {
	tok, err := keyringBackend.Get(keyringSvc, user)
	if err == nil {
		return tok, nil
	}
	if errors.Is(err, goKeyring.ErrNotFound) {
		// Not in keychain — try file
		return readCredFileForUser(configDir, user)
	}
	// Keychain access error — warn and fall back to file
	fmt.Fprintf(os.Stderr, "keychain unavailable: %v; falling back to credentials file\n", err)
	return readCredFileForUser(configDir, user)
}

// deleteForUser removes a token keyed by user from the keychain and credentials file.
func deleteForUser(configDir, user string) error {
	keychainErr := keyringBackend.Delete(keyringSvc, user)
	if keychainErr != nil && !errors.Is(keychainErr, errKeyringNotFound) {
		// Keychain unavailable or access error — warn and fall through to file,
		// matching the Store/Load fallback pattern.
		fmt.Fprintf(os.Stderr, "keychain unavailable: %v; trying credentials file\n", keychainErr)
	}
	// Keychain either succeeded, reported "not found", or is unavailable.
	// Also remove the fallback credentials file if present.
	path := credFileForUser(configDir, user)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		if keychainErr == nil {
			// Keychain delete succeeded; file absence is fine.
			return nil
		}
		// Token was neither in keychain nor in file.
		return ErrNotFound
	}
	return err
}

// credFileForUser returns the path to the credentials file for the given user.
// The legacy keyringUser maps to the original "credentials" filename for backward compatibility.
func credFileForUser(configDir, user string) string {
	if user == keyringUser {
		return filepath.Join(configDir, "credentials")
	}
	return filepath.Join(configDir, "credentials-"+user)
}

func writeCredFileForUser(configDir, user, token string) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(credFileForUser(configDir, user), []byte(token), 0600)
}

func readCredFileForUser(configDir, user string) (string, error) {
	data, err := os.ReadFile(credFileForUser(configDir, user))
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
