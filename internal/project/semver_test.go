package project

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"9.6.2", "9.8.0", -1},
		{"9.8.0", "9.6.2", 1},
		{"9.8.0", "9.8.0", 0},
		{"10.0.0", "9.99.99", 1},
		{"1.2.3", "1.2.4", -1},
		{"invalid", "9.8.0", 0},
		{"9.8.0", "invalid", 0},
	}
	for _, tt := range tests {
		got := CompareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestVersionStaleness(t *testing.T) {
	tests := []struct {
		current, latest string
		wantSev         string
	}{
		{"9.6.0", "9.8.0", "warning"},  // 2 minor behind
		{"9.7.0", "9.8.0", ""},         // 1 minor behind — ok
		{"8.0.0", "9.8.0", "error"},    // 1 major behind
		{"9.8.0", "9.8.0", ""},         // up to date
		{"9.9.0", "9.8.0", ""},         // ahead — ok
		{"invalid", "9.8.0", ""},       // unparseable — skip
	}
	for _, tt := range tests {
		got := VersionStaleness(tt.current, tt.latest)
		if got != tt.wantSev {
			t.Errorf("VersionStaleness(%q, %q) = %q, want %q", tt.current, tt.latest, got, tt.wantSev)
		}
	}
}
