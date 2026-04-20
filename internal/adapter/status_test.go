package adapter_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/adapter"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func TestEstimateTokens_Empty(t *testing.T) {
	assert.Equal(t, 0, adapter.EstimateTokens(""))
}

func TestEstimateTokens_KnownString(t *testing.T) {
	// "hello world foo bar" = 4 words → 4 * 13 / 10 = 5
	assert.Equal(t, 5, adapter.EstimateTokens("hello world foo bar"))
}

func TestScanOtherSections_Empty(t *testing.T) {
	result := adapter.ScanOtherSections("")
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestScanOtherSections_IgnoresSapDevs(t *testing.T) {
	content := "<!-- sap-devs:start:SAP Dev -->\nhello\n<!-- sap-devs:end:SAP Dev -->\n"
	result := adapter.ScanOtherSections(content)
	assert.Empty(t, result)
}

func TestScanOtherSections_OneMatch(t *testing.T) {
	content := "<!-- cursor:start:Rules -->\nsome cursor rules here\n<!-- cursor:end:Rules -->\n"
	result := adapter.ScanOtherSections(content)
	assert.Len(t, result, 1)
	assert.Equal(t, "cursor", result[0].Name)
	assert.Greater(t, result[0].Tokens, 0)
}

func TestScanOtherSections_MultipleTools(t *testing.T) {
	content := "<!-- cursor:start:Rules -->\ncursor rules\n<!-- cursor:end:Rules -->\n<!-- copilot:start:Instructions -->\ncopilot stuff\n<!-- copilot:end:Instructions -->\n"
	result := adapter.ScanOtherSections(content)
	assert.Len(t, result, 2)
	names := []string{result[0].Name, result[1].Name}
	assert.Contains(t, names, "cursor")
	assert.Contains(t, names, "copilot")
}

func makePackWithContent(id, contextMD string) *content.Pack {
	return &content.Pack{
		ID:      id,
		Name:    id,
		Context: content.VerbositySections{Core: contextMD},
	}
}

// writeSectionFile writes a file containing a sap-devs fenced section.
func writeSectionFile(t *testing.T, path, section, inner string) {
	t.Helper()
	data := fmt.Sprintf("<!-- sap-devs:start:%s -->\n%s\n<!-- sap-devs:end:%s -->\n", section, inner, section)
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))
}

func TestStatus_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md") // does not exist

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, targetFile, rows[0].TargetPath)
	assert.False(t, rows[0].FileExists)
	assert.False(t, rows[0].Injected)
	assert.False(t, rows[0].Stale)
}

func TestStatus_NotInjected(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(targetFile, []byte("# No SAP markers here\n"), 0644))

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].FileExists)
	assert.False(t, rows[0].Injected)
	assert.Greater(t, rows[0].FileSizeBytes, 0)
}

func TestStatus_Orphaned(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")
	// Start marker only, no end marker
	require.NoError(t, os.WriteFile(targetFile, []byte("<!-- sap-devs:start:SAP Dev -->\norphan\n"), 0644))

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].FileExists)
	assert.False(t, rows[0].Injected)
	assert.True(t, rows[0].Orphaned)
}

func TestStatus_Current(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")

	pack := makePackWithContent("test-pack", "## CAP\nUse CDS for data models.\n")
	packs := []*content.Pack{pack}

	a := adapter.Adapter{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}

	eng := adapter.NewEngine([]adapter.Adapter{a}, packs, nil, adapter.Options{Scope: "global"})

	// Write the file with exactly what renderSectionContent would produce
	rendered := eng.RenderSectionContentForTest(a)
	writeSectionFile(t, targetFile, "SAP Dev", rendered)

	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].Injected)
	assert.False(t, rows[0].Stale)
}

func TestStatus_Stale(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")
	writeSectionFile(t, targetFile, "SAP Dev", "old outdated content")

	pack := makePackWithContent("test-pack", "## CAP\nNew content.\n")
	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, []*content.Pack{pack}, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].Injected)
	assert.True(t, rows[0].Stale)
}

func TestStatus_ScopeFilter(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "project", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	// Engine scope is global — project target must be skipped
	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestStatus_ToolFilter(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "A.md")
	fileB := filepath.Join(dir, "B.md")

	adapters := []adapter.Adapter{
		{
			ID:   "tool-a",
			Name: "Tool A",
			Type: "file-inject",
			Targets: []adapter.Target{
				{Scope: "global", Path: fileA, Mode: "replace-section", Section: "SAP Dev"},
			},
		},
		{
			ID:   "tool-b",
			Name: "Tool B",
			Type: "file-inject",
			Targets: []adapter.Target{
				{Scope: "global", Path: fileB, Mode: "replace-section", Section: "SAP Dev"},
			},
		},
	}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global", ToolFilter: "tool-a"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "tool-a", rows[0].AdapterID)
}

func TestStatus_ReplaceFile(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "context.md")
	require.NoError(t, os.WriteFile(targetFile, []byte("preamble\ncontent"), 0644))

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-file"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].FileExists)
	assert.True(t, rows[0].Injected) // replace-file: existing file = injected
}

func TestStatus_OtherSections(t *testing.T) {
	dir := t.TempDir()
	targetFile := filepath.Join(dir, "CLAUDE.md")
	data := `<!-- sap-devs:start:SAP Dev -->
sap content
<!-- sap-devs:end:SAP Dev -->
<!-- cursor:start:Rules -->
cursor rules
<!-- cursor:end:Rules -->
`
	require.NoError(t, os.WriteFile(targetFile, []byte(data), 0644))

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: targetFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Len(t, rows[0].OtherSections, 1)
	assert.Equal(t, "cursor", rows[0].OtherSections[0].Name)
}

func TestStatus_SkipsNonFileInject(t *testing.T) {
	adapters := []adapter.Adapter{{
		ID:   "chatgpt",
		Name: "ChatGPT",
		Type: "clipboard-export",
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestStatus_ErrorContinues(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "A.md")
	fileB := filepath.Join(dir, "B.md")
	require.NoError(t, os.WriteFile(fileA, []byte("# hello"), 0644))
	// fileB does not exist — will yield FileExists=false but no error

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: fileA, Mode: "replace-section", Section: "SAP Dev"},
			{Scope: "global", Path: fileB, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err) // not-exist is not an error
	assert.Len(t, rows, 2)
	assert.True(t, rows[0].FileExists)
	assert.False(t, rows[1].FileExists)
}

func TestStatus_MultipleTargets_TwoRows(t *testing.T) {
	dir := t.TempDir()
	globalFile := filepath.Join(dir, "global.md")
	projectFile := filepath.Join(dir, "project.md")

	adapters := []adapter.Adapter{{
		ID:   "test-tool",
		Name: "Test Tool",
		Type: "file-inject",
		Targets: []adapter.Target{
			{Scope: "global", Path: globalFile, Mode: "replace-section", Section: "SAP Dev"},
			{Scope: "project", Path: projectFile, Mode: "replace-section", Section: "SAP Dev"},
		},
	}}

	// global scope: only global target → 1 row
	eng := adapter.NewEngine(adapters, nil, nil, adapter.Options{Scope: "global"})
	rows, err := eng.Status()
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "global", rows[0].Scope)
}
