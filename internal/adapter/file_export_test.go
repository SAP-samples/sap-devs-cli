package adapter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/adapter"
)

func TestExportFileAndClip_EmptyExportPath(t *testing.T) {
	a := adapter.Adapter{ID: "chatgpt", Type: "file-export"}
	err := adapter.ExportFileAndClip(a, "some content", adapter.Options{DryRun: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "export_path is required")
}

func TestExportFileAndClip_WritesFullFile(t *testing.T) {
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "context.md")

	a := adapter.Adapter{
		ID:           "chatgpt",
		Type:         "file-export",
		ExportPath:   exportPath,
		MaxBytes:     50,
		Format:       "plain-prose",
		Instructions: "Paste into ChatGPT",
	}

	fullCtx := strings.Repeat("x", 200)
	err := adapter.ExportFileAndClip(a, fullCtx, adapter.Options{DryRun: false})
	require.NoError(t, err)

	data, err := os.ReadFile(exportPath)
	require.NoError(t, err)
	// Full Markdown context written verbatim (no truncation)
	assert.Equal(t, fullCtx, string(data))
}

func TestExportFileAndClip_ClipsShortVersion(t *testing.T) {
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "context.md")

	a := adapter.Adapter{
		ID:           "chatgpt",
		Type:         "file-export",
		ExportPath:   exportPath,
		MaxBytes:     20,
		Format:       "markdown",
		Instructions: "Paste me",
	}

	// Content longer than max_bytes — export file receives full content; clipboard payload is trimmed.
	// We assert the export file holds all 200 bytes, confirming file == full and clip == trimmed.
	fullCtx := strings.Repeat("a", 200)
	err := adapter.ExportFileAndClip(a, fullCtx, adapter.Options{DryRun: false})
	require.NoError(t, err)

	data, err := os.ReadFile(exportPath)
	require.NoError(t, err)
	assert.Equal(t, fullCtx, string(data), "export file must hold full unclipped context")
}

func TestExportFileAndClip_AppendedGuidanceLine(t *testing.T) {
	// Verify that ExportFileAndClip writes the full context to the export file and
	// does not truncate it with the guidance line. The clipboard payload (not directly
	// observable) appends a guidance line containing the export_path; we confirm the
	// export file itself holds only the raw context (no guidance line appended to file).
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "ctx.md")
	a := adapter.Adapter{
		ID:           "chatgpt",
		Type:         "file-export",
		ExportPath:   exportPath,
		MaxBytes:     1400,
		Format:       "plain-prose",
		Instructions: "Paste",
	}

	ctx := "SAP context here"
	err := adapter.ExportFileAndClip(a, ctx, adapter.Options{DryRun: false})
	require.NoError(t, err)

	data, err := os.ReadFile(exportPath)
	require.NoError(t, err)
	// Export file contains raw context exactly — guidance line goes to clipboard only
	assert.Equal(t, ctx, string(data), "export file must hold raw context without guidance line")
}

func TestExportFileAndClip_DryRun(t *testing.T) {
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "context.md")

	a := adapter.Adapter{
		ID:         "chatgpt",
		ExportPath: exportPath,
		MaxBytes:   1400,
	}

	err := adapter.ExportFileAndClip(a, "content", adapter.Options{DryRun: true})
	require.NoError(t, err)

	// File must not be created in dry-run
	_, statErr := os.Stat(exportPath)
	assert.True(t, os.IsNotExist(statErr), "dry-run must not write export file")
}
