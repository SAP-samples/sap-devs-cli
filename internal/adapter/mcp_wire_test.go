package adapter_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestWriteMCPConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	server := content.MCPServer{
		ID:          "cap-mcp",
		Name:        "CAP MCP Server",
		Description: "CAP tools",
		Install: content.MCPInstall{
			Command: "npx",
			Args:    []string{"-y", "@sap/cap-mcp-server"},
		},
	}

	err := adapter.WriteMCPConfig(settingsPath, "mcpServers", server, false)
	require.NoError(t, err)

	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	servers, ok := result["mcpServers"].(map[string]interface{})
	require.True(t, ok)
	entry, ok := servers["cap-mcp"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "npx", entry["command"])
}

func TestWriteMCPConfig_Idempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Write existing settings
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"theme":"dark","mcpServers":{}}`), 0644))

	server := content.MCPServer{
		ID:      "cap-mcp",
		Install: content.MCPInstall{Command: "npx", Args: []string{"-y", "@sap/cap-mcp-server"}},
	}

	require.NoError(t, adapter.WriteMCPConfig(settingsPath, "mcpServers", server, false))
	require.NoError(t, adapter.WriteMCPConfig(settingsPath, "mcpServers", server, false))

	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	// Existing key preserved
	assert.Equal(t, "dark", result["theme"])
	// Server is present once
	servers, ok := result["mcpServers"].(map[string]interface{})
	require.True(t, ok, "mcpServers should be a JSON object")
	assert.Len(t, servers, 1)
}

func TestWriteMCPConfig_DryRun(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	server := content.MCPServer{ID: "cap-mcp", Install: content.MCPInstall{Command: "npx"}}
	require.NoError(t, adapter.WriteMCPConfig(settingsPath, "mcpServers", server, true))

	_, err := os.Stat(settingsPath)
	assert.True(t, os.IsNotExist(err))
}

func TestWriteMCPConfig_BadKeyType(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"mcpServers":"not-an-object"}`), 0644))

	server := content.MCPServer{ID: "cap-mcp", Install: content.MCPInstall{Command: "npx"}}
	err := adapter.WriteMCPConfig(settingsPath, "mcpServers", server, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a JSON object")
}

func TestReadMCPConfig_Missing(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	m, err := adapter.ReadMCPConfig(settingsPath, "mcpServers")
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestReadMCPConfig_Present(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(
		`{"mcpServers":{"cap-mcp":{"command":"npx","args":["-y","@sap/cap-mcp-server"]}}}`,
	), 0644))

	m, err := adapter.ReadMCPConfig(settingsPath, "mcpServers")
	require.NoError(t, err)
	require.Len(t, m, 1)
	_, ok := m["cap-mcp"]
	assert.True(t, ok)
}

func TestReadMCPConfig_KeyAbsent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"theme":"dark"}`), 0644))

	m, err := adapter.ReadMCPConfig(settingsPath, "mcpServers")
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestReadMCPConfig_BadKeyType(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"mcpServers":"not-an-object"}`), 0644))

	_, err := adapter.ReadMCPConfig(settingsPath, "mcpServers")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a JSON object")
}
