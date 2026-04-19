package editor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"charm.land/huh/v2"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/schema"
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

	for {
		listMdl := newListModel(items, columns, target, s)
		p := tea.NewProgram(listMdl, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			return err
		}

		result := finalModel.(listModel)

		// Handle delete.
		if result.deleteIdx >= 0 {
			items = append(items[:result.deleteIdx], items[result.deleteIdx+1:]...)
			continue
		}

		// Handle add new item.
		if result.addNew {
			newItem := make(map[string]any)
			if err := editItem(s.ItemSpec, newItem); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue // user cancelled; return to list
				}
				continue
			}
			items = append(items, MergedItem{
				Data:  newItem,
				Layer: target.Layer,
			})
			continue
		}

		// Handle edit existing item.
		if result.editIndex >= 0 {
			item := &items[result.editIndex]
			if item.Layer != target.Layer {
				// Clone inherited item into the target layer (override).
				cloned := make(map[string]any)
				for k, v := range item.Data {
					cloned[k] = v
				}
				item.Data = cloned
				item.Layer = target.Layer
				item.IsOverride = true
			}
			if err := editItem(s.ItemSpec, item.Data); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				continue
			}
			continue
		}

		// Handle save+quit.
		if result.save {
			return SaveItems(target.FilePath, items, target.Layer)
		}

		// Quit without saving.
		return nil
	}
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
