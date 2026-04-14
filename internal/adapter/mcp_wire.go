package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// WriteMCPConfig merges an MCP server entry into the host's settings JSON file.
// The file is created if it does not exist. Existing keys are preserved.
// When dryRun is true, prints what would happen and returns without writing.
func WriteMCPConfig(settingsPath, key string, server content.MCPServer, dryRun bool) error {
	if dryRun {
		fmt.Printf("[dry-run] would add MCP server %q to %s[%s]\n", server.ID, settingsPath, key)
		return nil
	}

	// Read existing JSON (or start empty)
	var root map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parse %s: %w", settingsPath, err)
		}
	} else if os.IsNotExist(err) {
		root = make(map[string]interface{})
	} else {
		return err
	}

	// Get or create the mcpServers map
	var servers map[string]interface{}
	if v, ok := root[key]; ok && v != nil {
		m, ok := v.(map[string]interface{})
		if !ok {
			return fmt.Errorf("key %q in %s is not a JSON object (got %T); cannot merge", key, settingsPath, v)
		}
		servers = m
	}
	if servers == nil {
		servers = make(map[string]interface{})
	}

	// Build the server entry
	args := server.Install.Args
	if args == nil {
		args = []string{}
	}
	entry := map[string]interface{}{
		"command": server.Install.Command,
		"args":    args,
	}
	servers[server.ID] = entry
	root[key] = servers

	// Write back with indentation
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(settingsPath, out, 0644)
}

// ReadMCPConfig reads the mcpServers map from a JSON settings file.
// Returns an empty map (not an error) if the file does not exist or the key is absent.
// Returns an error if the file exists but cannot be parsed, or if the key is not a JSON object.
func ReadMCPConfig(settingsPath, key string) (map[string]interface{}, error) {
	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", settingsPath, err)
	}
	v, ok := root[key]
	if !ok || v == nil {
		return map[string]interface{}{}, nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("key %q in %s is not a JSON object (got %T); cannot read", key, settingsPath, v)
	}
	return m, nil
}
