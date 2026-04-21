package cfcli

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
	case strings.Contains(lower, "not logged in"):
		return &AuthError{CLI: "cf", Message: "Not logged in to Cloud Foundry."}
	case strings.Contains(lower, "no api endpoint set"):
		return &AuthError{CLI: "cf", Message: "No API endpoint set. Run 'cf api' first."}
	case strings.Contains(lower, "failed") && strings.Contains(lower, "not authenticated"):
		return &AuthError{CLI: "cf", Message: "Authentication expired. Run 'cf login' to re-authenticate."}
	}
	return nil
}

func checkNotInstalled(err error) *NotInstalledError {
	if err != nil && errors.Is(err, exec.ErrNotFound) {
		return &NotInstalledError{CLI: "cf", Message: "Cloud Foundry CLI is not installed."}
	}
	return nil
}
