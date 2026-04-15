package shellhook

import (
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
	if len(profiles) != 3 {
		t.Fatalf("expected 3 windows profiles, got %d: %v", len(profiles), profiles)
	}
	if !strings.Contains(profiles[0], "PowerShell") {
		t.Errorf("expected PowerShell profile first, got %q", profiles[0])
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

