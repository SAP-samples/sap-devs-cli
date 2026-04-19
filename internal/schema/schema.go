// Package schema parses JSON Schema files from content/schemas/ and exposes
// FieldSpec models used by the content-editing UI and validation engine.
package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// knownFiles maps a YAML filename (without path) to the schema base-name
// (without the ".schema.json" suffix).
var knownFiles = map[string]string{
	"resources.yaml":       "resources",
	"pack.yaml":            "pack",
	"influencers.yaml":     "influencers",
	"event-types.yaml":     "event-types",
	"event-instances.yaml": "event-instances",
	"mcp.yaml":             "mcp",
	"tools.yaml":           "tools",
	"hook.yaml":            "hook",
	"samples.yaml":         "samples",
	"known_errors.yaml":    "known_errors",
}

// KnownFiles returns the set of YAML filenames that have an associated schema.
func KnownFiles() []string {
	out := make([]string, 0, len(knownFiles))
	for k := range knownFiles {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// SchemaForFile returns the schema base-name for a given YAML filename, and
// whether one exists.
func SchemaForFile(yamlFile string) (string, bool) {
	base := filepath.Base(yamlFile)
	name, ok := knownFiles[base]
	return name, ok
}

// FieldSpec describes a single field extracted from a JSON Schema property.
type FieldSpec struct {
	Key          string
	Title        string
	Type         string // "string","integer","boolean","array","object","map"
	Required     bool
	Enum         []string
	Format       string
	Pattern      string
	MapValueType string // for Type=="map": the additionalProperties type
	Children     []FieldSpec
	Condition    *Condition
	MinItems     int
	MaxItems     int   // 0 means no limit
	ItemType     string
	ItemEnum     []string
}

// Condition holds an if/then conditional parsed from the schema.
type Condition struct {
	// IfConst is the constant value the trigger field must equal.
	TriggerField string
	TriggerConst string
	// ThenFields lists fields that become required when the condition is met.
	ThenRequired []string
	// ThenEnum constrains enumerated values for a specific field when met.
	ThenField string
	ThenEnum  []string
}

// ObjectSpec is the top-level specification for an object schema.
type ObjectSpec struct {
	Fields       []FieldSpec
	Required     []string
	Conditionals []Condition
}

// Schema is the parsed representation of one JSON Schema file.
type Schema struct {
	Name       string
	Title      string
	TopType    string // "array" or "object"
	ItemSpec   *ObjectSpec // non-nil when TopType=="array"
	ObjectSpec *ObjectSpec // non-nil when TopType=="object"
}

// Load reads and parses the named schema from schemasDir.
// name is the base-name without ".schema.json", e.g. "resources".
func Load(schemasDir, name string) (*Schema, error) {
	path := filepath.Join(schemasDir, name+".schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("schema %s: %w", name, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("schema %s: %w", name, err)
	}

	s := &Schema{
		Name:  name,
		Title: strVal(raw, "title"),
	}

	topType := strVal(raw, "type")
	s.TopType = topType

	switch topType {
	case "array":
		itemsRaw, _ := raw["items"].(map[string]any)
		spec := parseObjectSpec(itemsRaw)
		s.ItemSpec = spec
	case "object":
		spec := parseObjectSpec(raw)
		conditionals := parseConditionals(raw)
		spec.Conditionals = conditionals
		s.ObjectSpec = spec
	}

	return s, nil
}

// parseObjectSpec extracts Fields and Required from a JSON Schema object node.
func parseObjectSpec(raw map[string]any) *ObjectSpec {
	if raw == nil {
		return &ObjectSpec{}
	}
	spec := &ObjectSpec{}
	spec.Required = extractStringSlice(raw, "required")

	propsRaw, _ := raw["properties"].(map[string]any)
	for key, v := range propsRaw {
		propMap, _ := v.(map[string]any)
		f := parseField(key, propMap)
		f.Required = contains(spec.Required, key)
		spec.Fields = append(spec.Fields, f)
	}

	sortFieldsBySchemaOrder(spec.Fields, spec.Required)
	return spec
}

// parseField converts a single JSON Schema property definition into a FieldSpec.
func parseField(key string, raw map[string]any) FieldSpec {
	if raw == nil {
		return FieldSpec{Key: key, Title: keyToTitle(key), Type: "string"}
	}

	f := FieldSpec{
		Key:   key,
		Title: keyToTitle(key),
	}

	t := strVal(raw, "type")

	// Check for additionalProperties → map type
	if addPropsRaw, hasAdd := raw["additionalProperties"]; hasAdd && t == "object" {
		addProps, _ := addPropsRaw.(map[string]any)
		if addProps != nil {
			f.Type = "map"
			f.MapValueType = strVal(addProps, "type")
			if fmtVal := strVal(addProps, "format"); fmtVal != "" {
				f.Format = fmtVal
			}
		} else {
			// additionalProperties: false or non-object
			f.Type = "object"
		}
	} else if t == "object" && raw["properties"] != nil {
		f.Type = "object"
		nested := parseObjectSpec(raw)
		f.Children = nested.Fields
	} else {
		f.Type = t
	}

	if f.Type == "" {
		f.Type = "string"
	}

	f.Enum = extractStringSlice(raw, "enum")
	f.Format = strVal(raw, "format")
	f.Pattern = strVal(raw, "pattern")

	// array specifics
	if t == "array" {
		if mi, ok := raw["minItems"].(float64); ok {
			f.MinItems = int(mi)
		}
		if ma, ok := raw["maxItems"].(float64); ok {
			f.MaxItems = int(ma)
		}
		if itemsRaw, ok := raw["items"].(map[string]any); ok {
			f.ItemType = strVal(itemsRaw, "type")
			f.ItemEnum = extractStringSlice(itemsRaw, "enum")
		}
	}

	// if/then at field level
	if ifRaw, ok := raw["if"].(map[string]any); ok {
		cond := parseFieldCondition(ifRaw, raw["then"])
		if cond != nil {
			f.Condition = cond
		}
	}

	return f
}

// parseConditionals extracts top-level if/then/else from a schema object.
func parseConditionals(raw map[string]any) []Condition {
	ifRaw, ok := raw["if"].(map[string]any)
	if !ok {
		return nil
	}
	cond := parseFieldCondition(ifRaw, raw["then"])
	if cond == nil {
		return nil
	}
	return []Condition{*cond}
}

// parseFieldCondition converts an if/then pair into a Condition.
func parseFieldCondition(ifRaw map[string]any, thenAny any) *Condition {
	if ifRaw == nil {
		return nil
	}
	cond := &Condition{}

	// Extract trigger: if.properties.<field>.const == <value>
	if propsRaw, ok := ifRaw["properties"].(map[string]any); ok {
		for field, vRaw := range propsRaw {
			vMap, _ := vRaw.(map[string]any)
			if constVal, ok := vMap["const"]; ok {
				cond.TriggerField = field
				switch cv := constVal.(type) {
				case string:
					cond.TriggerConst = cv
				case bool:
					if cv {
						cond.TriggerConst = "true"
					} else {
						cond.TriggerConst = "false"
					}
				}
			}
		}
	}

	// Extract then clause
	thenRaw, _ := thenAny.(map[string]any)
	if thenRaw != nil {
		cond.ThenRequired = extractStringSlice(thenRaw, "required")
		if thenProps, ok := thenRaw["properties"].(map[string]any); ok {
			for field, vRaw := range thenProps {
				vMap, _ := vRaw.(map[string]any)
				if enum := extractStringSlice(vMap, "enum"); len(enum) > 0 {
					cond.ThenField = field
					cond.ThenEnum = enum
				}
			}
		}
	}

	if cond.TriggerField == "" {
		return nil
	}
	return cond
}

// keyToTitle converts a snake_case or kebab-case key to a human-readable title.
func keyToTitle(key string) string {
	key = strings.ReplaceAll(key, "_", " ")
	key = strings.ReplaceAll(key, "-", " ")
	if key == "" {
		return key
	}
	runes := []rune(key)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// strVal safely reads a string value from a map.
func strVal(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// extractStringSlice reads a []string from a JSON array field.
func extractStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	arr, _ := m[key].([]any)
	if arr == nil {
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

// sortFieldsBySchemaOrder places required fields first, preserving insertion
// order within each group.
func sortFieldsBySchemaOrder(fields []FieldSpec, required []string) {
	sort.SliceStable(fields, func(i, j int) bool {
		ri := contains(required, fields[i].Key)
		rj := contains(required, fields[j].Key)
		if ri == rj {
			return false
		}
		return ri
	})
}

// contains reports whether s is in the slice.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
