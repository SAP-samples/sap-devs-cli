package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func mcpFixturePacks() []*content.Pack {
	return []*content.Pack{
		{
			ID: "cap",
			MCPServers: []content.MCPServer{
				{ID: "cap-mcp", Name: "CAP MCP Server", Hosts: []string{"claude-code"}, PackID: "cap"},
			},
		},
		{
			ID: "abap",
			MCPServers: []content.MCPServer{
				{ID: "abap-mcp", Name: "ABAP MCP Server", Hosts: []string{"cursor"}, PackID: "abap"},
			},
		},
	}
}

func TestFlattenMCPServers(t *testing.T) {
	got := content.FlattenMCPServers(mcpFixturePacks())
	require.Len(t, got, 2)
	assert.Equal(t, "cap-mcp", got[0].ID)
	assert.Equal(t, "cap", got[0].PackID)
	assert.Equal(t, "abap-mcp", got[1].ID)
	assert.Equal(t, "abap", got[1].PackID)
}

func TestFlattenMCPServers_Empty(t *testing.T) {
	got := content.FlattenMCPServers([]*content.Pack{{ID: "empty"}})
	assert.Empty(t, got)
}

func TestFindMCPServer_Found(t *testing.T) {
	got := content.FindMCPServer(mcpFixturePacks(), "abap-mcp")
	require.NotNil(t, got)
	assert.Equal(t, "ABAP MCP Server", got.Name)
}

func TestFindMCPServer_NotFound(t *testing.T) {
	got := content.FindMCPServer(mcpFixturePacks(), "nonexistent")
	assert.Nil(t, got)
}

func TestLoadPackSetsMCPPackID(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(`
id: mypak
name: My Pack
description: Test pack
`), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "mcp.yaml"), []byte(`
- id: mypak-mcp
  name: My MCP Server
  install:
    command: npx
    args: ["-y", "my-mcp"]
  hosts: [claude-code]
`), 0644))

	pack, err := content.LoadPack(dir)
	require.NoError(t, err)
	require.Len(t, pack.MCPServers, 1)
	assert.Equal(t, "mypak", pack.MCPServers[0].PackID)
}
