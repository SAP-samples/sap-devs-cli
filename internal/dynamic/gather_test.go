package dynamic_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/dynamic"
)

// --- Project type detection ---

func TestGatherDynamic_ProjectType_CdsrcJson(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".cdsrc.json"), []byte(`{}`), 0600))
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
	assert.Equal(t, "CAP (Node.js)", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_PackageJsonWithCDS(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies":{"@sap/cds":"^7.0.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0600))
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
	assert.Equal(t, "CAP (Node.js)", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_PackageJsonWithCDSInDevDeps(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"devDependencies":{"@sap/cds":"^7.0.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0600))
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
	assert.Equal(t, "CAP (Node.js)", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_MtaYaml(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mta.yaml"), []byte(`ID: myapp`), 0600))
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
	assert.Equal(t, "Multi-target Application (MTA)", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_XsAppJson(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "xs-app.json"), []byte(`{}`), 0600))
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
	assert.Equal(t, "Fiori / BAS app", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_PomXmlWithCAPJava(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(`<project><groupId>com.sap.cds</groupId></project>`), 0600))
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
	assert.Equal(t, "CAP (Java)", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_PlainPackageJson(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"myapp"}`), 0600))
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
	assert.Equal(t, "Node.js", ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_EmptyWhenNoFiles(t *testing.T) {
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: t.TempDir()})
	assert.Empty(t, ctx.ProjectType)
}

func TestGatherDynamic_ProjectType_CdsrcTakesPriorityOverMtaYaml(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".cdsrc.json"), []byte(`{}`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mta.yaml"), []byte(`ID: myapp`), 0600))
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: dir})
	assert.Equal(t, "CAP (Node.js)", ctx.ProjectType)
}

// --- Pack freshness ---

func TestGatherDynamic_LastSynced_NilWhenNoStateFile(t *testing.T) {
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{SyncStateDir: t.TempDir()})
	assert.Nil(t, ctx.LastSynced)
}

func TestGatherDynamic_LastSynced_ReturnsMostRecent(t *testing.T) {
	dir := t.TempDir()
	older := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	newer := time.Now().Add(-30 * time.Minute).UTC().Truncate(time.Second)
	state := map[string]interface{}{
		"version": 1,
		"categories": map[string]string{
			"tips":    older.Format(time.RFC3339),
			"context": newer.Format(time.RFC3339),
		},
		"packs":   map[string]interface{}{},
		"markers": map[string]interface{}{},
	}
	data, _ := json.Marshal(state)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sync-state.json"), data, 0600))
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{SyncStateDir: dir})
	require.NotNil(t, ctx.LastSynced)
	assert.Equal(t, newer, ctx.LastSynced.UTC().Truncate(time.Second))
}

// --- CLI self-awareness ---

func TestGatherDynamic_CLIVersion_PassedThrough(t *testing.T) {
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CLIVersion: "2.0.0"})
	assert.Equal(t, "2.0.0", ctx.CLIVersion)
}

func TestGatherDynamic_ActiveProfile_UsesName(t *testing.T) {
	p := &content.Profile{ID: "cap-developer", Name: "CAP Developer"}
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Profile: p})
	assert.Equal(t, "CAP Developer", ctx.ActiveProfile)
}

func TestGatherDynamic_ActiveProfile_FallsBackToID(t *testing.T) {
	p := &content.Profile{ID: "cap-developer", Name: ""}
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Profile: p})
	assert.Equal(t, "cap-developer", ctx.ActiveProfile)
}

func TestGatherDynamic_LoadedPackIDs_FromPacks(t *testing.T) {
	packs := []*content.Pack{{ID: "cap"}, {ID: "btp"}}
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Packs: packs})
	assert.Equal(t, []string{"cap", "btp"}, ctx.LoadedPackIDs)
}

func TestGatherDynamic_Commands_PassedThrough(t *testing.T) {
	cmds := []content.CommandInfo{{Name: "inject", Short: "Push SAP context"}}
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Commands: cmds})
	require.Len(t, ctx.Commands, 1)
	assert.Equal(t, "inject", ctx.Commands[0].Name)
}

// --- Wired MCP servers ---

func TestGatherDynamic_WiredMCP_OnlySAPServersReturned(t *testing.T) {
	dir := t.TempDir()
	mcpConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"sap-cap-mcp":    map[string]interface{}{},
			"some-other-mcp": map[string]interface{}{},
		},
	}
	data, _ := json.Marshal(mcpConfig)
	cfgPath := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(cfgPath, data, 0600))

	adapters := []adapter.Adapter{{
		ID:   "claude-code",
		Name: "Claude Code",
		MCPConfig: &adapter.MCPConfig{
			Path:   cfgPath,
			Format: "json",
			Key:    "mcpServers",
		},
	}}
	packs := []*content.Pack{{
		ID:         "cap",
		MCPServers: []content.MCPServer{{ID: "sap-cap-mcp"}},
	}}

	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Adapters: adapters, Packs: packs})
	require.Len(t, ctx.WiredMCPServers, 1)
	assert.Equal(t, "Claude Code", ctx.WiredMCPServers[0].AdapterName)
	assert.Equal(t, []string{"sap-cap-mcp"}, ctx.WiredMCPServers[0].ServerIDs)
}

func TestGatherDynamic_WiredMCP_EmptyWhenConfigFileMissing(t *testing.T) {
	a := adapter.Adapter{
		Name: "Claude Code",
		MCPConfig: &adapter.MCPConfig{
			Path: "/nonexistent/path/mcp.json",
			Key:  "mcpServers",
		},
	}
	packs := []*content.Pack{{
		ID:         "cap",
		MCPServers: []content.MCPServer{{ID: "sap-cap-mcp"}},
	}}
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{Adapters: []adapter.Adapter{a}, Packs: packs})
	assert.Empty(t, ctx.WiredMCPServers)
}

// --- Error resilience ---

func TestGatherDynamic_NeverPanics_AllZeroOpts(t *testing.T) {
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{})
	require.NotNil(t, ctx)
}

func TestGatherDynamic_NeverPanics_MissingCWD(t *testing.T) {
	ctx := dynamic.GatherDynamic(dynamic.GatherOpts{CWD: "/nonexistent/dir/xyz"})
	require.NotNil(t, ctx)
	assert.Empty(t, ctx.ProjectType)
}
