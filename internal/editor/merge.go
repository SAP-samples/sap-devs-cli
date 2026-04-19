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
	IsOverride bool // true when this item overrides an item from an earlier layer
}

// LoadMergedItems loads items from all layers for a given pack and file,
// merging by "id" or "name" key. Later layers override earlier ones.
func LoadMergedItems(cwd, packID, filename string) ([]MergedItem, error) {
	layers := AllLayers(cwd)
	seen := make(map[string]int) // id -> index in result
	var result []MergedItem

	for _, l := range layers {
		packDir := filepath.Join(l.Dir, packID)
		filePath := filepath.Join(packDir, filename)

		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // file doesn't exist in this layer
		}

		var items []map[string]any
		if err := yaml.Unmarshal(data, &items); err != nil {
			continue
		}

		for _, item := range items {
			id := itemKey(item)
			if id == "" {
				// No identifiable key; just append
				result = append(result, MergedItem{
					Data:  item,
					Layer: l.Layer,
				})
				continue
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

// itemKey extracts the merge key ("id" or "name") from an item.
func itemKey(item map[string]any) string {
	if id, ok := item["id"].(string); ok && id != "" {
		return id
	}
	if name, ok := item["name"].(string); ok && name != "" {
		return name
	}
	return ""
}

// LoadSingleObject loads a single-object YAML file (e.g. pack.yaml).
// Returns an empty map if the file does not exist.
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
// Only items belonging to targetLayer are included in the output file.
func SaveItems(filePath string, items []MergedItem, targetLayer Layer) error {
	var toSave []map[string]any
	for _, item := range items {
		if item.Layer == targetLayer {
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

// ColumnsForSchema picks the first 4 string-typed fields from the spec,
// excluding verbose fields (description, pattern, cause, fix) that would
// clutter a narrow list view.
func ColumnsForSchema(spec *schema.ObjectSpec) []string {
	exclude := map[string]bool{
		"description": true,
		"pattern":     true,
		"cause":       true,
		"fix":         true,
	}
	var cols []string
	for _, f := range spec.Fields {
		if f.Type == "string" && !exclude[f.Key] {
			cols = append(cols, f.Key)
			if len(cols) >= 4 {
				break
			}
		}
	}
	return cols
}
