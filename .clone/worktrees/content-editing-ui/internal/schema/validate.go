package schema

import (
	"fmt"
	"net/url"
	"regexp"
)

// Severity indicates how serious a validation finding is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// ValidationError describes a single validation finding.
type ValidationError struct {
	Path     string   // JSON-pointer-style path, e.g. "[0].url"
	Field    string   // field key
	Message  string   // human-readable message
	Severity Severity // "error" or "warning"
}

// String returns a formatted representation of the error.
func (e ValidationError) String() string {
	return fmt.Sprintf("[%s] %s%s: %s", e.Severity, e.Path, e.Field, e.Message)
}

// Validate dispatches to array or object validation based on the schema type.
// data should be the unmarshalled YAML/JSON value ([]any or map[string]any).
func Validate(s *Schema, data any) []ValidationError {
	switch s.TopType {
	case "array":
		if s.ItemSpec == nil {
			return nil
		}
		items, ok := data.([]any)
		if !ok {
			return []ValidationError{{
				Path:     "",
				Field:    "",
				Message:  "expected array",
				Severity: SeverityError,
			}}
		}
		var errs []ValidationError
		for i, item := range items {
			obj, ok := item.(map[string]any)
			if !ok {
				errs = append(errs, ValidationError{
					Path:     fmt.Sprintf("[%d]", i),
					Field:    "",
					Message:  "expected object",
					Severity: SeverityError,
				})
				continue
			}
			for _, ve := range validateObject(s.ItemSpec, obj) {
				ve.Path = fmt.Sprintf("[%d]%s", i, ve.Path)
				errs = append(errs, ve)
			}
		}
		return errs

	case "object":
		if s.ObjectSpec == nil {
			return nil
		}
		obj, ok := data.(map[string]any)
		if !ok {
			return []ValidationError{{
				Path:     "",
				Field:    "",
				Message:  "expected object",
				Severity: SeverityError,
			}}
		}
		return validateObject(s.ObjectSpec, obj)

	default:
		return nil
	}
}

// validateObject checks required fields and validates each present field
// against its FieldSpec.
func validateObject(spec *ObjectSpec, obj map[string]any) []ValidationError {
	var errs []ValidationError

	// Required field presence
	for _, req := range spec.Required {
		if _, ok := obj[req]; !ok {
			errs = append(errs, ValidationError{
				Path:     ".",
				Field:    req,
				Message:  "required field is missing",
				Severity: SeverityError,
			})
		}
	}

	// Build field spec map for lookup
	fieldMap := make(map[string]FieldSpec, len(spec.Fields))
	for _, f := range spec.Fields {
		fieldMap[f.Key] = f
	}

	// Validate each present field
	for key, val := range obj {
		f, ok := fieldMap[key]
		if !ok {
			// unknown field — not validated here (additionalProperties enforcement
			// is left to the schema validator)
			continue
		}
		for _, ve := range validateField(f, val) {
			ve.Path = "." + key
			errs = append(errs, ve)
		}
	}

	return errs
}

