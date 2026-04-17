// cmd/inject_test.go
package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// TestInjectEndToEnd tests ReplaceSection → file content round-trip
// without invoking the full cobra command (avoids XDG path dependencies in CI).
func TestInjectEndToEnd(t *testing.T) {
	dir := t.TempDir()
	claudeMD := filepath.Join(dir, "CLAUDE.md")

	// Simulate existing CLAUDE.md
	require.NoError(t, os.WriteFile(claudeMD, []byte("# My Project\n\nMy notes.\n"), 0644))

	// Build packs and run engine with a file-inject adapter targeting our temp file
	packs := []*content.Pack{
		{ID: "cap", Name: "CAP", ContextMD: "## SAP CAP\n\nUse @sap/cds for data models."},
	}

	// Run engine with a file-inject adapter targeting our temp file
	adapters := []adapter.Adapter{
		{
			ID:   "claude-code",
			Type: "file-inject",
			Targets: []adapter.Target{
				{Scope: "global", Path: claudeMD, Mode: "replace-section", Section: "SAP Developer Context"},
			},
		},
	}
	engine := adapter.NewEngine(adapters, packs, nil, adapter.Options{Scope: "global"})
	res := engine.Run()
	require.NoError(t, res.Err)

	// Verify output
	data, err := os.ReadFile(claudeMD)
	require.NoError(t, err)
	result := string(data)

	assert.Contains(t, result, "# My Project")
	assert.Contains(t, result, "My notes.")
	assert.Contains(t, result, "<!-- sap-devs:start:SAP Developer Context -->")
	assert.Contains(t, result, "Use @sap/cds for data models.")
	assert.Contains(t, result, "<!-- sap-devs:end:SAP Developer Context -->")

	// Second run — idempotent
	res2 := engine.Run()
	require.NoError(t, res2.Err)
	data2, err := os.ReadFile(claudeMD)
	require.NoError(t, err)
	result2 := string(data2)
	assert.Equal(t, 1, strings.Count(result2, "<!-- sap-devs:start:SAP Developer Context -->"))
	assert.Contains(t, result2, "# My Project")
	assert.Contains(t, result2, "My notes.")
}
