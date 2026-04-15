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

// Result describes what happened to a single profile file.
type Result struct {
	Path    string
	Updated bool // false = already present (Add) or line not found (Remove)
}

// homeDir is a variable so tests can substitute a temp directory.
var homeDir = os.UserHomeDir

// candidateProfiles returns platform-appropriate shell profile paths
// rooted at homeDir(). Does not check whether paths exist.
func candidateProfiles() ([]string, error) {
	home, err := homeDir()
	if err != nil {
		return nil, err
	}
	return profilesForOS(runtime.GOOS, home), nil
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

// Add appends comment and line to every existing profile that does not
// already contain line as a complete line. Returns one Result per
// candidate profile found on disk.
func Add(line, comment string) ([]Result, error) {
	candidates, err := candidateProfiles()
	if err != nil {
		return nil, err
	}
	return addToProfiles(line, comment, candidates)
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
