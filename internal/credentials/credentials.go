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
func Store(configDir, token string) error {
	err := keyringBackend.Set(keyringSvc, keyringUser, token)
	if err == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "keychain unavailable: %v; token stored in credentials file\n", err)
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
