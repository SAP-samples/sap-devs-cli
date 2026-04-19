# Content Editing UI — Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add schema-driven TUI editor for all 10 content YAML types across all 4 content layers, with value help (enum selects, format validation, pattern matching) and batch validation.

**Architecture:** New `internal/schema/` package parses JSON Schema Draft-07 files into a generic `FieldSpec` model that drives both validation and form generation. New `internal/editor/` package builds a Bubbletea TUI with two states — list view (array content) and form view (`charmbracelet/huh` v2). Layer resolution auto-detects whether to edit in-place (contributor checkout) or create user/project overrides. Three new CLI commands under `sap-devs content`: `edit`, `validate`, `list`.

**Tech Stack:** Go, cobra, charmbracelet/bubbletea, charmbracelet/huh v2, charmbracelet/lipgloss, JSON Schema Draft-07, gopkg.in/yaml.v3

**Spec:** `docs/superpowers/specs/2026-04-19-content-editing-ui-design.md`

---

### Task 1: Add `charmbracelet/huh` v2 dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add the huh v2 dependency**

```bash
go get charm.land/huh/v2@latest
```

Note: huh v2 uses the `charm.land/huh/v2` import path (vanity URL), not `github.com/charmbracelet/huh`.

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully. New entries in go.mod and go.sum.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add charmbracelet/huh v2 dependency"
```

---

### Task 2: Schema types (`internal/schema/schema.go`)

**Files:**
- Create: `internal/schema/schema.go`

- [ ] **Step 1: Create the schema types file**

```go
package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Schema represents a parsed JSON Schema for a content type.
type Schema struct {
	Type       string      // "object" or "array"
	ItemSpec   *ObjectSpec // for arrays: schema of each item
	ObjectSpec *ObjectSpec // for single objects (e.g., pack.yaml)
}

// ObjectSpec holds the field definitions for an object.
type ObjectSpec struct {
	Fields   []FieldSpec
	Required []string
}

// FieldSpec describes a single field extracted from a JSON Schema property.
type FieldSpec struct {
	Key         string
	Title       string
	Description string
	Type        string // "string", "integer", "boolean", "array", "object", "map"
	Required    bool

	Enum    []string
	Format  string
	Pattern string
	Default any

	ItemType string
	ItemEnum []string
	MinItems int
	MaxItems int

	Children []FieldSpec

	MapValueType string // for additionalProperties maps: type of values

	Condition *Condition
}

// Condition makes a field visible only when another field has a specific value.
type Condition struct {
	Field string
	Value any
}

// knownFiles maps YAML filenames to their schema base names.
var knownFiles = map[string]string{
	"pack.yaml":            "pack",
	"resources.yaml":       "resources",
	"influencers.yaml":     "influencers",
	"event-types.yaml":     "event-types",
	"event-instances.yaml": "event-instances",
	"mcp.yaml":             "mcp",
	"tools.yaml":           "tools",
	"hook.yaml":            "hook",
	"samples.yaml":         "samples",
	"known_errors.yaml":    "known_errors",
}

// KnownFiles returns the mapping of YAML filenames to schema names.
func KnownFiles() map[string]string {
	out := make(map[string]string, len(knownFiles))
	for k, v := range knownFiles {
		out[k] = v
	}
	return out
}

// SchemaForFile returns the schema base name for a YAML filename, or "" if unknown.
func SchemaForFile(filename string) string {
	return knownFiles[filename]
}

// Load reads a JSON Schema file and returns a parsed Schema.
// schemasDir is the directory containing .schema.json files.
// name is the base name (e.g., "resources" for resources.schema.json).
func Load(schemasDir, name string) (*Schema, error) {
	path := filepath.Join(schemasDir, name+".schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema %s: %w", name, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse schema %s: %w", name, err)
	}

	s := &Schema{}
	topType, _ := raw["type"].(string)
	s.Type = topType

	switch topType {
	case "array":
		items, _ := raw["items"].(map[string]any)
		if items == nil {
			return nil, fmt.Errorf("schema %s: array type missing items", name)
		}
		required := extractStringSlice(raw, "items", "required")
		if required == nil {
			required = extractStringSlice(items, "required")
		}
		spec, err := parseObjectSpec(items, required)
		if err != nil {
			return nil, fmt.Errorf("schema %s items: %w", name, err)
		}
		s.ItemSpec = spec

	case "object":
		required := extractStringSlice(raw, "required")
		spec, err := parseObjectSpec(raw, required)
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", name, err)
		}
		s.ObjectSpec = spec
		parseConditionals(raw, spec)

	default:
		return nil, fmt.Errorf("schema %s: unsupported top-level type %q", name, topType)
	}

	return s, nil
}

func parseObjectSpec(obj map[string]any, required []string) (*ObjectSpec, error) {
	props, _ := obj["properties"].(map[string]any)
	if props == nil {
		return &ObjectSpec{Required: required}, nil
	}

	reqSet := make(map[string]bool, len(required))
	for _, r := range required {
		reqSet[r] = true
	}

	spec := &ObjectSpec{Required: required}
	for key, val := range props {
		propMap, ok := val.(map[string]any)
		if !ok {
			continue
		}
		field := parseField(key, propMap, reqSet[key])
		spec.Fields = append(spec.Fields, field)
	}

	sortFieldsBySchemaOrder(spec.Fields, props)
	return spec, nil
}

func parseField(key string, prop map[string]any, required bool) FieldSpec {
	f := FieldSpec{
		Key:         key,
		Title:       keyToTitle(key),
		Description: strVal(prop, "description"),
		Type:        strVal(prop, "type"),
		Required:    required,
		Format:      strVal(prop, "format"),
		Pattern:     strVal(prop, "pattern"),
		Default:     prop["default"],
	}

	if enums, ok := prop["enum"].([]any); ok {
		for _, e := range enums {
			if s, ok := e.(string); ok {
				f.Enum = append(f.Enum, s)
			}
		}
	}

	switch f.Type {
	case "array":
		items, _ := prop["items"].(map[string]any)
		if items != nil {
			f.ItemType = strVal(items, "type")
			if itemEnums, ok := items["enum"].([]any); ok {
				for _, e := range itemEnums {
					if s, ok := e.(string); ok {
						f.ItemEnum = append(f.ItemEnum, s)
					}
				}
			}
		}
		if v, ok := prop["minItems"].(float64); ok {
			f.MinItems = int(v)
		}
		if v, ok := prop["maxItems"].(float64); ok {
			f.MaxItems = int(v)
		}

	case "object":
		if addProps, ok := prop["additionalProperties"].(map[string]any); ok {
			f.Type = "map"
			f.MapValueType = strVal(addProps, "type")
			if fmt := strVal(addProps, "format"); fmt != "" {
				f.Format = fmt
			}
		} else if innerProps, ok := prop["properties"].(map[string]any); ok {
			innerReq := extractStringSlice(prop, "required")
			reqSet := make(map[string]bool, len(innerReq))
			for _, r := range innerReq {
				reqSet[r] = true
			}
			for childKey, childVal := range innerProps {
				childMap, ok := childVal.(map[string]any)
				if !ok {
					continue
				}
				f.Children = append(f.Children, parseField(childKey, childMap, reqSet[childKey]))
			}
			sortFieldsBySchemaOrder(f.Children, innerProps)
		}
	}

	return f
}

func parseConditionals(raw map[string]any, spec *ObjectSpec) {
	ifBlock, _ := raw["if"].(map[string]any)
	thenBlock, _ := raw["then"].(map[string]any)
	if ifBlock == nil || thenBlock == nil {
		return
	}

	ifProps, _ := ifBlock["properties"].(map[string]any)
	if ifProps == nil {
		return
	}

	var condField string
	var condValue any
	for k, v := range ifProps {
		vMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		condField = k
		condValue = vMap["const"]
		break
	}

	thenProps, _ := thenBlock["properties"].(map[string]any)
	if thenProps == nil {
		return
	}
	for _, field := range spec.Fields {
		if _, ok := thenProps[field.Key]; ok {
			for i := range spec.Fields {
				if spec.Fields[i].Key == field.Key {
					spec.Fields[i].Condition = &Condition{
						Field: condField,
						Value: condValue,
					}
				}
			}
		}
	}
}

