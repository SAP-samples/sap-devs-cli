package i18n

import (
	"bytes"
	"embed"
	"encoding/json"
	"os"
	"strings"
	"text/template"
)

//go:embed catalogs/*.json
var catalogFS embed.FS

// ActiveLang is the resolved active language tag.
// Set once in rootCmd.PersistentPreRunE before any command body runs.
var ActiveLang string

// catalogs holds all loaded catalogs: map[lang]map[key]value.
var catalogs map[string]map[string]string

func init() {
	catalogs = make(map[string]map[string]string)
	entries, err := catalogFS.ReadDir("catalogs")
	if err != nil {
		panic("i18n: failed to read catalogs directory: " + err.Error())
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		lang := strings.TrimSuffix(e.Name(), ".json")
		data, err := catalogFS.ReadFile("catalogs/" + e.Name())
		if err != nil {
			panic("i18n: failed to read catalog " + e.Name() + ": " + err.Error())
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			panic("i18n: malformed catalog " + e.Name() + ": " + err.Error())
		}
		catalogs[lang] = m
	}
}

// Resolve returns the active language tag.
// Priority: cfgLang (if non-empty, used exclusively) → LANG env var →
// LC_ALL env var → "en". Empty env var values are treated as unset.
// All inputs are stripped of region/encoding suffixes: "de_AT.UTF-8" → "de".
// Unknown tags (no catalog) resolve to "en".
func Resolve(cfgLang string) string {
	if cfgLang != "" {
		return resolveTag(cfgLang)
	}
	for _, env := range []string{"LANG", "LC_ALL"} {
		if v := os.Getenv(env); v != "" {
			return resolveTag(v)
		}
	}
	return "en"
}

func resolveTag(s string) string {
	tag := stripTag(s)
	if _, ok := catalogs[tag]; ok {
		return tag
	}
	return "en"
}

// stripTag removes region and encoding suffixes: "de_AT.UTF-8" → "de".
func stripTag(s string) string {
	s = strings.ToLower(s)
	if i := strings.IndexAny(s, "_."); i > 0 {
		s = s[:i]
	}
	return s
}

// Lookup returns the translation for key in lang (falling back to en).
// Returns the string and true if found in either catalog; "", false if absent from both.
func Lookup(lang, key string) (string, bool) {
	if lang != "" && lang != "en" {
		if cat, ok := catalogs[lang]; ok {
			if v, ok := cat[key]; ok {
				return v, true
			}
		}
	}
	if cat, ok := catalogs["en"]; ok {
		if v, ok := cat[key]; ok {
			return v, true
		}
	}
	return "", false
}

// T returns the translation for key, falling back to the raw key string if not found.
func T(lang, key string) string {
	if v, ok := Lookup(lang, key); ok {
		return v
	}
	return key
}

// Tf resolves key via T and executes it as a text/template with data.
// Uses missingkey=error: missing keys in data trigger a failure.
// On any parse or execute failure, returns the raw (un-executed) catalog string.
// If data is nil, template actions will fail (missingkey=error), returning the raw string.
func Tf(lang, key string, data map[string]any) string {
	raw := T(lang, key)
	tmpl, err := template.New("").Option("missingkey=error").Parse(raw)
	if err != nil {
		return raw
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return raw
	}
	return buf.String()
}
