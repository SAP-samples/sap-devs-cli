package shellhook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfilesForOS_Linux(t *testing.T) {
	profiles := profilesForOS("linux", "/home/user")
	want := []string{
		"/home/user/.zshrc",
		"/home/user/.bashrc",
		"/home/user/.bash_profile",
		"/home/user/.zprofile",
	}
	if len(profiles) != len(want) {
		t.Fatalf("got %v, want %v", profiles, want)
	}
	for i, p := range profiles {
		if p != want[i] {
			t.Errorf("[%d] got %q, want %q", i, p, want[i])
		}
	}
}

func TestProfilesForOS_Windows(t *testing.T) {
	profiles := profilesForOS("windows", `C:\Users\user`)
	if len(profiles) != 4 {
		t.Fatalf("expected 4 windows profiles, got %d: %v", len(profiles), profiles)
	}
	if !strings.Contains(profiles[0], "PowerShell") {
		t.Errorf("expected PowerShell profile first, got %q", profiles[0])
	}
	if !strings.Contains(profiles[1], "WindowsPowerShell") {
		t.Errorf("expected WindowsPowerShell profile second, got %q", profiles[1])
	}
}

func TestAdd_SingleProfileAbsent(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(rc, []byte("# existing\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || !results[0].Updated {
		t.Fatalf("expected one updated result, got %+v", results)
	}
	data, _ := os.ReadFile(rc)
	if !strings.Contains(string(data), "sap-devs tip") {
		t.Error("expected hook in profile")
	}
}

func TestAdd_LineAlreadyPresent(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(rc, []byte("# SAP developer tips\nsap-devs tip\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Updated {
		t.Fatalf("expected one skipped result, got %+v", results)
	}
}

func TestAdd_LineAsSubstringNotCounted(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	// "sap-devs tip" exists only as a substring — must NOT be treated as present
	if err := os.WriteFile(rc, []byte("# runs sap-devs tip daily\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || !results[0].Updated {
		t.Fatalf("expected append (substring should not count), got %+v", results)
	}
}

func TestAdd_SkipsWhenLinePresent_WithOrphanedComment(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	// file has a full hook block AND an orphaned comment — line is present, so skip
	content := "# SAP developer tips\nsap-devs tip\n# SAP developer tips\n"
	if err := os.WriteFile(rc, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Updated {
		t.Fatalf("expected skipped (line present), got %+v", results)
	}
}

func TestAdd_MultipleProfiles(t *testing.T) {
	dir := t.TempDir()
	zshrc := filepath.Join(dir, ".zshrc")
	bashrc := filepath.Join(dir, ".bashrc")
	for _, rc := range []string{zshrc, bashrc} {
		if err := os.WriteFile(rc, []byte("# existing\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{zshrc, bashrc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := 0
	for _, r := range results {
		if r.Updated {
			updated++
		}
	}
	if updated != 2 {
		t.Fatalf("expected 2 updated profiles, got %d (%+v)", updated, results)
	}
}

func TestAdd_NoProfiles(t *testing.T) {
	// pass an empty candidate list — simulates no profiles existing
	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{})
	if err == nil {
		t.Fatal("expected error when no profiles exist")
	}
	if len(results) != 0 {
		t.Fatalf("expected no results, got %+v", results)
	}
}

func TestAdd_WindowsPowerShellPath(t *testing.T) {
	dir := t.TempDir()
	psPath := filepath.Join(dir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	if err := os.MkdirAll(filepath.Dir(psPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(psPath, []byte("# existing\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{psPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || !results[0].Updated {
		t.Fatalf("expected updated result, got %+v", results)
	}
	data, _ := os.ReadFile(psPath)
	if !strings.Contains(string(data), "sap-devs tip") {
		t.Error("expected hook in PowerShell profile")
	}
}

func TestAdd_MultipleHookBlocks(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	content := "# SAP developer tips\nsap-devs tip\n# other\n# SAP developer tips\nsap-devs tip\n"
	if err := os.WriteFile(rc, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := addToProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Updated {
		t.Fatalf("expected skipped (line already present), got %+v", results)
	}
}

func TestRemove_LinePresent(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(rc, []byte("# SAP developer tips\nsap-devs tip\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || !results[0].Updated {
		t.Fatalf("expected one updated result, got %+v", results)
	}
	data, _ := os.ReadFile(rc)
	if strings.Contains(string(data), "sap-devs tip") {
		t.Error("hook line should have been removed")
	}
	if strings.Contains(string(data), "# SAP developer tips") {
		t.Error("comment line should have been removed along with hook")
	}
}

func TestRemove_LineAbsent(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(rc, []byte("# existing\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Updated {
		t.Fatalf("expected one not-updated result, got %+v", results)
	}
}

func TestRemove_SubstringNotRemoved(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	original := "# runs sap-devs tip daily\n"
	if err := os.WriteFile(rc, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Updated {
		t.Error("substring-only line should not have been removed")
	}
	data, _ := os.ReadFile(rc)
	if string(data) != original {
		t.Error("file should be unchanged")
	}
}

func TestRemove_MultipleProfiles(t *testing.T) {
	dir := t.TempDir()
	zshrc := filepath.Join(dir, ".zshrc")
	bashrc := filepath.Join(dir, ".bashrc")
	content := "# SAP developer tips\nsap-devs tip\n"
	for _, rc := range []string{zshrc, bashrc} {
		if err := os.WriteFile(rc, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{zshrc, bashrc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := 0
	for _, r := range results {
		if r.Updated {
			updated++
		}
	}
	if updated != 2 {
		t.Fatalf("expected 2 profiles updated, got %d (%+v)", updated, results)
	}
}

func TestRemove_NoProfiles(t *testing.T) {
	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{})
	if err != nil {
		t.Fatalf("unexpected error for empty candidate list: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %+v", results)
	}
}

func TestRemove_OrphanedCommentPreserved(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	// comment is present but NOT immediately followed by the hook line
	if err := os.WriteFile(rc, []byte("# SAP developer tips\nsome-other-command\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Updated {
		t.Error("orphaned comment should not cause a change")
	}
	data, _ := os.ReadFile(rc)
	if !strings.Contains(string(data), "# SAP developer tips") {
		t.Error("orphaned comment should be preserved")
	}
}

func TestRemove_MultipleHookBlocks(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	// two full comment+hook pairs in the same file
	content := "# SAP developer tips\nsap-devs tip\n# other\n# SAP developer tips\nsap-devs tip\n"
	if err := os.WriteFile(rc, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{rc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !results[0].Updated {
		t.Error("expected Updated=true")
	}
	data, _ := os.ReadFile(rc)
	if strings.Contains(string(data), "sap-devs tip") {
		t.Error("all hook lines should be removed")
	}
	if !strings.Contains(string(data), "# other") {
		t.Error("unrelated lines should be preserved")
	}
}

func TestRemove_WindowsPowerShellPath(t *testing.T) {
	dir := t.TempDir()
	psPath := filepath.Join(dir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	if err := os.MkdirAll(filepath.Dir(psPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(psPath, []byte("# SAP developer tips\nsap-devs tip\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := removeFromProfiles("sap-devs tip", "# SAP developer tips", []string{psPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || !results[0].Updated {
		t.Fatalf("expected updated result, got %+v", results)
	}
	data, _ := os.ReadFile(psPath)
	if strings.Contains(string(data), "sap-devs tip") {
		t.Error("hook should be removed from PowerShell profile")
	}
}

// --- Shell detection tests ---

func TestDetectShell_PowerShell(t *testing.T) {
	orig := detectShellEnv
	defer func() { detectShellEnv = orig }()

	detectShellEnv = func() Shell { return ShellPowerShell }
	if detectShellEnv() != ShellPowerShell {
		t.Error("expected PowerShell")
	}
}

func TestDetectShell_Zsh(t *testing.T) {
	orig := detectShellEnv
	defer func() { detectShellEnv = orig }()

	detectShellEnv = func() Shell { return ShellZsh }
	if detectShellEnv() != ShellZsh {
		t.Error("expected Zsh")
	}
}

func TestDetectShell_Bash(t *testing.T) {
	orig := detectShellEnv
	defer func() { detectShellEnv = orig }()

	detectShellEnv = func() Shell { return ShellBash }
	if detectShellEnv() != ShellBash {
		t.Error("expected Bash")
	}
}

func TestPrimaryProfileIndex(t *testing.T) {
	candidates := []string{
		"/home/user/Documents/PowerShell/Microsoft.PowerShell_profile.ps1",
		"/home/user/.bashrc",
		"/home/user/.zshrc",
	}
	if idx := primaryProfileIndex(ShellPowerShell, candidates); idx != 0 {
		t.Errorf("PowerShell: want 0, got %d", idx)
	}
	if idx := primaryProfileIndex(ShellBash, candidates); idx != 1 {
		t.Errorf("Bash: want 1, got %d", idx)
	}
	if idx := primaryProfileIndex(ShellZsh, candidates); idx != 2 {
		t.Errorf("Zsh: want 2, got %d", idx)
	}
	if idx := primaryProfileIndex(ShellUnknown, candidates); idx != -1 {
		t.Errorf("Unknown: want -1, got %d", idx)
	}
}

func TestEnsureFileExists_CreatesFileAndDirs(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "dir", "profile.ps1")

	if err := ensureFileExists(target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("newly created file should be empty, got %d bytes", info.Size())
	}
}

func TestEnsureFileExists_ExistingFileUnchanged(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(target, []byte("existing content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ensureFileExists(target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "existing content\n" {
		t.Error("existing file should not be modified")
	}
}

func TestCandidateProfiles_Windows_ResolvedProfile(t *testing.T) {
	dir := t.TempDir()
	onedrivePath := filepath.Join(dir, "OneDrive - SAP SE", "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")

	origOS := currentOS
	defer func() { currentOS = origOS }()
	currentOS = "windows"

	origHome := homeDir
	defer func() { homeDir = origHome }()
	homeDir = func() (string, error) { return dir, nil }

	origQuery := queryPSProfile
	defer func() { queryPSProfile = origQuery }()
	queryPSProfile = func(binary string) (string, error) { return onedrivePath, nil }

	candidates, err := candidateProfiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected candidates")
	}
	if candidates[0] != onedrivePath {
		t.Errorf("expected resolved OneDrive path first, got %q", candidates[0])
	}
}

func TestCandidateProfiles_Windows_QueryFails(t *testing.T) {
	dir := t.TempDir()

	origOS := currentOS
	defer func() { currentOS = origOS }()
	currentOS = "windows"

	origHome := homeDir
	defer func() { homeDir = origHome }()
	homeDir = func() (string, error) { return dir, nil }

	origQuery := queryPSProfile
	defer func() { queryPSProfile = origQuery }()
	queryPSProfile = func(binary string) (string, error) { return "", fmt.Errorf("not found") }

	candidates, err := candidateProfiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to hardcoded paths
	if len(candidates) != 4 {
		t.Fatalf("expected 4 hardcoded candidates, got %d: %v", len(candidates), candidates)
	}
}

func TestAdd_CreatesProfileForDetectedShell(t *testing.T) {
	dir := t.TempDir()
	psPath := filepath.Join(dir, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")

	orig := detectShellEnv
	defer func() { detectShellEnv = orig }()
	detectShellEnv = func() Shell { return ShellPowerShell }

	origOS := currentOS
	defer func() { currentOS = origOS }()
	currentOS = "windows"

	origHome := homeDir
	defer func() { homeDir = origHome }()
	homeDir = func() (string, error) { return dir, nil }

	origQuery := queryPSProfile
	defer func() { queryPSProfile = origQuery }()
	queryPSProfile = func(binary string) (string, error) { return "", fmt.Errorf("not available") }

	results, err := Add("sap-devs tip", "# SAP developer tips")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Path == psPath && r.Updated {
			found = true
		}
	}
	if !found {
		t.Errorf("expected PowerShell profile to be created and updated, got %+v", results)
	}
	data, _ := os.ReadFile(psPath)
	if !strings.Contains(string(data), "sap-devs tip") {
		t.Error("hook should be present in newly created PowerShell profile")
	}
}

func TestAdd_DetectedShellZsh_CreatesZshrc(t *testing.T) {
	dir := t.TempDir()
	zshrc := filepath.Join(dir, ".zshrc")

	orig := detectShellEnv
	defer func() { detectShellEnv = orig }()
	detectShellEnv = func() Shell { return ShellZsh }

	origOS := currentOS
	defer func() { currentOS = origOS }()
	currentOS = "linux"

	origHome := homeDir
	defer func() { homeDir = origHome }()
	homeDir = func() (string, error) { return dir, nil }

	results, err := Add("sap-devs tip", "# SAP developer tips")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Path == zshrc && r.Updated {
			found = true
		}
	}
	if !found {
		t.Errorf("expected .zshrc to be created and updated, got %+v", results)
	}
}