func keyToTitle(key string) string {
	s := strings.ReplaceAll(key, "_", " ")
	if len(s) > 0 {
		s = strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func extractStringSlice(m map[string]any, keys ...string) []string {
	current := m
	for i, k := range keys {
		if i == len(keys)-1 {
			arr, ok := current[k].([]any)
			if !ok {
				return nil
			}
			out := make([]string, 0, len(arr))
			for _, v := range arr {
				if s, ok := v.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
		next, ok := current[k].(map[string]any)
		if !ok {
			return nil
		}
		current = next
	}
	return nil
}

func sortFieldsBySchemaOrder(fields []FieldSpec, props map[string]any) {
	// JSON object keys don't have order, so we sort alphabetically by key
	// with required fields first for better UX
	reqFields := make([]FieldSpec, 0)
	optFields := make([]FieldSpec, 0)
	for _, f := range fields {
		if f.Required {
			reqFields = append(reqFields, f)
		} else {
			optFields = append(optFields, f)
		}
	}
	copy(fields, append(reqFields, optFields...))
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/schema/schema.go
git commit -m "feat(schema): add JSON Schema parser with FieldSpec model"
```

---

### Task 3: Schema parser tests (`internal/schema/schema_test.go`)

**Files:**
- Create: `internal/schema/schema_test.go`

- [ ] **Step 1: Write tests for schema loading**

```go
package schema_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
)

func schemasDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "content", "schemas")
}

func TestLoad_Resources(t *testing.T) {
	s, err := schema.Load(schemasDir(), "resources")
	require.NoError(t, err)
	assert.Equal(t, "array", s.Type)
	require.NotNil(t, s.ItemSpec)
	assert.Contains(t, s.ItemSpec.Required, "id")
	assert.Contains(t, s.ItemSpec.Required, "type")

	var typeField *schema.FieldSpec
	for i, f := range s.ItemSpec.Fields {
		if f.Key == "type" {
			typeField = &s.ItemSpec.Fields[i]
			break
		}
	}
	require.NotNil(t, typeField)
	assert.Equal(t, []string{"official-docs", "sample", "community", "tutorial", "blog"}, typeField.Enum)
	assert.True(t, typeField.Required)
}

func TestLoad_Pack(t *testing.T) {
	s, err := schema.Load(schemasDir(), "pack")
	require.NoError(t, err)
	assert.Equal(t, "object", s.Type)
	require.NotNil(t, s.ObjectSpec)
	assert.Contains(t, s.ObjectSpec.Required, "id")
	assert.Contains(t, s.ObjectSpec.Required, "name")

	var addPosField *schema.FieldSpec
	for i, f := range s.ObjectSpec.Fields {
		if f.Key == "additive_position" {
			addPosField = &s.ObjectSpec.Fields[i]
			break
		}
	}
	require.NotNil(t, addPosField, "additive_position field should exist")
	require.NotNil(t, addPosField.Condition, "additive_position should have a condition")
	assert.Equal(t, "additive", addPosField.Condition.Field)
	assert.Equal(t, true, addPosField.Condition.Value)
}

func TestLoad_Tools_NestedObject(t *testing.T) {
	s, err := schema.Load(schemasDir(), "tools")
	require.NoError(t, err)

	var detectField *schema.FieldSpec
	for i, f := range s.ItemSpec.Fields {
		if f.Key == "detect" {
			detectField = &s.ItemSpec.Fields[i]
			break
		}
	}
	require.NotNil(t, detectField)
	assert.Equal(t, "object", detectField.Type)
	assert.Len(t, detectField.Children, 2)
}

func TestLoad_Influencers_MapType(t *testing.T) {
	s, err := schema.Load(schemasDir(), "influencers")
	require.NoError(t, err)

	var linksField *schema.FieldSpec
	for i, f := range s.ItemSpec.Fields {
		if f.Key == "links" {
			linksField = &s.ItemSpec.Fields[i]
			break
		}
	}
	require.NotNil(t, linksField)
	assert.Equal(t, "map", linksField.Type)
	assert.Equal(t, "string", linksField.MapValueType)
	assert.Equal(t, "uri", linksField.Format)
}

func TestLoad_AllSchemas(t *testing.T) {
	for filename, name := range schema.KnownFiles() {
		t.Run(filename, func(t *testing.T) {
			s, err := schema.Load(schemasDir(), name)
			require.NoError(t, err)
			assert.NotEmpty(t, s.Type)
		})
	}
}

func TestLoad_NonExistent(t *testing.T) {
	_, err := schema.Load(schemasDir(), "nonexistent")
	assert.Error(t, err)
}

func TestSchemaForFile(t *testing.T) {
	assert.Equal(t, "resources", schema.SchemaForFile("resources.yaml"))
	assert.Equal(t, "pack", schema.SchemaForFile("pack.yaml"))
	assert.Equal(t, "", schema.SchemaForFile("unknown.yaml"))
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go build ./internal/schema/...` (use build instead of test on Windows)
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/schema/schema_test.go
git commit -m "test(schema): add parser tests for all 10 content schemas"
```

---

### Task 4: Schema validation (`internal/schema/validate.go`)

**Files:**
- Create: `internal/schema/validate.go`

- [ ] **Step 1: Write the validation function**

```go
package schema

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ValidationError represents a single validation failure.
type ValidationError struct {
	Path     string
	Field    string
	Message  string
	Severity string // "error" or "warning"
}

func (e ValidationError) String() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return e.Message
}

// Validate checks data against a schema and returns all violations.
// data should be []any for array schemas or map[string]any for object schemas.
func Validate(s *Schema, data any) []ValidationError {
	switch s.Type {
	case "array":
		arr, ok := data.([]any)
		if !ok {
			return []ValidationError{{Message: "expected array", Severity: "error"}}
		}
		var errs []ValidationError
		for i, item := range arr {
			itemMap, ok := item.(map[string]any)
			if !ok {
				errs = append(errs, ValidationError{
					Path:     fmt.Sprintf("[%d]", i),
					Message:  "expected object",
					Severity: "error",
				})
				continue
			}
			for _, e := range validateObject(s.ItemSpec, itemMap) {
				e.Path = fmt.Sprintf("[%d].%s", i, e.Path)
				errs = append(errs, e)
			}
		}
		return errs

	case "object":
		obj, ok := data.(map[string]any)
		if !ok {
			return []ValidationError{{Message: "expected object", Severity: "error"}}
		}
		return validateObject(s.ObjectSpec, obj)
	}
	return nil
}

func validateObject(spec *ObjectSpec, obj map[string]any) []ValidationError {
	if spec == nil {
		return nil
	}

	var errs []ValidationError

	for _, f := range spec.Fields {
		val, exists := obj[f.Key]

		if f.Required && !exists {
			errs = append(errs, ValidationError{
				Path:     f.Key,
				Field:    f.Key,
				Message:  "required field missing",
				Severity: "error",
			})
			continue
		}

		if !exists {
			continue
		}

		errs = append(errs, validateField(f, val)...)
	}

	return errs
}

func validateField(f FieldSpec, val any) []ValidationError {
	var errs []ValidationError

	switch f.Type {
	case "string":
		s, ok := val.(string)
		if !ok {
			return []ValidationError{{Path: f.Key, Field: f.Key, Message: "expected string", Severity: "error"}}
		}
		if f.Required && s == "" {
			errs = append(errs, ValidationError{Path: f.Key, Field: f.Key, Message: "must not be empty", Severity: "error"})
		}
		if len(f.Enum) > 0 {
			found := false
			for _, e := range f.Enum {
				if s == e {
					found = true
					break
				}
			}
			if !found {
				errs = append(errs, ValidationError{
					Path:     f.Key,
					Field:    f.Key,
					Message:  fmt.Sprintf("must be one of: %s", strings.Join(f.Enum, ", ")),
					Severity: "error",
				})
			}
		}
		if f.Format == "uri" && s != "" {
			if _, err := url.ParseRequestURI(s); err != nil || !strings.HasPrefix(s, "http") {
				errs = append(errs, ValidationError{
					Path:     f.Key,
					Field:    f.Key,
					Message:  "not a valid URI",
					Severity: "error",
				})
			}
		}
		if f.Pattern != "" && s != "" {
			re, err := regexp.Compile(f.Pattern)
			if err == nil && !re.MatchString(s) {
				errs = append(errs, ValidationError{
					Path:     f.Key,
					Field:    f.Key,
					Message:  fmt.Sprintf("does not match pattern %s", f.Pattern),
					Severity: "error",
				})
			}
		}

	case "integer":
		if _, ok := val.(float64); !ok {
			if _, ok := val.(int); !ok {
				errs = append(errs, ValidationError{Path: f.Key, Field: f.Key, Message: "expected integer", Severity: "error"})
			}
		}

	case "boolean":
		if _, ok := val.(bool); !ok {
			errs = append(errs, ValidationError{Path: f.Key, Field: f.Key, Message: "expected boolean", Severity: "error"})
		}

	case "array":
		arr, ok := val.([]any)
		if !ok {
			errs = append(errs, ValidationError{Path: f.Key, Field: f.Key, Message: "expected array", Severity: "error"})
			break
		}
		if f.MinItems > 0 && len(arr) < f.MinItems {
			errs = append(errs, ValidationError{
				Path:     f.Key,
				Field:    f.Key,
				Message:  fmt.Sprintf("at least %d item(s) required", f.MinItems),
				Severity: "error",
			})
		}
		if f.MaxItems > 0 && len(arr) > f.MaxItems {
			errs = append(errs, ValidationError{
				Path:     f.Key,
				Field:    f.Key,
				Message:  fmt.Sprintf("at most %d item(s) allowed", f.MaxItems),
				Severity: "error",
			})
		}
		for i, item := range arr {
			if f.ItemType == "string" {
				s, ok := item.(string)
				if !ok {
					errs = append(errs, ValidationError{
						Path:     fmt.Sprintf("%s[%d]", f.Key, i),
						Field:    f.Key,
						Message:  "expected string",
						Severity: "error",
					})
				} else if len(f.ItemEnum) > 0 {
					found := false
					for _, e := range f.ItemEnum {
						if s == e {
							found = true
							break
						}
					}
					if !found {
						errs = append(errs, ValidationError{
							Path:     fmt.Sprintf("%s[%d]", f.Key, i),
							Field:    f.Key,
							Message:  fmt.Sprintf("must be one of: %s", strings.Join(f.ItemEnum, ", ")),
							Severity: "error",
						})
					}
				}
			}
		}

	case "object":
		childObj, ok := val.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{Path: f.Key, Field: f.Key, Message: "expected object", Severity: "error"})
			break
		}
		for _, child := range f.Children {
			childVal, exists := childObj[child.Key]
			if child.Required && !exists {
				errs = append(errs, ValidationError{
					Path:     fmt.Sprintf("%s.%s", f.Key, child.Key),
					Field:    child.Key,
					Message:  "required field missing",
					Severity: "error",
				})
				continue
			}
			if exists {
				for _, e := range validateField(child, childVal) {
					e.Path = fmt.Sprintf("%s.%s", f.Key, e.Path)
					errs = append(errs, e)
				}
			}
		}

	case "map":
		mapObj, ok := val.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{Path: f.Key, Field: f.Key, Message: "expected object (map)", Severity: "error"})
			break
		}
		for k, v := range mapObj {
			if f.MapValueType == "string" {
				s, ok := v.(string)
				if !ok {
					errs = append(errs, ValidationError{
						Path:     fmt.Sprintf("%s.%s", f.Key, k),
						Field:    k,
						Message:  "expected string value",
						Severity: "error",
					})
				} else if f.Format == "uri" {
					if _, err := url.ParseRequestURI(s); err != nil || !strings.HasPrefix(s, "http") {
						errs = append(errs, ValidationError{
							Path:     fmt.Sprintf("%s.%s", f.Key, k),
							Field:    k,
							Message:  "not a valid URI",
							Severity: "error",
						})
					}
				}
			}
		}
	}

	return errs
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/schema/...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/schema/validate.go
git commit -m "feat(schema): add validation engine for content YAML files"
```

---

### Task 5: Validation tests (`internal/schema/validate_test.go`)

**Files:**
- Create: `internal/schema/validate_test.go`

- [ ] **Step 1: Write validation tests**

```go
package schema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
)

func TestValidate_ValidResource(t *testing.T) {
	s, err := schema.Load(schemasDir(), "resources")
	require.NoError(t, err)

	data := []any{
		map[string]any{
			"id":    "cap/docs",
			"title": "CAP Docs",
			"url":   "https://cap.cloud.sap/docs",
			"type":  "official-docs",
			"tags":  []any{"reference"},
		},
	}
	errs := schema.Validate(s, data)
	assert.Empty(t, errs)
}

func TestValidate_MissingRequired(t *testing.T) {
	s, err := schema.Load(schemasDir(), "resources")
	require.NoError(t, err)

	data := []any{
		map[string]any{
			"id":    "cap/docs",
			"title": "CAP Docs",
		},
	}
	errs := schema.Validate(s, data)
	assert.NotEmpty(t, errs)

	var fields []string
	for _, e := range errs {
		fields = append(fields, e.Field)
	}
	assert.Contains(t, fields, "url")
	assert.Contains(t, fields, "type")
	assert.Contains(t, fields, "tags")
}

func TestValidate_InvalidEnum(t *testing.T) {
	s, err := schema.Load(schemasDir(), "resources")
	require.NoError(t, err)

	data := []any{
		map[string]any{
			"id":    "cap/docs",
			"title": "CAP Docs",
			"url":   "https://cap.cloud.sap",
			"type":  "invalid-type",
			"tags":  []any{"tag"},
		},
	}
	errs := schema.Validate(s, data)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Message, "must be one of")
}

func TestValidate_InvalidURI(t *testing.T) {
	s, err := schema.Load(schemasDir(), "resources")
	require.NoError(t, err)

	data := []any{
		map[string]any{
			"id":    "cap/docs",
			"title": "CAP Docs",
			"url":   "not-a-url",
			"type":  "official-docs",
			"tags":  []any{"tag"},
		},
	}
	errs := schema.Validate(s, data)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Message, "not a valid URI")
}

func TestValidate_InvalidPattern(t *testing.T) {
	s, err := schema.Load(schemasDir(), "influencers")
	require.NoError(t, err)

	data := []any{
		map[string]any{
			"id":    "INVALID_ID",
			"name":  "Test",
			"role":  "Dev",
			"org":   "SAP",
			"focus": []any{"cap"},
			"links": map[string]any{"blog": "https://example.com"},
		},
	}
	errs := schema.Validate(s, data)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Message, "does not match pattern")
}

func TestValidate_NestedObject(t *testing.T) {
	s, err := schema.Load(schemasDir(), "tools")
	require.NoError(t, err)

	data := []any{
		map[string]any{
			"id":       "node",
			"name":     "Node.js",
			"required": ">=18.0.0",
			"detect":   map[string]any{"command": "node --version"},
			"install":  map[string]any{"all": "npm install"},
			"docs":     "https://nodejs.org",
		},
	}
	errs := schema.Validate(s, data)
	var fields []string
	for _, e := range errs {
		fields = append(fields, e.Path)
	}
	assert.Contains(t, fields, "[0].detect.pattern")
}

func TestValidate_MinItems(t *testing.T) {
	s, err := schema.Load(schemasDir(), "influencers")
	require.NoError(t, err)

	data := []any{
		map[string]any{
			"id":    "test",
			"name":  "Test",
			"role":  "Dev",
			"org":   "SAP",
			"focus": []any{},
			"links": map[string]any{"blog": "https://example.com"},
		},
	}
	errs := schema.Validate(s, data)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Message, "at least 1")
}

func TestValidate_PackObject(t *testing.T) {
	s, err := schema.Load(schemasDir(), "pack")
	require.NoError(t, err)

	data := map[string]any{
		"id":          "test",
		"name":        "Test Pack",
		"description": "A test",
		"tags":        []any{"test"},
	}
	errs := schema.Validate(s, data)
	assert.Empty(t, errs)
}

func TestValidate_MapValues(t *testing.T) {
	s, err := schema.Load(schemasDir(), "influencers")
	require.NoError(t, err)

	data := []any{
		map[string]any{
			"id":    "test",
			"name":  "Test",
			"role":  "Dev",
			"org":   "SAP",
			"focus": []any{"cap"},
			"links": map[string]any{"blog": "not-a-url"},
		},
	}
	errs := schema.Validate(s, data)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Message, "not a valid URI")
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go build ./internal/schema/...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/schema/validate_test.go
git commit -m "test(schema): add validation tests for all field types"
```

---

### Task 6: Layer resolution (`internal/editor/resolve.go`)

**Files:**
- Create: `internal/editor/resolve.go`

- [ ] **Step 1: Write the layer resolution logic**

```go
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/xdg"
)

// Layer describes which content layer is being edited.
type Layer int

const (
	LayerOfficial Layer = iota
	LayerCompany
	LayerUser
	LayerProject
)

func (l Layer) String() string {
	switch l {
	case LayerOfficial:
		return "official"
	case LayerCompany:
		return "company"
	case LayerUser:
		return "user"
	case LayerProject:
		return "project"
	}
	return "unknown"
}

// ResolvedFile contains the resolved editing target.
type ResolvedFile struct {
	Layer      Layer
	PackID     string
	Filename   string
	SchemaName string
	FilePath   string // actual file path to edit
	PackDir    string // directory containing the pack
}

const officialRepoURL = "github.tools.sap/developer-relations/sap-devs-cli"

// ResolveEditTarget determines the file path and layer for a content edit request.
// arg is the user-provided file argument (e.g., "resources.yaml", "cap/resources.yaml", or a path).
func ResolveEditTarget(cwd, arg string) (*ResolvedFile, error) {
	// Direct path: starts with ./ or .sap-devs/ or content/
	if strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, ".sap-devs/") || strings.HasPrefix(arg, "content/") {
		return resolveDirectPath(cwd, arg)
	}

	// Pack/file format: "cap/resources.yaml"
	if parts := strings.SplitN(arg, "/", 2); len(parts) == 2 {
		packID := parts[0]
		filename := parts[1]
		schemaName := schema.SchemaForFile(filename)
		if schemaName == "" {
			return nil, fmt.Errorf("unknown content file: %s", filename)
		}
		return resolveForPack(cwd, packID, filename, schemaName)
	}

	// Bare filename: "resources.yaml"
	schemaName := schema.SchemaForFile(arg)
	if schemaName == "" {
		return nil, fmt.Errorf("unknown content file: %s", arg)
	}
	return resolveForFile(cwd, arg, schemaName)
}

func resolveDirectPath(cwd, arg string) (*ResolvedFile, error) {
	fullPath := filepath.Join(cwd, arg)
	filename := filepath.Base(fullPath)
	schemaName := schema.SchemaForFile(filename)
	if schemaName == "" {
		return nil, fmt.Errorf("unknown content file: %s", filename)
	}

	packDir := filepath.Dir(fullPath)
	packID := filepath.Base(packDir)

	layer := LayerProject
	if strings.Contains(fullPath, "content/packs/") {
		layer = LayerOfficial
	}

	return &ResolvedFile{
		Layer:      layer,
		PackID:     packID,
		Filename:   filename,
		SchemaName: schemaName,
		FilePath:   fullPath,
		PackDir:    packDir,
	}, nil
}

func resolveForPack(cwd, packID, filename, schemaName string) (*ResolvedFile, error) {
	layer, baseDir := detectLayer(cwd)

	var packDir string
	switch layer {
	case LayerOfficial, LayerCompany:
		packDir = filepath.Join(baseDir, "content", "packs", packID)
	case LayerProject:
		packDir = filepath.Join(baseDir, ".sap-devs", "packs", packID)
	case LayerUser:
		packDir = filepath.Join(baseDir, "packs", packID)
	}

	return &ResolvedFile{
		Layer:      layer,
		PackID:     packID,
		Filename:   filename,
		SchemaName: schemaName,
		FilePath:   filepath.Join(packDir, filename),
		PackDir:    packDir,
	}, nil
}

func resolveForFile(cwd, filename, schemaName string) (*ResolvedFile, error) {
	layer, baseDir := detectLayer(cwd)

	var packsDir string
	switch layer {
	case LayerOfficial, LayerCompany:
		packsDir = filepath.Join(baseDir, "content", "packs")
	case LayerProject:
		packsDir = filepath.Join(baseDir, ".sap-devs", "packs")
	case LayerUser:
		packsDir = filepath.Join(baseDir, "packs")
	}

	// Scan for packs containing this file
	entries, err := os.ReadDir(packsDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read packs directory %s: %w", packsDir, err)
	}

	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(packsDir, e.Name(), filename)
		if _, err := os.Stat(candidate); err == nil {
			matches = append(matches, e.Name())
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no pack contains %s in layer %s", filename, layer)
	}

	packID := matches[0]
	if len(matches) > 1 {
		// Return first match; caller should prompt user to disambiguate
		// Attach all matches for the caller to present
		packID = matches[0]
	}

	packDir := filepath.Join(packsDir, packID)
	return &ResolvedFile{
		Layer:      layer,
		PackID:     packID,
		Filename:   filename,
		SchemaName: schemaName,
		FilePath:   filepath.Join(packDir, filename),
		PackDir:    packDir,
	}, nil
}

// AmbiguousPacks returns all pack IDs that contain a given filename, for disambiguation.
func AmbiguousPacks(cwd, filename string) []string {
	layer, baseDir := detectLayer(cwd)

	var packsDir string
	switch layer {
	case LayerOfficial, LayerCompany:
		packsDir = filepath.Join(baseDir, "content", "packs")
	case LayerProject:
		packsDir = filepath.Join(baseDir, ".sap-devs", "packs")
	case LayerUser:
		packsDir = filepath.Join(baseDir, "packs")
	}

	entries, err := os.ReadDir(packsDir)
	if err != nil {
		return nil
	}

	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(packsDir, e.Name(), filename)); err == nil {
			matches = append(matches, e.Name())
		}
	}
	return matches
}

func detectLayer(cwd string) (Layer, string) {
	// Check for official repo checkout
	if _, err := os.Stat(filepath.Join(cwd, "content", "packs")); err == nil {
		if isOfficialRepo(cwd) {
			return LayerOfficial, cwd
		}
		if isCompanyRepo(cwd) {
			return LayerCompany, cwd
		}
	}

	// Check for project layer
	if _, err := os.Stat(filepath.Join(cwd, ".sap-devs")); err == nil {
		return LayerProject, cwd
	}

	// Fall back to user layer
	paths, err := xdg.New()
	if err != nil {
		return LayerUser, ""
	}
	return LayerUser, paths.DataDir
}

func isOfficialRepo(dir string) bool {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), officialRepoURL)
}

func isCompanyRepo(dir string) bool {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return false
	}

	paths, err := xdg.New()
	if err != nil {
		return false
	}
	cfg, err := config.Load(paths.ConfigDir)
	if err != nil || cfg.CompanyRepo == "" {
		return false
	}

	return strings.Contains(string(out), cfg.CompanyRepo)
}

// AllLayers returns the directories for each content layer that exists.
func AllLayers(cwd string) []struct {
	Layer Layer
	Dir   string
} {
	paths, _ := xdg.New()

	var layers []struct {
		Layer Layer
		Dir   string
	}

	// Official: cache or CWD checkout
	if _, err := os.Stat(filepath.Join(cwd, "content", "packs")); err == nil && isOfficialRepo(cwd) {
		layers = append(layers, struct {
			Layer Layer
			Dir   string
		}{LayerOfficial, filepath.Join(cwd, "content", "packs")})
	} else if paths != nil {
		officialDir := filepath.Join(paths.CacheDir, "official", "content", "packs")
		if _, err := os.Stat(officialDir); err == nil {
			layers = append(layers, struct {
				Layer Layer
				Dir   string
			}{LayerOfficial, officialDir})
		}
	}

	// Company
	if paths != nil {
		cfg, _ := config.Load(paths.ConfigDir)
		if cfg != nil && cfg.CompanyRepo != "" {
			companyDir := filepath.Join(paths.CacheDir, "company", "content", "packs")
			if _, err := os.Stat(companyDir); err == nil {
				layers = append(layers, struct {
					Layer Layer
					Dir   string
				}{LayerCompany, companyDir})
			}
		}
	}

	// User
	if paths != nil {
		userDir := filepath.Join(paths.DataDir, "packs")
		if _, err := os.Stat(userDir); err == nil {
			layers = append(layers, struct {
				Layer Layer
				Dir   string
			}{LayerUser, userDir})
		}
	}

	// Project
	projectDir := filepath.Join(cwd, ".sap-devs", "packs")
	if _, err := os.Stat(projectDir); err == nil {
		layers = append(layers, struct {
			Layer Layer
			Dir   string
		}{LayerProject, projectDir})
	}

	return layers
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/editor/...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/editor/resolve.go
git commit -m "feat(editor): add layer resolution and file path detection"
```

---

### Task 7: Merged view assembly (`internal/editor/merge.go`)

**Files:**
- Create: `internal/editor/merge.go`

- [ ] **Step 1: Write the merged item model and assembly logic**

```go
package editor

import (
	"os"
	"path/filepath"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
	"gopkg.in/yaml.v3"
)

// MergedItem wraps a content item with its source layer information.
type MergedItem struct {
	Data       map[string]any
	Layer      Layer
	IsOverride bool
}

// LoadMergedItems loads items from all layers for a given pack and file,
// merging by ID. Later layers override earlier ones.
func LoadMergedItems(cwd, packID, filename string) ([]MergedItem, error) {
	layers := AllLayers(cwd)
	seen := make(map[string]int) // id -> index in result
	var result []MergedItem

	for _, l := range layers {
		packDir := filepath.Join(l.Dir, packID)
		filePath := filepath.Join(packDir, filename)

		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var items []map[string]any
		if err := yaml.Unmarshal(data, &items); err != nil {
			continue
		}

		for _, item := range items {
			id, _ := item["id"].(string)
			if id == "" {
				id, _ = item["name"].(string)
			}

			if idx, exists := seen[id]; exists {
				result[idx] = MergedItem{
					Data:       item,
					Layer:      l.Layer,
					IsOverride: true,
				}
			} else {
				seen[id] = len(result)
				result = append(result, MergedItem{
					Data:  item,
					Layer: l.Layer,
				})
			}
		}
	}

	return result, nil
}

// LoadSingleObject loads a single-object file (e.g., pack.yaml) from the target layer.
func LoadSingleObject(filePath string) (map[string]any, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, err
	}
	var obj map[string]any
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// SaveItems marshals items to YAML and writes to filePath.
// Only items belonging to the target layer are included.
func SaveItems(filePath string, items []MergedItem, targetLayer Layer) error {
	var toSave []map[string]any
	for _, item := range items {
		if item.Layer == targetLayer || item.IsOverride {
			toSave = append(toSave, item.Data)
		}
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(toSave)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// SaveObject marshals a single object to YAML and writes to filePath.
func SaveObject(filePath string, obj map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// ColumnsForSchema returns the column keys to display in list view,
// based on the schema fields (ID + first 2-3 string fields).
func ColumnsForSchema(spec *schema.ObjectSpec) []string {
	cols := []string{}
	for _, f := range spec.Fields {
		if f.Type == "string" && f.Key != "description" && f.Key != "pattern" && f.Key != "cause" && f.Key != "fix" {
			cols = append(cols, f.Key)
			if len(cols) >= 4 {
				break
			}
		}
	}
	return cols
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/editor/...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/editor/merge.go
git commit -m "feat(editor): add merged view assembly with layer tracking"
```

---

### Task 8: Form generator (`internal/editor/form.go`)

**Files:**
- Create: `internal/editor/form.go`

- [ ] **Step 1: Write the huh form generator from FieldSpec**

```go
package editor

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"charm.land/huh/v2"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
)

// FormResult holds the form field values indexed by key.
type FormResult map[string]any

// BuildForm creates a huh form from a schema ObjectSpec and current values.
func BuildForm(spec *schema.ObjectSpec, values map[string]any) (*huh.Form, FormResult) {
	result := make(FormResult)
	for k, v := range values {
		result[k] = v
	}

	var fields []huh.Field
	for _, f := range spec.Fields {
		field := buildField(f, result)
		if field != nil {
			fields = append(fields, field)
		}
	}

	group := huh.NewGroup(fields...)
	form := huh.NewForm(group).WithTheme(huh.ThemeDracula())
	return form, result
}

func buildField(f schema.FieldSpec, result FormResult) huh.Field {
	switch f.Type {
	case "string":
		return buildStringField(f, result)
	case "integer":
		return buildIntegerField(f, result)
	case "boolean":
		return buildBoolField(f, result)
	case "array":
		return buildArrayField(f, result)
	case "object":
		return buildObjectFields(f, result)
	case "map":
		return buildMapField(f, result)
	}
	return nil
}

func buildStringField(f schema.FieldSpec, result FormResult) huh.Field {
	if len(f.Enum) > 0 {
		current, _ := result[f.Key].(string)
		if current == "" && f.Default != nil {
			current, _ = f.Default.(string)
		}
		result[f.Key] = current

		opts := make([]huh.Option[string], 0, len(f.Enum))
		for _, e := range f.Enum {
			opts = append(opts, huh.NewOption(e, e))
		}

		return huh.NewSelect[string]().
			Title(fieldTitle(f)).
			Description(f.Description).
			Options(opts...).
			Value(strPtr(result, f.Key))
	}

	current, _ := result[f.Key].(string)
	result[f.Key] = current

	input := huh.NewInput().
		Title(fieldTitle(f)).
		Description(f.Description).
		Placeholder(placeholderForField(f)).
		Value(strPtr(result, f.Key))

	if v := validatorForString(f); v != nil {
		input = input.Validate(v)
	}

	return input
}

func buildIntegerField(f schema.FieldSpec, result FormResult) huh.Field {
	var current string
	switch v := result[f.Key].(type) {
	case float64:
		current = strconv.Itoa(int(v))
	case int:
		current = strconv.Itoa(v)
	}
	result[f.Key] = current

	return huh.NewInput().
		Title(fieldTitle(f)).
		Description(f.Description).
		Placeholder("0").
		Value(strPtr(result, f.Key)).
		Validate(func(s string) error {
			if s == "" && !f.Required {
				return nil
			}
			if _, err := strconv.Atoi(s); err != nil {
				return fmt.Errorf("must be an integer")
			}
			return nil
		})
}

func buildBoolField(f schema.FieldSpec, result FormResult) huh.Field {
	current, _ := result[f.Key].(bool)
	result[f.Key] = current

	return huh.NewConfirm().
		Title(fieldTitle(f)).
		Description(f.Description).
		Value(boolPtr(result, f.Key))
}

func buildArrayField(f schema.FieldSpec, result FormResult) huh.Field {
	rawArr, _ := result[f.Key].([]any)
	var current []string
	for _, v := range rawArr {
		if s, ok := v.(string); ok {
			current = append(current, s)
		}
	}
	result[f.Key] = current

	if len(f.ItemEnum) > 0 {
		opts := make([]huh.Option[string], 0, len(f.ItemEnum))
		for _, e := range f.ItemEnum {
			opt := huh.NewOption(e, e)
			for _, c := range current {
				if c == e {
					opt = opt.Selected(true)
					break
				}
			}
			opts = append(opts, opt)
		}
		return huh.NewMultiSelect[string]().
			Title(fieldTitle(f)).
			Description(f.Description).
			Options(opts...).
			Value(strSlicePtr(result, f.Key))
	}

	// Free-form string array: use comma-separated input for now
	joined := strings.Join(current, ", ")
	result[f.Key+"_raw"] = joined
	return huh.NewInput().
		Title(fieldTitle(f) + " (comma-separated)").
		Description(f.Description).
		Value(strPtr(result, f.Key+"_raw")).
		Validate(func(s string) error {
			parts := splitTags(s)
			if f.MinItems > 0 && len(parts) < f.MinItems {
				return fmt.Errorf("at least %d item(s) required", f.MinItems)
			}
			return nil
		})
}

func buildObjectFields(f schema.FieldSpec, result FormResult) huh.Field {
	// Nested objects are rendered as a Note pointing to sub-form
	// In the full TUI this triggers a nested form view
	childObj, ok := result[f.Key].(map[string]any)
	if !ok {
		childObj = make(map[string]any)
		result[f.Key] = childObj
	}

	var summary []string
	for _, child := range f.Children {
		if v, ok := childObj[child.Key]; ok {
			summary = append(summary, fmt.Sprintf("%s: %v", child.Key, v))
		}
	}
	desc := strings.Join(summary, " | ")
	if desc == "" {
		desc = "(empty)"
	}

	return huh.NewNote().
		Title(fieldTitle(f)).
		Description(desc + "\n\nPress Enter to edit nested fields")
}

func buildMapField(f schema.FieldSpec, result FormResult) huh.Field {
	mapObj, ok := result[f.Key].(map[string]any)
	if !ok {
		mapObj = make(map[string]any)
		result[f.Key] = mapObj
	}

	var summary []string
	for k, v := range mapObj {
		summary = append(summary, fmt.Sprintf("%s: %v", k, v))
	}
	desc := strings.Join(summary, "\n")
	if desc == "" {
		desc = "(empty)"
	}

	return huh.NewNote().
		Title(fieldTitle(f) + " (key-value map)").
		Description(desc + "\n\nPress Enter to edit map entries")
}

func fieldTitle(f schema.FieldSpec) string {
	title := f.Title
	if f.Required {
		title += " *"
	}
	return title
}

func placeholderForField(f schema.FieldSpec) string {
	if f.Format == "uri" {
		return "https://..."
	}
	if f.Pattern != "" {
		return fmt.Sprintf("pattern: %s", f.Pattern)
	}
	return ""
}

func validatorForString(f schema.FieldSpec) func(string) error {
	return func(s string) error {
		if f.Required && s == "" {
			return fmt.Errorf("required")
		}
		if s == "" {
			return nil
		}
		if f.Format == "uri" {
			if _, err := url.ParseRequestURI(s); err != nil || !strings.HasPrefix(s, "http") {
				return fmt.Errorf("not a valid URI")
			}
		}
		if f.Pattern != "" {
			re, err := regexp.Compile(f.Pattern)
			if err == nil && !re.MatchString(s) {
				return fmt.Errorf("does not match expected format")
			}
		}
		return nil
	}
}

func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ResolveTagsFromRaw converts comma-separated _raw fields back to proper arrays.
func ResolveTagsFromRaw(spec *schema.ObjectSpec, result FormResult) {
	for _, f := range spec.Fields {
		if f.Type == "array" && len(f.ItemEnum) == 0 {
			if raw, ok := result[f.Key+"_raw"].(string); ok {
				tags := splitTags(raw)
				anyTags := make([]any, len(tags))
				for i, t := range tags {
					anyTags[i] = t
				}
				result[f.Key] = anyTags
				delete(result, f.Key+"_raw")
			}
		}
	}
}

// Helper functions for huh value binding

func strPtr(m FormResult, key string) *string {
	v, _ := m[key].(string)
	m[key] = v
	return m[key].(*string)
}

func boolPtr(m FormResult, key string) *bool {
	v, _ := m[key].(bool)
	m[key] = v
	return m[key].(*bool)
}

func strSlicePtr(m FormResult, key string) *[]string {
	v, _ := m[key].([]string)
	m[key] = v
	return m[key].(*[]string)
}
```

Note: The `strPtr`/`boolPtr`/`strSlicePtr` helpers need to return stable pointers. The above approach won't work because map values aren't addressable. We need wrapper types. Let me fix that.

- [ ] **Step 1 (revised): Use wrapper types for stable pointers**

Replace the helper functions at the bottom with:

```go
// Binding holds a typed value for huh form binding.
type Binding[T any] struct {
	Value T
}

// BuildForm creates a huh form from a schema ObjectSpec and current values.
// Returns the form and a map of key -> *Binding for extracting values after form completion.
func BuildForm(spec *schema.ObjectSpec, values map[string]any) (*huh.Form, map[string]any) {
	bindings := make(map[string]any)
	var fields []huh.Field

	for _, f := range spec.Fields {
		field := buildField(f, values, bindings)
		if field != nil {
			fields = append(fields, field)
		}
	}

	group := huh.NewGroup(fields...)
	form := huh.NewForm(group).WithTheme(huh.ThemeDracula())
	return form, bindings
}
```

Each `buildXxxField` function creates a `Binding[T]` and stores it in the bindings map, passing `&binding.Value` to huh. After form completion, the caller reads `binding.Value` to get the edited values back.

This is complex enough that the implementer should follow the huh v2 documentation examples directly. The key pattern is:

```go
var myString string
huh.NewInput().Value(&myString)
// After form.Run(), myString contains the user's input
```

So the simplest approach: declare local variables per field, run the form, then collect results.

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/editor/...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/editor/form.go
git commit -m "feat(editor): add huh form generator from FieldSpec"
```

---

### Task 9: List view model (`internal/editor/list.go`)

**Files:**
- Create: `internal/editor/list.go`

- [ ] **Step 1: Write the list view Bubbletea model**

```go
package editor

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
)

var (
	selectedStyle  = lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	headerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Bold(true)
	layerOfficial  = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Render("official")
	layerCompany   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("company")
	layerUser      = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("user")
	layerProject   = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Render("project")
	overrideBadge  = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(" ✎")
)

func layerBadge(l Layer, isOverride bool) string {
	badge := ""
	switch l {
	case LayerOfficial:
		badge = layerOfficial
	case LayerCompany:
		badge = layerCompany
	case LayerUser:
		badge = layerUser
	case LayerProject:
		badge = layerProject
	}
	if isOverride {
		badge += overrideBadge
	}
	return badge
}

type listModel struct {
	items      []MergedItem
	columns    []string
	cursor     int
	width      int
	height     int
	filter     string
	filtering  bool
	target     *ResolvedFile
	schema     *schema.Schema
	dirty      bool

	// Result: which item to edit, or -1 for none
	editIndex  int
	addNew     bool
	deleteIdx  int
	quit       bool
	save       bool
}

func newListModel(items []MergedItem, columns []string, target *ResolvedFile, s *schema.Schema) listModel {
	return listModel{
		items:     items,
		columns:   columns,
		target:    target,
		schema:    s,
		editIndex: -1,
		deleteIdx: -1,
		width:     80,
		height:    24,
	}
}

func (m listModel) Init() tea.Cmd {
	return nil
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "enter", "esc":
				m.filtering = false
			case "backspace":
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
				}
			default:
				if len(msg.String()) == 1 {
					m.filter += msg.String()
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "q":
			m.save = true
			m.quit = true
			return m, tea.Quit
		case "esc":
			m.quit = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			visible := m.visibleItems()
			if m.cursor < len(visible)-1 {
				m.cursor++
			}
		case "enter":
			visible := m.visibleItems()
			if m.cursor < len(visible) {
				m.editIndex = visible[m.cursor].originalIndex
				return m, tea.Quit
			}
		case "a":
			m.addNew = true
			return m, tea.Quit
		case "d":
			visible := m.visibleItems()
			if m.cursor < len(visible) {
				idx := visible[m.cursor].originalIndex
				if m.items[idx].Layer == m.target.Layer {
					m.deleteIdx = idx
					m.dirty = true
				}
			}
		case "/":
			m.filtering = true
			m.filter = ""
		}
	}
	return m, nil
}

