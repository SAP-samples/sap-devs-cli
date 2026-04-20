package schema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SAP-samples/sap-devs-cli/internal/schema"
)

// loadSchema is a helper that loads a named schema using the shared schemasDir().
func loadSchema(t *testing.T, name string) *schema.Schema {
	t.Helper()
	s, err := schema.Load(schemasDir(), name)
	require.NoError(t, err)
	return s
}

// TestValidate_ValidResource passes a well-formed resource and expects no errors.
func TestValidate_ValidResource(t *testing.T) {
	s := loadSchema(t, "resources")
	data := []any{
		map[string]any{
			"id":    "cap/docs-official",
			"title": "CAP Official Docs",
			"url":   "https://cap.cloud.sap/docs",
			"type":  "official-docs",
			"tags":  []any{"cap", "docs"},
		},
	}
	errs := schema.Validate(s, data)
	assert.Empty(t, errs, "valid resource should produce no errors")
}

// TestValidate_MissingRequired flags missing url, type, and tags.
func TestValidate_MissingRequired(t *testing.T) {
	s := loadSchema(t, "resources")
	data := []any{
		map[string]any{
			"id":    "cap/docs-official",
			"title": "CAP Official Docs",
			// url, type, tags missing
		},
	}
	errs := schema.Validate(s, data)
	fields := make([]string, 0, len(errs))
	for _, e := range errs {
		fields = append(fields, e.Field)
	}
	assert.Contains(t, fields, "url")
	assert.Contains(t, fields, "type")
	assert.Contains(t, fields, "tags")
}

// TestValidate_InvalidEnum rejects a "type" value that isn't in the enum.
func TestValidate_InvalidEnum(t *testing.T) {
	s := loadSchema(t, "resources")
	data := []any{
		map[string]any{
			"id":    "cap/docs-official",
			"title": "CAP Official Docs",
			"url":   "https://cap.cloud.sap/docs",
			"type":  "invalid-type",
			"tags":  []any{"cap"},
		},
	}
	errs := schema.Validate(s, data)
	require.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if e.Field == "type" {
			found = true
			assert.Equal(t, schema.SeverityError, e.Severity)
		}
	}
	assert.True(t, found, "expected error on type field")
}

// TestValidate_InvalidURI rejects "not-a-url" for a uri-format field.
func TestValidate_InvalidURI(t *testing.T) {
	s := loadSchema(t, "resources")
	data := []any{
		map[string]any{
			"id":    "cap/docs-official",
			"title": "CAP Official Docs",
			"url":   "not-a-url",
			"type":  "official-docs",
			"tags":  []any{"cap"},
		},
	}
	errs := schema.Validate(s, data)
	require.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if e.Field == "url" {
			found = true
		}
	}
	assert.True(t, found, "expected uri error on url field")
}

// TestValidate_InvalidPattern rejects an influencer id that doesn't match the pattern.
func TestValidate_InvalidPattern(t *testing.T) {
	s := loadSchema(t, "influencers")
	data := []any{
		map[string]any{
			"id":   "INVALID_ID", // uppercase and underscore violate pattern
			"name": "Test Person",
			"role": "Developer",
			"org":  "SAP",
			"focus": []any{"cap"},
			"links": map[string]any{
				"blog": "https://example.com",
			},
		},
	}
	errs := schema.Validate(s, data)
	require.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if e.Field == "id" {
			found = true
		}
	}
	assert.True(t, found, "expected pattern error on id field")
}

// TestValidate_NestedObject flags a missing required field inside the detect object.
func TestValidate_NestedObject(t *testing.T) {
	s := loadSchema(t, "tools")
	data := []any{
		map[string]any{
			"id":       "nodejs",
			"name":     "Node.js",
			"required": ">=18.0.0",
			"detect": map[string]any{
				"command": "node --version",
				// "pattern" missing
			},
			"install": map[string]any{},
			"docs":    "https://nodejs.org",
		},
	}
	errs := schema.Validate(s, data)
	require.NotEmpty(t, errs, "expected error for missing pattern in detect")
	found := false
	for _, e := range errs {
		if e.Field == "pattern" {
			found = true
		}
	}
	assert.True(t, found, "expected missing pattern error")
}

// TestValidate_MinItems rejects an influencer with an empty focus array.
func TestValidate_MinItems(t *testing.T) {
	s := loadSchema(t, "influencers")
	data := []any{
		map[string]any{
			"id":    "dj-adams",
			"name":  "DJ Adams",
			"role":  "Developer Advocate",
			"org":   "SAP",
			"focus": []any{}, // minItems:1
			"links": map[string]any{
				"blog": "https://qmacro.org",
			},
		},
	}
	errs := schema.Validate(s, data)
	require.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if e.Field == "focus" {
			found = true
		}
	}
	assert.True(t, found, "expected minItems error on focus")
}

// TestValidate_PackObject validates a well-formed pack.yaml object.
func TestValidate_PackObject(t *testing.T) {
	s := loadSchema(t, "pack")
	data := map[string]any{
		"id":          "cap",
		"name":        "CAP",
		"description": "SAP Cloud Application Programming Model",
		"tags":        []any{"cap", "node"},
	}
	errs := schema.Validate(s, data)
	assert.Empty(t, errs, "valid pack object should produce no errors: %v", errs)
}

// TestValidate_MapValues flags an invalid URI in an influencer's links map.
func TestValidate_MapValues(t *testing.T) {
	s := loadSchema(t, "influencers")
	data := []any{
		map[string]any{
			"id":    "dj-adams",
			"name":  "DJ Adams",
			"role":  "Developer Advocate",
			"org":   "SAP",
			"focus": []any{"cap"},
			"links": map[string]any{
				"blog": "not-a-valid-url", // invalid URI
			},
		},
	}
	errs := schema.Validate(s, data)
	require.NotEmpty(t, errs)
	found := false
	for _, e := range errs {
		if e.Field == "links[blog]" {
			found = true
		}
	}
	assert.True(t, found, "expected uri error on links[blog]")
}