// validateField performs type-specific validation of a single field value.
func validateField(f FieldSpec, val any) []ValidationError {
	if val == nil {
		return nil
	}
	var errs []ValidationError

	switch f.Type {
	case "string":
		s, ok := val.(string)
		if !ok {
			errs = append(errs, ValidationError{
				Field:    f.Key,
				Message:  fmt.Sprintf("expected string, got %T", val),
				Severity: SeverityError,
			})
			break
		}
		// enum check
		if len(f.Enum) > 0 && !contains(f.Enum, s) {
			errs = append(errs, ValidationError{
				Field:   f.Key,
				Message: fmt.Sprintf("value %q is not one of %v", s, f.Enum),
				Severity: SeverityError,
			})
		}
		// format uri
		if f.Format == "uri" {
			if err := validateURI(s); err != nil {
				errs = append(errs, ValidationError{
					Field:    f.Key,
					Message:  err.Error(),
					Severity: SeverityError,
				})
			}
		}
		// pattern
		if f.Pattern != "" {
			matched, err := regexp.MatchString(f.Pattern, s)
			if err == nil && !matched {
				errs = append(errs, ValidationError{
					Field:   f.Key,
					Message: fmt.Sprintf("value %q does not match pattern %q", s, f.Pattern),
					Severity: SeverityError,
				})
			}
		}

	case "integer":
		switch val.(type) {
		case float64, int, int64, int32:
			// acceptable numeric types from YAML/JSON
		default:
			errs = append(errs, ValidationError{
				Field:    f.Key,
				Message:  fmt.Sprintf("expected integer, got %T", val),
				Severity: SeverityError,
			})
		}

	case "boolean":
		if _, ok := val.(bool); !ok {
			errs = append(errs, ValidationError{
				Field:    f.Key,
				Message:  fmt.Sprintf("expected boolean, got %T", val),
				Severity: SeverityError,
			})
		}

	case "array":
		items, ok := val.([]any)
		if !ok {
			errs = append(errs, ValidationError{
				Field:    f.Key,
				Message:  fmt.Sprintf("expected array, got %T", val),
				Severity: SeverityError,
			})
			break
		}
		if f.MinItems > 0 && len(items) < f.MinItems {
			errs = append(errs, ValidationError{
				Field:   f.Key,
				Message: fmt.Sprintf("array has %d items, minimum is %d", len(items), f.MinItems),
				Severity: SeverityError,
			})
		}
		if f.MaxItems > 0 && len(items) > f.MaxItems {
			errs = append(errs, ValidationError{
				Field:   f.Key,
				Message: fmt.Sprintf("array has %d items, maximum is %d", len(items), f.MaxItems),
				Severity: SeverityError,
			})
		}
		for i, item := range items {
			if f.ItemType == "string" {
				s, ok := item.(string)
				if !ok {
					errs = append(errs, ValidationError{
						Field:    fmt.Sprintf("%s[%d]", f.Key, i),
						Message:  fmt.Sprintf("expected string, got %T", item),
						Severity: SeverityError,
					})
					continue
				}
				if len(f.ItemEnum) > 0 && !contains(f.ItemEnum, s) {
					errs = append(errs, ValidationError{
						Field:   fmt.Sprintf("%s[%d]", f.Key, i),
						Message: fmt.Sprintf("value %q is not one of %v", s, f.ItemEnum),
						Severity: SeverityError,
					})
				}
			}
		}

	case "object":
		obj, ok := val.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{
				Field:    f.Key,
				Message:  fmt.Sprintf("expected object, got %T", val),
				Severity: SeverityError,
			})
			break
		}
		if len(f.Children) > 0 {
			// Build a temporary ObjectSpec from Children for recursive validation
			childRequired := make([]string, 0)
			for _, child := range f.Children {
				if child.Required {
					childRequired = append(childRequired, child.Key)
				}
			}
			childSpec := &ObjectSpec{
				Fields:   f.Children,
				Required: childRequired,
			}
			for _, ve := range validateObject(childSpec, obj) {
				ve.Path = "." + f.Key + ve.Path
				errs = append(errs, ve)
			}
		}

	case "map":
		m, ok := val.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{
				Field:    f.Key,
				Message:  fmt.Sprintf("expected map, got %T", val),
				Severity: SeverityError,
			})
			break
		}
		for k, v := range m {
			switch f.MapValueType {
			case "string":
				s, ok := v.(string)
				if !ok {
					errs = append(errs, ValidationError{
						Field:    fmt.Sprintf("%s[%s]", f.Key, k),
						Message:  fmt.Sprintf("expected string, got %T", v),
						Severity: SeverityError,
					})
					continue
				}
				if f.Format == "uri" {
					if err := validateURI(s); err != nil {
						errs = append(errs, ValidationError{
							Field:    fmt.Sprintf("%s[%s]", f.Key, k),
							Message:  err.Error(),
							Severity: SeverityError,
						})
					}
				}
			}
		}
	}

	return errs
}

// validateURI checks that s is an absolute URI with a scheme.
func validateURI(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("value %q is not a valid URI", s)
	}
	return nil
}
