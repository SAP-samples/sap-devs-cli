package shellhook

import (
	"os"
	"path/filepath"
	"runtime"
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