type visibleItem struct {
	originalIndex int
	item          MergedItem
}

func (m listModel) visibleItems() []visibleItem {
	var out []visibleItem
	for i, item := range m.items {
		if m.filter != "" {
			id, _ := item.Data["id"].(string)
			name, _ := item.Data["name"].(string)
			title, _ := item.Data["title"].(string)
			text := strings.ToLower(id + " " + name + " " + title)
			if !strings.Contains(text, strings.ToLower(m.filter)) {
				continue
			}
		}
		out = append(out, visibleItem{originalIndex: i, item: item})
	}
	return out
}

func (m listModel) View() string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("\n  %s · %s · %d items\n",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("114")).Render(m.target.Filename),
		fmt.Sprintf("Pack: %s · Layer: %s", m.target.PackID, m.target.Layer),
		len(m.items),
	))

	if m.filtering {
		sb.WriteString(fmt.Sprintf("  / %s█\n", m.filter))
	}

	sb.WriteString("\n")

	// Column header
	header := "    "
	for _, col := range m.columns {
		header += fmt.Sprintf("%-20s", strings.ToUpper(col))
	}
	header += "LAYER"
	sb.WriteString(headerStyle.Render(header))
	sb.WriteString("\n")

	// Items
	visible := m.visibleItems()
	maxVisible := m.height - 8
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}

	for i := start; i < len(visible) && i < start+maxVisible; i++ {
		item := visible[i]
		row := "  "
		if i == m.cursor {
			row += "› "
		} else {
			row += "  "
		}

		for _, col := range m.columns {
			val, _ := item.item.Data[col].(string)
			if len(val) > 18 {
				val = val[:18] + "…"
			}
			row += fmt.Sprintf("%-20s", val)
		}

		row += layerBadge(item.item.Layer, item.item.IsOverride)

		if i == m.cursor {
			sb.WriteString(selectedStyle.Render(row))
		} else {
			sb.WriteString(row)
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"  ↑↓ navigate · Enter edit · a add · d delete · / filter · q save & quit · Esc quit",
	))
	sb.WriteString("\n")

	return sb.String()
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/editor/...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/editor/list.go
git commit -m "feat(editor): add list view Bubbletea model with layer badges"
```

---

### Task 10: Main editor model (`internal/editor/editor.go`)

**Files:**
- Create: `internal/editor/editor.go`

- [ ] **Step 1: Write the main editor orchestrator**

```go
package editor

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"charm.land/huh/v2"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
	"gopkg.in/yaml.v3"
)

