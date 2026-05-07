package adapter

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveSelfCommand returns the command string with "sap-devs" replaced by
// the absolute path of the running binary. This ensures MCP hosts and hook
// runners can find the binary without relying on shell PATH.
// If the command does not reference sap-devs, it is returned unchanged.
func ResolveSelfCommand(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return command
	}
	if !isSelfRef(parts[0]) {
		return command
	}
	exe, err := resolveExecutable()
	if err != nil {
		return command
	}
	parts[0] = exe
	return strings.Join(parts, " ")
}

// ResolveSelfArgs returns the command (first word only) resolved to an absolute
// path if it references sap-devs. Used for MCP configs where command and args
// are separate fields.
func ResolveSelfArgs(command string) string {
	if !isSelfRef(command) {
		return command
	}
	exe, err := resolveExecutable()
	if err != nil {
		return command
	}
	return exe
}

func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe, nil
	}
	return resolved, nil
}

func isSelfRef(cmd string) bool {
	base := filepath.Base(cmd)
	base = strings.TrimSuffix(base, ".exe")
	return base == "sap-devs"
}
