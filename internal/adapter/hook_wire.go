package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteHookConfig adds a hook command entry to the tool's settings JSON.
// key is a dot-separated JSON path, e.g. "hooks.SessionStart".
// Idempotent: if the command is already present, it is a no-op.
// When dryRun is true, prints what would happen and returns without writing.
func WriteHookConfig(settingsPath, key, command string, dryRun bool) error {
	if dryRun {
		fmt.Printf("[dry-run] would add hook %q to %s[%s]\n", command, settingsPath, key)
		return nil
	}

	root, err := readJSONFile(settingsPath)
	if err != nil {
		return err
	}

	arr := navigateToArray(root, key)
	if hookCommandPresent(arr, command) {
		return nil // idempotent
	}

	entry := map[string]interface{}{
		"matcher": "",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": ResolveSelfCommand(command),
			},
		},
	}
	arr = append(arr, entry)
	setNestedArray(root, key, arr)

	return writeJSONFile(settingsPath, root)
}

// RemoveHookConfig removes a hook command entry from the tool's settings JSON.
// No-op if the file does not exist or the entry is not present.
// When dryRun is true, prints what would happen and returns without writing.
func RemoveHookConfig(settingsPath, key, command string, dryRun bool) error {
	if dryRun {
		fmt.Printf("[dry-run] would remove hook %q from %s[%s]\n", command, settingsPath, key)
		return nil
	}

	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse %s: %w", settingsPath, err)
	}

	arr := lookupArray(root, key)
	if len(arr) == 0 {
		return nil
	}

	var filtered []interface{}
	for _, item := range arr {
		if !entryMatchesCommand(item, command) {
			filtered = append(filtered, item)
		}
	}

	if len(filtered) == 0 {
		deleteNestedKey(root, key)
	} else {
		setNestedArray(root, key, filtered)
	}

	return writeJSONFile(settingsPath, root)
}

// HookConfigInstalled reports whether the command appears in the settings JSON.
func HookConfigInstalled(settingsPath, key, command string) (bool, error) {
	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("parse %s: %w", settingsPath, err)
	}
	return hookCommandPresent(lookupArray(root, key), command), nil
}

// --- private helpers ---

func readJSONFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]interface{}), nil
	}
	if err != nil {
		return nil, err
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return root, nil
}

func writeJSONFile(path string, root map[string]interface{}) error {
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// lookupArray walks the dot-separated key path without creating intermediate nodes.
// Returns nil if any segment of the path does not exist.
func lookupArray(root map[string]interface{}, key string) []interface{} {
	parts := strings.SplitN(key, ".", 2)
	v, ok := root[parts[0]]
	if !ok || v == nil {
		return nil
	}
	if len(parts) == 1 {
		arr, _ := v.([]interface{})
		return arr
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return lookupArray(m, parts[1])
}

// navigateToArray walks the dot-separated key path and returns the array at
// the leaf, creating intermediate objects as needed. Returns nil if not found.
func navigateToArray(root map[string]interface{}, key string) []interface{} {
	parts := strings.SplitN(key, ".", 2)
	head := parts[0]
	if len(parts) == 1 {
		if v, ok := root[head]; ok {
			if arr, ok := v.([]interface{}); ok {
				return arr
			}
		}
		return nil
	}
	sub, ok := root[head]
	if !ok || sub == nil {
		sub = make(map[string]interface{})
		root[head] = sub
	}
	m, ok := sub.(map[string]interface{})
	if !ok {
		return nil
	}
	return navigateToArray(m, parts[1])
}

// setNestedArray walks the dot-separated key path and sets the array at the leaf.
func setNestedArray(root map[string]interface{}, key string, arr []interface{}) {
	parts := strings.SplitN(key, ".", 2)
	head := parts[0]
	if len(parts) == 1 {
		root[head] = arr
		return
	}
	sub, ok := root[head]
	if !ok || sub == nil {
		sub = make(map[string]interface{})
		root[head] = sub
	}
	m, ok := sub.(map[string]interface{})
	if !ok {
		m = make(map[string]interface{})
		root[head] = m
	}
	setNestedArray(m, parts[1], arr)
}

// deleteNestedKey removes the leaf key at the dot-separated path.
func deleteNestedKey(root map[string]interface{}, key string) {
	parts := strings.SplitN(key, ".", 2)
	head := parts[0]
	if len(parts) == 1 {
		delete(root, head)
		return
	}
	sub, ok := root[head]
	if !ok {
		return
	}
	m, ok := sub.(map[string]interface{})
	if !ok {
		return
	}
	deleteNestedKey(m, parts[1])
	if len(m) == 0 {
		delete(root, head)
	}
}

// hookCommandPresent returns true if any entry in arr contains a hook with the given command.
func hookCommandPresent(arr []interface{}, command string) bool {
	for _, item := range arr {
		if entryMatchesCommand(item, command) {
			return true
		}
	}
	return false
}

// entryMatchesCommand returns true if the matcher-group entry contains a hook with the given command.
// It compares against both the raw command and the resolved form (absolute path)
// since WriteHookConfig stores commands in resolved form.
func entryMatchesCommand(item interface{}, command string) bool {
	m, ok := item.(map[string]interface{})
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]interface{})
	if !ok {
		return false
	}
	resolved := ResolveSelfCommand(command)
	for _, h := range hooks {
		hm, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if cmd == command || cmd == resolved {
			return true
		}
	}
	return false
}