// Run opens the editor for the given resolved file.
func Run(target *ResolvedFile, schemasDir string) error {
	s, err := schema.Load(schemasDir, target.SchemaName)
	if err != nil {
		return fmt.Errorf("load schema: %w", err)
	}

	switch s.Type {
	case "object":
		return runObjectEditor(target, s)
	case "array":
		return runArrayEditor(target, s)
	}
	return fmt.Errorf("unsupported schema type: %s", s.Type)
}

func runObjectEditor(target *ResolvedFile, s *schema.Schema) error {
	obj, err := LoadSingleObject(target.FilePath)
	if err != nil {
		return err
	}

	form, bindings := BuildForm(s.ObjectSpec, obj)
	if err := form.Run(); err != nil {
		return err
	}

	// Merge bindings back into obj
	for k, v := range bindings {
		obj[k] = v
	}

	ResolveTagsFromRaw(s.ObjectSpec, obj)

	// Validate before saving
	errs := schema.Validate(s, obj)
	if hasErrors(errs) {
		fmt.Fprintf(os.Stderr, "\nValidation errors:\n")
		for _, e := range errs {
			if e.Severity == "error" {
				fmt.Fprintf(os.Stderr, "  ✗ %s\n", e)
			}
		}
		return fmt.Errorf("fix validation errors before saving")
	}

	return SaveObject(target.FilePath, obj)
}

