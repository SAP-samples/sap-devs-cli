package i18n

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name    string
		cfgLang string
		lang    string
		lcAll   string
		want    string
	}{
		{"config wins over env", "de", "fr", "", "de"},
		{"config with unknown lang falls back to en", "klingon", "", "", "en"},
		{"LANG used when config empty", "", "de_AT.UTF-8", "", "de"},
		{"LC_ALL used when LANG empty", "", "", "de", "de"},
		{"LANG wins over LC_ALL", "", "de", "fr", "de"},
		{"LANG empty string treated as unset", "", "", "de", "de"},
		{"unknown LANG falls back to en", "", "fr_FR.UTF-8", "", "en"},
		{"no input returns en", "", "", "", "en"},
		{"config de_AT strips to de", "de_AT.UTF-8", "", "", "de"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LANG", tt.lang)
			t.Setenv("LC_ALL", tt.lcAll)
			assert.Equal(t, tt.want, Resolve(tt.cfgLang))
		})
	}
}

func TestT(t *testing.T) {
	// Key exists in de catalog
	assert.Equal(t, "KI-gestütztes SAP Entwickler-Toolkit", T("de", "root.short"))
	// Key absent in de, falls back to en
	assert.Equal(t, "SAP developer context injected ({{.Scope}} scope).", T("de", "inject.done"))
	// Key absent from both catalogs — returns raw key
	assert.Equal(t, "missing.key", T("de", "missing.key"))
}

func TestLookup(t *testing.T) {
	// Found in de catalog
	v, ok := Lookup("de", "root.short")
	assert.True(t, ok)
	assert.NotEmpty(t, v)

	// Not in de, found in en fallback
	v, ok = Lookup("de", "inject.done")
	assert.True(t, ok)
	assert.Contains(t, v, "{{.Scope}}")

	// Not in either
	_, ok = Lookup("de", "missing.key")
	assert.False(t, ok)
}

func TestTf(t *testing.T) {
	// Template substitution using en fallback
	got := Tf("de", "inject.done", map[string]any{"Scope": "global"})
	assert.Equal(t, "SAP developer context injected (global scope).", got)

	// Missing data key with missingkey=error → raw template string
	got = Tf("de", "inject.done", map[string]any{})
	assert.Equal(t, "SAP developer context injected ({{.Scope}} scope).", got)

	// nil data with template action → raw template string
	got = Tf("de", "inject.done", nil)
	assert.Equal(t, "SAP developer context injected ({{.Scope}} scope).", got)
}

func TestActiveLang(t *testing.T) {
	// ActiveLang is a package-level var that starts empty
	orig := ActiveLang
	t.Cleanup(func() { ActiveLang = orig })
	ActiveLang = "de"
	assert.Equal(t, "de", ActiveLang)
}

func init() {
	// Override LANG/LC_ALL in case test environment has them set
	os.Unsetenv("LANG")
	os.Unsetenv("LC_ALL")
}
