package editor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"charm.land/huh/v2"
	"github.com/SAP-samples/sap-devs-cli/internal/schema"
	"gopkg.in/yaml.v3"
)

// Run opens the editor for the given resolved file. It loads the matching
// schema and dispatches to either an object editor (pack.yaml) or an array
// editor (resources.yaml, etc.).
func Run(target *ResolvedFile, schemasDir string) error {
	s, err := schema.Load(schemasDir, target.SchemaName)
	if err != nil {
		return fmt.Errorf("load schema: %w", err)
	}

	switch s.TopType {
	case "object":
		return runObjectEditor(target, s)
	case "array":
		return runArrayEditor(target, s)
	}
	return fmt.Errorf("unsupported schema type: %s", s.TopType)
}

func runObjectEditor(target *ResolvedFile, s *schema.Schema) error {
	obj, err := LoadSingleObject(target.FilePath)
	if err != nil {
		return err
	}

	form, bindings := BuildForm(s.ObjectSpec, obj)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil // user cancelled
		}
		return err
	}

	obj = bindings.ToMap(s.ObjectSpec)

	// Validate before saving.
	errs := schema.Validate(s, obj)
	if hasErrors(errs) {
		fmt.Fprintf(os.Stderr, "\nValidation errors:\n")
		for _, e := range errs {
			if e.Severity == schema.SeverityError {
				fmt.Fprintf(os.Stderr, "  %s\n", e)
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
	history := NewHistory(items)
	var statusMsg string

	for {
		listMdl := newListModel(items, columns, target, s, history, statusMsg)
		p := tea.NewProgram(listMdl, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			return err
		}

		result := finalModel.(listModel)
		statusMsg = ""

		// Handle undo.
		if result.undone {
			if restored, desc, ok := history.Undo(items); ok {
				items = restored
				statusMsg = fmt.Sprintf("↩ undid: %s", desc)
			}
			continue
		}

		// Handle redo.
		if result.redone {
			if restored, desc, ok := history.Redo(items); ok {
				items = restored
				statusMsg = fmt.Sprintf("↪ redid: %s", desc)
			}
			continue
		}

		// Handle reorder.
		if result.moveUp || result.moveDown {
			sel := result.selected
			if len(sel) == 0 {
				cursorIdx := result.cursorOriginalIndex
				if cursorIdx >= 0 && items[cursorIdx].Layer == target.Layer {
					sel = map[int]bool{cursorIdx: true}
				}
			}
			if len(sel) > 0 {
				desc := fmt.Sprintf("reordered %d item(s)", len(sel))
				history.Push(items, desc)
				items = MoveItems(items, sel, result.moveUp)
				statusMsg = fmt.Sprintf("✓ %s", desc)
			}
			continue
		}

		// Handle bulk actions.
		if result.bulkAction != "" {
			switch result.bulkAction {
			case "set-field":
				field, value, err := BulkSetField(s.ItemSpec)
				if err != nil {
					if IsUserAborted(err) {
						continue
					}
					continue
				}
				// Coerce string values to native types for integer and boolean fields.
				for _, f := range s.ItemSpec.Fields {
					if f.Key == field {
						if f.Type == "integer" {
							if n, convErr := strconv.Atoi(value.(string)); convErr == nil {
								value = n
							}
						} else if f.Type == "boolean" {
							switch value.(string) {
							case "true":
								value = true
							case "false":
								value = false
							}
						}
						break
					}
				}
				desc := fmt.Sprintf("set %s on %d item(s)", field, len(result.selected))
				history.Push(items, desc)
				for idx := range result.selected {
					if items[idx].Layer != target.Layer {
						cloned := make(map[string]any)
						for k, v := range items[idx].Data {
							cloned[k] = v
						}
						items[idx].Data = cloned
						items[idx].Layer = target.Layer
						items[idx].IsOverride = true
					}
					items[idx].Data[field] = value
				}
				statusMsg = fmt.Sprintf("✓ %s", desc)

			case "delete":
				desc := fmt.Sprintf("deleted %d item(s)", len(result.selected))
				history.Push(items, desc)
				items = BulkDeleteItems(items, result.selected)
				statusMsg = fmt.Sprintf("✓ %s", desc)

			case "add-tag":
				action, field, value, err := BulkAddRemoveTag(s.ItemSpec)
				if err != nil {
					if IsUserAborted(err) {
						continue
					}
					continue
				}
				desc := fmt.Sprintf("%s tag %q on %s for %d item(s)", action, value, field, len(result.selected))
				history.Push(items, desc)
				for idx := range result.selected {
					if items[idx].Layer != target.Layer {
						cloned := make(map[string]any)
						for k, v := range items[idx].Data {
							cloned[k] = v
						}
						items[idx].Data = cloned
						items[idx].Layer = target.Layer
						items[idx].IsOverride = true
					}
				}
				BulkApplyTag(items, result.selected, field, value, action)
				statusMsg = fmt.Sprintf("✓ %s", desc)
			}
			continue
		}

		// Handle delete.
		if result.deleteIdx >= 0 {
			desc := descForItem("deleted", items[result.deleteIdx].Data)
			history.Push(items, desc)
			items = append(items[:result.deleteIdx], items[result.deleteIdx+1:]...)
			statusMsg = fmt.Sprintf("✓ %s", desc)
			continue
		}

		// Handle add new item.
		if result.addNew {
			newItem := make(map[string]any)
			if err := editItem(s.ItemSpec, newItem); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				continue
			}
			history.Push(items, descForItem("added", newItem))
			items = append(items, MergedItem{
				Data:  newItem,
				Layer: target.Layer,
			})
			statusMsg = fmt.Sprintf("✓ %s", descForItem("added", newItem))
			continue
		}

		// Handle edit existing item.
		if result.editIndex >= 0 {
			item := &items[result.editIndex]
			if item.Layer != target.Layer {
				cloned := make(map[string]any)
				for k, v := range item.Data {
					cloned[k] = v
				}
				item.Data = cloned
				item.Layer = target.Layer
				item.IsOverride = true
			}
			desc := descForItem("edited", item.Data)
			history.Push(items, desc)
			if err := editItem(s.ItemSpec, item.Data); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					if restored, ok := history.DiscardLast(); ok {
						items = restored
					}
					continue
				}
				continue
			}
			statusMsg = fmt.Sprintf("✓ %s", desc)
			continue
		}

		// Handle save+quit.
		if result.save {
			if !history.HasChanges(items) {
				fmt.Fprintln(os.Stdout, "No changes.")
				return nil
			}

			changes := history.Changes(items)
			dm := newDiffModel(changes)
			dp := tea.NewProgram(dm, tea.WithAltScreen())
			diffResult, err := dp.Run()
			if err != nil {
				return err
			}

			switch diffResult.(diffModel).action {
			case diffSave:
				return SaveItems(target.FilePath, items, target.Layer)
			case diffDiscard:
				fmt.Fprintln(os.Stdout, "Changes discarded.")
				return nil
			case diffCancel:
				statusMsg = "Save cancelled — back to editing"
				continue
			}
		}

		// Quit without saving (Esc).
		return nil
	}
}

func descForItem(verb string, data map[string]any) string {
	id := itemKey(data)
	if id == "" {
		id = "item"
	}
	return fmt.Sprintf("%s %q", verb, id)
}

// editItem opens a form for a single item and merges the result back.
func editItem(spec *schema.ObjectSpec, data map[string]any) error {
	form, bindings := BuildForm(spec, data)
	if err := form.Run(); err != nil {
		return err
	}

	result := bindings.ToMap(spec)
	for k, v := range result {
		data[k] = v
	}
	return nil
}

// hasErrors reports whether any validation errors have severity "error".
func hasErrors(errs []schema.ValidationError) bool {
	for _, e := range errs {
		if e.Severity == schema.SeverityError {
			return true
		}
	}
	return false
}

// ValidateFile validates a single YAML file against its schema and returns
// all validation errors. Useful for batch validation from CLI commands.
func ValidateFile(filePath, schemasDir string) ([]schema.ValidationError, error) {
	filename := filepath.Base(filePath)
	schemaName, ok := schema.SchemaForFile(filename)
	if !ok {
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
	switch s.TopType {
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
	default:
		return nil, fmt.Errorf("unsupported schema type: %s", s.TopType)
	}

	return schema.Validate(s, data), nil
}
