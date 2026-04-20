package editor

import (
	"errors"
	"fmt"

	"charm.land/huh/v2"
	"github.com/SAP-samples/sap-devs-cli/internal/schema"
	"github.com/SAP-samples/sap-devs-cli/internal/theme"
)

// BulkSetField opens a form to pick a field and value for bulk assignment.
// Returns the field key and new value. Returns an error if the user aborts.
func BulkSetField(spec *schema.ObjectSpec) (string, any, error) {
	candidates := bulkSettableFields(spec)
	if len(candidates) == 0 {
		return "", nil, fmt.Errorf("no fields available for bulk set")
	}

	var fieldKey string
	opts := make([]huh.Option[string], 0, len(candidates))
	for _, f := range candidates {
		label := fmt.Sprintf("%s (%s)", f.Title, f.Type)
		opts = append(opts, huh.NewOption(label, f.Key))
	}

	fieldForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Field to set").
				Options(opts...).
				Value(&fieldKey),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))

	if err := fieldForm.Run(); err != nil {
		return "", nil, err
	}

	var chosen schema.FieldSpec
	for _, f := range candidates {
		if f.Key == fieldKey {
			chosen = f
			break
		}
	}

	if len(chosen.Enum) > 0 {
		var val string
		enumOpts := make([]huh.Option[string], 0, len(chosen.Enum))
		for _, e := range chosen.Enum {
			enumOpts = append(enumOpts, huh.NewOption(e, e))
		}
		valForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("Value for %s", chosen.Title)).
					Options(enumOpts...).
					Value(&val),
			),
		).WithTheme(huh.ThemeFunc(theme.ThemeFiori))
		if err := valForm.Run(); err != nil {
			return "", nil, err
		}
		return fieldKey, val, nil
	}

	var val string
	valForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Value for %s", chosen.Title)).
				Value(&val),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))
	if err := valForm.Run(); err != nil {
		return "", nil, err
	}
	return fieldKey, val, nil
}

// BulkAddRemoveTag opens a form to add or remove a tag value on an array field.
func BulkAddRemoveTag(spec *schema.ObjectSpec) (action string, field string, value string, err error) {
	arrayFields := bulkArrayFields(spec)
	if len(arrayFields) == 0 {
		return "", "", "", fmt.Errorf("no array fields available")
	}

	actionForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Action").
				Options(
					huh.NewOption("Add tag", "add"),
					huh.NewOption("Remove tag", "remove"),
				).
				Value(&action),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))
	if err := actionForm.Run(); err != nil {
		return "", "", "", err
	}

	if len(arrayFields) == 1 {
		field = arrayFields[0].Key
	} else {
		fieldOpts := make([]huh.Option[string], 0, len(arrayFields))
		for _, f := range arrayFields {
			fieldOpts = append(fieldOpts, huh.NewOption(f.Title, f.Key))
		}
		fieldForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Field").
					Options(fieldOpts...).
					Value(&field),
			),
		).WithTheme(huh.ThemeFunc(theme.ThemeFiori))
		if err := fieldForm.Run(); err != nil {
			return "", "", "", err
		}
	}

	valForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Tag value").
				Value(&value),
		),
	).WithTheme(huh.ThemeFunc(theme.ThemeFiori))
	if err := valForm.Run(); err != nil {
		return "", "", "", err
	}

	return action, field, value, nil
}

// IsUserAborted reports whether the error is a user abort from huh.
func IsUserAborted(err error) bool {
	return errors.Is(err, huh.ErrUserAborted)
}

// BulkDeleteItems removes items at the selected indices and returns a new slice.
func BulkDeleteItems(items []MergedItem, selected map[int]bool) []MergedItem {
	result := make([]MergedItem, 0, len(items)-len(selected))
	for i, item := range items {
		if !selected[i] {
			result = append(result, item)
		}
	}
	return result
}

// BulkApplyTag adds or removes a tag value from an array field on the selected items.
func BulkApplyTag(items []MergedItem, selected map[int]bool, field, value, action string) {
	for idx := range selected {
		if idx < 0 || idx >= len(items) {
			continue
		}
		arr, _ := items[idx].Data[field].([]any)
		if arr == nil {
			arr = []any{}
		}
		switch action {
		case "add":
			arr = append(arr, value)
		case "remove":
			filtered := make([]any, 0, len(arr))
			for _, v := range arr {
				if v != value {
					filtered = append(filtered, v)
				}
			}
			arr = filtered
		}
		items[idx].Data[field] = arr
	}
}

// bulkSettableFields returns fields suitable for bulk set: string, integer, boolean types.
func bulkSettableFields(spec *schema.ObjectSpec) []schema.FieldSpec {
	var out []schema.FieldSpec
	for _, f := range spec.Fields {
		switch f.Type {
		case "string", "integer", "boolean":
			out = append(out, f)
		}
	}
	return out
}

// bulkArrayFields returns fields of type "array" from the spec.
func bulkArrayFields(spec *schema.ObjectSpec) []schema.FieldSpec {
	var out []schema.FieldSpec
	for _, f := range spec.Fields {
		if f.Type == "array" {
			out = append(out, f)
		}
	}
	return out
}
