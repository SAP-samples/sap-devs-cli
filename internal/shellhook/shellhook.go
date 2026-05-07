package shellhook

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Shell identifies a shell type.
type Shell int

const (
	ShellUnknown Shell = iota
	ShellPowerShell
	ShellBash
	ShellZsh
)

// detectShellEnv is the detection function variable, replaceable in tests.
var detectShellEnv = detectShell

// detectShell returns the active shell based on environment heuristics.
func detectShell() Shell {
	if os.Getenv("PSModulePath") != "" {
		return ShellPowerShell
	}
	sh := os.Getenv("SHELL")
	if strings.Contains(sh, "zsh") {
		return ShellZsh
	}
	if strings.Contains(sh, "bash") {
		return ShellBash
	}
	return ShellUnknown
}

// primaryProfileIndex returns the index into candidates that corresponds to the
// detected shell's profile, or -1 if none matches.
func primaryProfileIndex(shell Shell, candidates []string) int {
	for i, c := range candidates {
		switch shell {
		case ShellPowerShell:
			if strings.HasSuffix(c, ".ps1") {
				return i
			}
		case ShellZsh:
			if strings.HasSuffix(c, ".zshrc") {
				return i
			}
		case ShellBash:
			if strings.HasSuffix(c, ".bashrc") {
				return i
			}
		}
	}
	return -1
}

// ensureFileExists creates the file (and parent directories) if it does not
// already exist. Returns nil if the file already exists.
func ensureFileExists(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

// Result describes what happened to a single profile file.
type Result struct {
	Path    string
	Updated bool // false = already present (Add) or line not found (Remove)
}

// currentOS is the GOOS value used by candidateProfiles, replaceable in tests.
var currentOS = runtime.GOOS

// homeDir is a variable so tests can substitute a temp directory.
var homeDir = os.UserHomeDir

// candidateProfiles returns platform-appropriate shell profile paths
// rooted at homeDir(). Does not check whether paths exist.
func candidateProfiles() ([]string, error) {
	home, err := homeDir()
	if err != nil {
		return nil, err
	}
	return profilesForOS(currentOS, home), nil
}

// profilesForOS returns candidate profile paths for a given GOOS and home
// directory. Kept separate so tests can exercise any platform without
// running on it.
func profilesForOS(goos, home string) []string {
	if goos == "windows" {
		return []string{
			filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
			filepath.Join(home, ".bashrc"),
			filepath.Join(home, ".bash_profile"),
		}
	}
	return []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".zprofile"),
	}
}

// Add appends comment and line to the detected shell's primary profile.
// Only one profile is written to avoid duplication on platforms where
// multiple profiles are sourced in the same session (e.g. Git Bash sources
// both .bashrc and .bash_profile). Falls back to the first existing
// candidate if the shell cannot be detected.
func Add(line, comment string) ([]Result, error) {
	candidates, err := candidateProfiles()
	if err != nil {
		return nil, err
	}
	shell := detectShellEnv()
	if idx := primaryProfileIndex(shell, candidates); idx >= 0 {
		if err := ensureFileExists(candidates[idx]); err != nil {
			return nil, fmt.Errorf("creating profile %s: %w", candidates[idx], err)
		}
		return addToProfiles(line, comment, []string{candidates[idx]})
	}
	// Unknown shell — use first existing candidate only.
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return addToProfiles(line, comment, []string{c})
		}
	}
	return nil, fmt.Errorf("no shell profile found; add %q to your shell profile manually", line)
}

// addToProfiles is the testable core of Add: it operates on an explicit
// list of paths rather than calling candidateProfiles().
func addToProfiles(line, comment string, candidates []string) ([]Result, error) {
	var results []Result
	var errs []error

	for _, path := range candidates {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		if hasLine(string(data), line) {
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		_, writeErr := fmt.Fprintf(f, "\n%s\n%s\n", comment, line)
		f.Close()
		if writeErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, writeErr))
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		results = append(results, Result{Path: path, Updated: true})
	}

	if len(results) == 0 && len(errs) == 0 {
		return nil, fmt.Errorf("no shell profile found; add %q to your shell profile manually", line)
	}
	return results, errors.Join(errs...)
}

// hasLine reports whether s contains line as a complete line.
// bufio.Scanner trims \r\n automatically, so CRLF files are handled correctly.
func hasLine(s, line string) bool {
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		if scanner.Text() == line {
			return true
		}
	}
	return false
}

// Remove strips every occurrence of line (full-line match) and any
// immediately preceding line equal to comment from all existing profiles.
// Returns one Result per candidate profile found on disk.
func Remove(line, comment string) ([]Result, error) {
	candidates, err := candidateProfiles()
	if err != nil {
		return nil, err
	}
	return removeFromProfiles(line, comment, candidates)
}

// removeFromProfiles is the testable core of Remove.
// Note: splitting on "\n" and joining back on "\n" preserves CRLF files
// naturally — lines retain their trailing \r, and comparison uses
// strings.TrimRight to strip it before matching.
func removeFromProfiles(line, comment string, candidates []string) ([]Result, error) {
	var results []Result
	var errs []error

	for _, path := range candidates {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			results = append(results, Result{Path: path, Updated: false})
			continue
		}

		rawLines := strings.Split(string(data), "\n")
		out := make([]string, 0, len(rawLines))
		changed := false

		for _, l := range rawLines {
			if strings.TrimRight(l, "\r") == line {
				// Remove this hook line and its immediately preceding comment.
				if len(out) > 0 && strings.TrimRight(out[len(out)-1], "\r") == comment {
					out = out[:len(out)-1]
				}
				changed = true
				continue
			}
			out = append(out, l)
		}

		if !changed {
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0644); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			results = append(results, Result{Path: path, Updated: false})
			continue
		}
		results = append(results, Result{Path: path, Updated: true})
	}

	return results, errors.Join(errs...)
}