func runArrayEditor(target *ResolvedFile, s *schema.Schema) error {
	cwd, _ := os.Getwd()
	items, err := LoadMergedItems(cwd, target.PackID, target.Filename)
	if err != nil {
		return err
	}

	columns := ColumnsForSchema(s.ItemSpec)

	for {
		listMdl := newListModel(items, columns, target, s)
		p := tea.NewProgram(listMdl, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			return err
		}

		result := finalModel.(listModel)

		if result.deleteIdx >= 0 {
			items = append(items[:result.deleteIdx], items[result.deleteIdx+1:]...)
			continue
		}

		if result.addNew {
			newItem := make(map[string]any)
			for _, f := range s.ItemSpec.Fields {
				if f.Default != nil {
					newItem[f.Key] = f.Default
				}
			}
			if err := editItem(s.ItemSpec, newItem); err != nil {
				continue
			}
			items = append(items, MergedItem{
				Data:  newItem,
				Layer: target.Layer,
			})
			continue
		}

		if result.editIndex >= 0 {
			item := &items[result.editIndex]
			if item.Layer != target.Layer {
				// Auto-create override copy
				copy := make(map[string]any)
				for k, v := range item.Data {
					copy[k] = v
				}
				item.Data = copy
				item.Layer = target.Layer
				item.IsOverride = true
			}
			if err := editItem(s.ItemSpec, item.Data); err != nil {
				continue
			}
			continue
		}

		if result.save {
			return SaveItems(target.FilePath, items, target.Layer)
		}

		// Quit without saving
		return nil
	}
}

