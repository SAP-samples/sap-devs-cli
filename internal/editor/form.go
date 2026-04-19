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

// StringBinding holds a string value for huh form binding.
type StringBinding struct{ Value string }

// BoolBinding holds a bool value for huh form binding.
type BoolBinding struct{ Value bool }

// SliceBinding holds a string slice value for huh form binding.
type SliceBinding struct{ Value []string }

// Bindings maps field keys to their typed binding structs.
type Bindings struct {
	Strings map[string]*StringBinding
	Bools   map[string]*BoolBinding
	Slices  map[string]*SliceBinding
	Objects map[string]map[string]any // nested objects (passthrough)
	Maps    map[string]map[string]any // map fields (passthrough)
}

// NewBindings creates an empty Bindings container.
func NewBindings() *Bindings {
	return &Bindings{
		Strings: make(map[string]*StringBinding),
		Bools:   make(map[string]*BoolBinding),
		Slices:  make(map[string]*SliceBinding),
		Objects: make(map[string]map[string]any),
		Maps:    make(map[string]map[string]any),
	}
}

// ToMap collects all binding values into a flat map[string]any suitable for
// YAML marshaling. It resolves _raw fields back to arrays and converts integer
// strings back to int.
func (b *Bindings) ToMap(spec *schema.ObjectSpec) map[string]any {
	result := make(map[string]any)

	for k, v := range b.Strings {
		if strings.HasSuffix(k, "_raw") {
			continue
		}
		result[k] = v.Value
	}
	for k, v := range b.Bools {
		result[k] = v.Value
	}
	for k, v := range b.Slices {
		anySlice := make([]any, len(v.Value))
		for i, s := range v.Value {
			anySlice[i] = s
		}
		result[k] = anySlice
	}
	for k, v := range b.Objects {
		result[k] = v
	}
	for k, v := range b.Maps {
		result[k] = v
	}

	// Resolve comma-separated _raw fields back to arrays.
	for _, f := range spec.Fields {
		if f.Type == "array" && len(f.ItemEnum) == 0 {
			if raw, ok := b.Strings[f.Key+"_raw"]; ok {
				tags := splitTags(raw.Value)
				anyTags := make([]any, len(tags))
				for i, t := range tags {
					anyTags[i] = t
				}
				result[f.Key] = anyTags
			}
		}
		// Convert integer strings back to int.
		if f.Type == "integer" {
			if s, ok := b.Strings[f.Key]; ok && s.Value != "" {
				if n, err := strconv.Atoi(s.Value); err == nil {
					result[f.Key] = n
				}
			}
		}
	}

	return result
}

// BuildForm creates a huh form from a schema ObjectSpec and current values.
// Conditional fields whose condition is not met are skipped.
func BuildForm(spec *schema.ObjectSpec, values map[string]any) (*huh.Form, *Bindings) {
	bindings := NewBindings()

	var fields []huh.Field
	for _, f := range spec.Fields {
		// Skip conditional fields whose condition is not met.
		if f.Condition != nil {
			currentVal := fmt.Sprintf("%v", values[f.Condition.TriggerField])
			if currentVal != f.Condition.TriggerConst {
				continue
			}
		}

		field := buildField(f, values, bindings)
		if field != nil {
			fields = append(fields, field)
		}
	}

	group := huh.NewGroup(fields...)
	form := huh.NewForm(group).WithTheme(huh.ThemeFunc(huh.ThemeDracula))
	return form, bindings
}

func buildField(f schema.FieldSpec, values map[string]any, bindings *Bindings) huh.Field {
	switch f.Type {
	case "string":
		return buildStringField(f, values, bindings)
	case "integer":
		return buildIntegerField(f, values, bindings)
	case "boolean":
		return buildBoolField(f, values, bindings)
	case "array":
		return buildArrayField(f, values, bindings)
	case "object":
		return buildObjectFields(f, values, bindings)
	case "map":
		return buildMapField(f, values, bindings)
	}
	return nil
}

func buildStringField(f schema.FieldSpec, values map[string]any, bindings *Bindings) huh.Field {
	if len(f.Enum) > 0 {
		current, _ := values[f.Key].(string)
		b := &StringBinding{Value: current}
		bindings.Strings[f.Key] = b

		opts := make([]huh.Option[string], 0, len(f.Enum))
		for _, e := range f.Enum {
			opts = append(opts, huh.NewOption(e, e))
		}

		return huh.NewSelect[string]().
			Title(fieldTitle(f)).
			Options(opts...).
			Value(&b.Value)
	}

	current, _ := values[f.Key].(string)
	b := &StringBinding{Value: current}
	bindings.Strings[f.Key] = b

	input := huh.NewInput().
		Title(fieldTitle(f)).
		Placeholder(placeholderForField(f)).
		Value(&b.Value)

	if v := validatorForString(f); v != nil {
		input = input.Validate(v)
	}

	return input
}

func buildIntegerField(f schema.FieldSpec, values map[string]any, bindings *Bindings) huh.Field {
	var current string
	switch v := values[f.Key].(type) {
	case float64:
		current = strconv.Itoa(int(v))
	case int:
		current = strconv.Itoa(v)
	}
	b := &StringBinding{Value: current}
	bindings.Strings[f.Key] = b

	return huh.NewInput().
		Title(fieldTitle(f)).
		Placeholder("0").
		Value(&b.Value).
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

func buildBoolField(f schema.FieldSpec, values map[string]any, bindings *Bindings) huh.Field {
	current, _ := values[f.Key].(bool)
	b := &BoolBinding{Value: current}
	bindings.Bools[f.Key] = b

	return huh.NewConfirm().
		Title(fieldTitle(f)).
		Value(&b.Value)
}

func buildArrayField(f schema.FieldSpec, values map[string]any, bindings *Bindings) huh.Field {
	rawArr, _ := values[f.Key].([]any)
	var current []string
	for _, v := range rawArr {
		if s, ok := v.(string); ok {
			current = append(current, s)
		}
	}

	if len(f.ItemEnum) > 0 {
		b := &SliceBinding{Value: current}
		bindings.Slices[f.Key] = b

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
			Options(opts...).
			Value(&b.Value)
	}

	// Free-form string array: comma-separated input.
	joined := strings.Join(current, ", ")
	b := &StringBinding{Value: joined}
	bindings.Strings[f.Key+"_raw"] = b
	return huh.NewInput().
		Title(fieldTitle(f) + " (comma-separated)").
		Value(&b.Value).
		Validate(func(s string) error {
			parts := splitTags(s)
			if f.MinItems > 0 && len(parts) < f.MinItems {
				return fmt.Errorf("at least %d item(s) required", f.MinItems)
			}
			return nil
		})
}

func buildObjectFields(f schema.FieldSpec, values map[string]any, bindings *Bindings) huh.Field {
	childObj, ok := values[f.Key].(map[string]any)
	if !ok {
		childObj = make(map[string]any)
	}
	bindings.Objects[f.Key] = childObj

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

func buildMapField(f schema.FieldSpec, values map[string]any, bindings *Bindings) huh.Field {
	mapObj, ok := values[f.Key].(map[string]any)
	if !ok {
		mapObj = make(map[string]any)
	}
	bindings.Maps[f.Key] = mapObj

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
