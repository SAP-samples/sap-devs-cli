package btpcli

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type AuthError struct {
	CLI     string
	Message string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("%s: %s", e.CLI, e.Message)
}

type NotInstalledError struct {
	CLI     string
	Message string
}

func (e *NotInstalledError) Error() string {
	return fmt.Sprintf("%s: %s", e.CLI, e.Message)
}

func checkAuthError(output string) *AuthError {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "login required"):
		return &AuthError{CLI: "btp", Message: "Login required. Run 'btp login' to authenticate."}
	case strings.Contains(lower, "you are not logged in"):
		return &AuthError{CLI: "btp", Message: "Not logged in. Run 'btp login' to authenticate."}
	case strings.Contains(lower, "session has expired"):
		return &AuthError{CLI: "btp", Message: "Session expired. Run 'btp login' to re-authenticate."}
	}
	return nil
}

func checkNotInstalled(err error) *NotInstalledError {
	if err != nil && errors.Is(err, exec.ErrNotFound) {
		return &NotInstalledError{CLI: "btp", Message: "BTP CLI is not installed."}
	}
	return nil
}