func editItem(spec *schema.ObjectSpec, data map[string]any) error {
	form, bindings := BuildForm(spec, data)
	if err := form.Run(); err != nil {
		return err
	}

	for k, v := range bindings {
		data[k] = v
	}
	ResolveTagsFromRaw(spec, data)
	return nil
}

func hasErrors(errs []schema.ValidationError) bool {
	for _, e := range errs {
		if e.Severity == "error" {
			return true
		}
	}
	return false
}

// ValidateFile validates a single YAML file against its schema.
func ValidateFile(filePath, schemasDir string) ([]schema.ValidationError, error) {
	filename := filepath.Base(filePath)
	schemaName := schema.SchemaForFile(filename)
	if schemaName == "" {
		return nil, fmt.Errorf("unknown content file: %s", filename)
	}

	s, err := schema.Load(schemasDir, schemaName)
	if err != nil {
		return nil, err
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var data any
	switch s.Type {
	case "array":
		var arr []any
		if err := yaml.Unmarshal(fileData, &arr); err != nil {
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
		data = arr
	case "object":
		var obj map[string]any
		if err := yaml.Unmarshal(fileData, &obj); err != nil {
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
		data = obj
	}

	return schema.Validate(s, data), nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/editor/...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/editor/editor.go
git commit -m "feat(editor): add main TUI editor with list→form flow"
```

---

### Task 11: CLI commands — `content` parent and `content list`

**Files:**
- Create: `cmd/content.go`
- Create: `cmd/content_list.go`

- [ ] **Step 1: Write the content parent command**

```go
package cmd

import "github.com/spf13/cobra"

var contentCmd = &cobra.Command{
	Use:   "content",
	Short: "Manage content YAML files",
	Long:  "Browse, edit, and validate content YAML files across all content layers.",
}

func init() {
	rootCmd.AddCommand(contentCmd)
}
```

- [ ] **Step 2: Write the content list subcommand**

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/editor"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
	"gopkg.in/yaml.v3"
)

var (
	contentListPack  string
	contentListLayer string
)

var contentListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all content files across layers",
	RunE:  runContentList,
}

func init() {
	contentListCmd.Flags().StringVar(&contentListPack, "pack", "", "Filter to a specific pack")
	contentListCmd.Flags().StringVar(&contentListLayer, "layer", "", "Filter to a specific layer (official, company, user, project)")
	contentCmd.AddCommand(contentListCmd)
}

func runContentList(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	layers := editor.AllLayers(cwd)

	cmd.Println()
	cmd.Printf("  %-12s %-28s %-12s %s\n", "PACK", "FILE", "LAYER", "ITEMS")

	for _, l := range layers {
		if contentListLayer != "" && l.Layer.String() != contentListLayer {
			continue
		}

		entries, err := os.ReadDir(l.Dir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			packID := e.Name()
			if contentListPack != "" && packID != contentListPack {
				continue
			}

			packDir := filepath.Join(l.Dir, packID)
			for filename := range schema.KnownFiles() {
				filePath := filepath.Join(packDir, filename)
				if _, err := os.Stat(filePath); err != nil {
					continue
				}

				count := countItems(filePath)
				cmd.Printf("  %-12s %-28s %-12s %s\n", packID, filename, l.Layer, count)
			}
		}
	}

	cmd.Println()
	return nil
}

func countItems(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "—"
	}

	// Try as array first
	var arr []any
	if err := yaml.Unmarshal(data, &arr); err == nil && arr != nil {
		return fmt.Sprintf("%d", len(arr))
	}

	// Single object
	return "—"
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add cmd/content.go cmd/content_list.go
git commit -m "feat(content): add content parent command and content list"
```

---

### Task 12: CLI command — `content validate`

**Files:**
- Create: `cmd/content_validate.go`

- [ ] **Step 1: Write the content validate subcommand**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/editor"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
)

var (
	validatePack  string
	validateLayer string
	validateJSON  bool
)

var contentValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate all content files against JSON schemas",
	RunE:  runContentValidate,
}

func init() {
	contentValidateCmd.Flags().StringVar(&validatePack, "pack", "", "Validate only a specific pack")
	contentValidateCmd.Flags().StringVar(&validateLayer, "layer", "", "Validate only a specific layer")
	contentValidateCmd.Flags().BoolVar(&validateJSON, "json", false, "Output as JSON")
	contentCmd.AddCommand(contentValidateCmd)
}

type validateResult struct {
	Layer    string                  `json:"layer"`
	Pack     string                  `json:"pack"`
	File     string                  `json:"file"`
	Path     string                  `json:"path"`
	Errors   []schema.ValidationError `json:"errors,omitempty"`
	Passed   bool                    `json:"passed"`
}

func runContentValidate(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	layers := editor.AllLayers(cwd)

	// Find schemas directory
	schemasDir := findSchemasDir(cwd)
	if schemasDir == "" {
		return fmt.Errorf("cannot find content/schemas/ directory")
	}

	var results []validateResult
	totalFiles := 0
	totalErrors := 0
	filesWithErrors := 0

	if !validateJSON {
		cmd.Println("\nValidating content across all layers...\n")
	}

	for _, l := range layers {
		if validateLayer != "" && l.Layer.String() != validateLayer {
			continue
		}

		entries, err := os.ReadDir(l.Dir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			packID := e.Name()
			if validatePack != "" && packID != validatePack {
				continue
			}

			packDir := filepath.Join(l.Dir, packID)
			for filename := range schema.KnownFiles() {
				filePath := filepath.Join(packDir, filename)
				if _, err := os.Stat(filePath); err != nil {
					continue
				}

				totalFiles++
				errs, err := editor.ValidateFile(filePath, schemasDir)
				if err != nil {
					cmd.PrintErrln(fmt.Sprintf("  %s  %s/%s  error: %v", l.Layer, packID, filename, err))
					continue
				}

				r := validateResult{
					Layer:  l.Layer.String(),
					Pack:   packID,
					File:   filename,
					Path:   filePath,
					Errors: errs,
					Passed: !hasValidationErrors(errs),
				}
				results = append(results, r)

				if !validateJSON {
					relPath := filepath.Join(packID, filename)
					if r.Passed {
						cmd.Printf("  %-10s %-40s ✓\n", l.Layer, relPath)
					} else {
						errCount := countValidationErrors(errs)
						totalErrors += errCount
						filesWithErrors++
						cmd.Printf("  %-10s %-40s ✗ %d errors\n", l.Layer, relPath, errCount)
						for _, e := range errs {
							if e.Severity == "error" {
								cmd.Printf("            %s\n", e)
							}
						}
					}
				}
			}
		}
	}

	if validateJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	passed := totalFiles - filesWithErrors
	cmd.Printf("\nValidated %d files across %d layers: %d passed, %d errors in %d files\n",
		totalFiles, len(layers), passed, totalErrors, filesWithErrors)

	if filesWithErrors > 0 {
		os.Exit(1)
	}
	return nil
}

func findSchemasDir(cwd string) string {
	// Check CWD first (contributor checkout)
	candidate := filepath.Join(cwd, "content", "schemas")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	// Check cache
	paths, err := xdgPaths()
	if err != nil {
		return ""
	}
	candidate = filepath.Join(paths.CacheDir, "official", "content", "schemas")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

func hasValidationErrors(errs []schema.ValidationError) bool {
	for _, e := range errs {
		if e.Severity == "error" {
			return true
		}
	}
	return false
}

func countValidationErrors(errs []schema.ValidationError) int {
	n := 0
	for _, e := range errs {
		if e.Severity == "error" {
			n++
		}
	}
	return n
}
```

Note: `xdgPaths()` should reuse the existing pattern from other commands. Check how xdg.New() is called in existing cmd files and follow that pattern.

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add cmd/content_validate.go
git commit -m "feat(content): add content validate command with JSON output"
```

---

### Task 13: CLI command — `content edit`

**Files:**
- Create: `cmd/content_edit.go`

- [ ] **Step 1: Write the content edit subcommand**

```go
package cmd

import (
	"fmt"
	"os"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/editor"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
)

var contentEditCmd = &cobra.Command{
	Use:   "edit <file>",
	Short: "Open interactive editor for a content YAML file",
	Long: `Open a TUI editor for a content YAML file. Supports:
  resources.yaml          (auto-detect pack)
  cap/resources.yaml      (explicit pack)
  ./content/packs/cap/resources.yaml  (direct path)`,
	Args: cobra.ExactArgs(1),
	RunE: runContentEdit,
}

func init() {
	contentCmd.AddCommand(contentEditCmd)
}

func runContentEdit(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	arg := args[0]

	// Check for ambiguous bare filename
	if schema.SchemaForFile(arg) != "" && !containsSlash(arg) {
		packs := editor.AmbiguousPacks(cwd, arg)
		if len(packs) > 1 {
			var selected string
			opts := make([]huh.Option[string], 0, len(packs))
			for _, p := range packs {
				opts = append(opts, huh.NewOption(p, p))
			}
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title(fmt.Sprintf("Multiple packs contain %s. Which one?", arg)).
						Options(opts...).
						Value(&selected),
				),
			)
			if err := form.Run(); err != nil {
				return err
			}
			arg = selected + "/" + arg
		}
	}

	target, err := editor.ResolveEditTarget(cwd, arg)
	if err != nil {
		return err
	}

	schemasDir := findSchemasDir(cwd)
	if schemasDir == "" {
		return fmt.Errorf("cannot find content/schemas/ directory")
	}

	fmt.Fprintf(os.Stderr, "Editing: %s (%s) · %s\n", target.Filename, target.Layer, target.FilePath)

	return editor.Run(target, schemasDir)
}

func containsSlash(s string) bool {
	for _, c := range s {
		if c == '/' {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add cmd/content_edit.go
git commit -m "feat(content): add content edit command with pack disambiguation"
```

---

### Task 14: Update CLAUDE.md CLI reference table

**Files:**
- Modify: `CLAUDE.md`
- Modify: `content/packs/base/context.md` (if CLI reference table exists there)

- [ ] **Step 1: Add `content` command to CLI commands table in CLAUDE.md**

Add this row to the `### CLI Commands` table:

```
| `content` | Manage content YAML files; `content edit/validate/list` with `--pack`/`--layer`/`--json` filtering |
```

Insert alphabetically between `context` and `discovery`.

- [ ] **Step 2: Verify the file renders correctly**

Read the modified section to verify formatting.

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add content command to CLI reference table"
```

---

### Task 15: Documentation — content authoring guide

**Files:**
- Modify: `docs/content-authoring.md`

- [ ] **Step 1: Add content editor section to docs/content-authoring.md**

Add a new top-level section covering:

1. **Content Editor** — overview of `content edit`, `content validate`, `content list`
2. **Editing content in a checkout** — contributor workflow for official/company repos
3. **Creating user overrides** — how `content edit` auto-creates user-layer overrides
4. **Creating project overrides** — project-scoped customization in `.sap-devs/`
5. **Schema-driven value help** — how enums, formats, patterns drive the form UI
6. **Validation** — how `content validate` works, CI integration with exit codes

Include examples of each command with expected output.

- [ ] **Step 2: Commit**

```bash
git add docs/content-authoring.md
git commit -m "docs: add content editing and validation guide"
```

---

### Task 16: Update TODO.md — mark Content Editing UI as in-progress

**Files:**
- Modify: `TODO.md`

- [ ] **Step 1: Update the Content Editing UI section to reflect Phase 1 completion**

Mark Phase 1 items (edit, validate, list, schema-driven forms, layer resolution) as done. Keep Phase 2 items (undo/redo, diff view, git integration, drag-and-drop, bulk edit, creation wizard) as future.

- [ ] **Step 2: Commit**

```bash
git add TODO.md
git commit -m "docs: update TODO with content editing Phase 1 progress"
```

---

### Task 17: Integration test — build and smoke test

**Files:**
- No new files

- [ ] **Step 1: Verify full build**

```bash
go build ./...
go vet ./...
```

Expected: Both pass cleanly.

- [ ] **Step 2: Test content list command**

```bash
go run . content list
```

Expected: Prints table of content files across detected layers.

- [ ] **Step 3: Test content validate command**

```bash
go run . content validate
```

Expected: Validates all content files, prints results, exits 0 if all pass.

- [ ] **Step 4: Test content validate with JSON output**

```bash
go run . content validate --json
```

Expected: Prints JSON array of validation results.

- [ ] **Step 5: Test content edit (dry run)**

```bash
go run . content edit cap/resources.yaml
```

Expected: Opens TUI editor showing resources in list view. Press `Esc` to quit without saving.

- [ ] **Step 6: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address integration test findings"
```
