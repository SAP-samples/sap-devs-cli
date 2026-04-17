package adapter_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
)

func TestWriteHookConfig_CreatesFileWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	err := adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false)
	require.NoError(t, err)
	installed, err := adapter.HookConfigInstalled(path, "hooks.SessionStart", "sap-devs tip --markdown")
	require.NoError(t, err)
	assert.True(t, installed)
}

func TestWriteHookConfig_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))
	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var root map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &root))
	hooks := root["hooks"].(map[string]interface{})
	entries := hooks["SessionStart"].([]interface{})
	assert.Len(t, entries, 1)
}

func TestWriteHookConfig_PreservesExistingKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	existing := `{"mcpServers":{"existing":{"command":"npx","args":[]}}}`
	require.NoError(t, os.WriteFile(path, []byte(existing), 0644))

	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var root map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &root))
	assert.NotNil(t, root["mcpServers"], "existing key must be preserved")
	assert.NotNil(t, root["hooks"], "hooks key must be added")
}

func TestRemoveHookConfig_RemovesEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))

	require.NoError(t, adapter.RemoveHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))

	installed, err := adapter.HookConfigInstalled(path, "hooks.SessionStart", "sap-devs tip --markdown")
	require.NoError(t, err)
	assert.False(t, installed)
}

func TestRemoveHookConfig_CleansUpEmptyArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))
	require.NoError(t, adapter.RemoveHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))

	installed, err := adapter.HookConfigInstalled(path, "hooks.SessionStart", "sap-devs tip --markdown")
	require.NoError(t, err)
	assert.False(t, installed, "command must not be present after removal")

	// The hooks key should be completely removed when the array is empty
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var root map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &root))
	assert.NotContains(t, root, "hooks", "hooks key must be removed when array is empty")
}

func TestRemoveHookConfig_NoopWhenNotInstalled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	err := adapter.RemoveHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false)
	assert.NoError(t, err)
}

func TestHookConfigInstalled_TrueWhenInstalled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, adapter.WriteHookConfig(path, "hooks.SessionStart", "sap-devs tip --markdown", false))
	installed, err := adapter.HookConfigInstalled(path, "hooks.SessionStart", "sap-devs tip --markdown")
	require.NoError(t, err)
	assert.True(t, installed)
}

func TestHookConfigInstalled_FalseWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	installed, err := adapter.HookConfigInstalled(path, "hooks.SessionStart", "sap-devs tip --markdown")
	require.NoError(t, err)
	assert.False(t, installed)
}
