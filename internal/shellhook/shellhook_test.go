package shellhook

import (
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
