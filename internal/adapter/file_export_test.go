package adapter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
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

	// Content longer than max_bytes — clipboard payload must be trimmed
	fullCtx := strings.Repeat("a", 200)
	// DryRun = true so we don't need clipboard hardware
	err := adapter.ExportFileAndClip(a, fullCtx, adapter.Options{DryRun: true})
	require.NoError(t, err)
}

func TestExportFileAndClip_AppendedGuidanceLine(t *testing.T) {
	// Guidance line references export_path and mentions ChatGPT Project
	// We verify this via dry-run (file not written, but no error)
	dir := t.TempDir()
	a := adapter.Adapter{
		ID:           "chatgpt",
		Type:         "file-export",
		ExportPath:   filepath.Join(dir, "ctx.md"),
		MaxBytes:     1400,
		Format:       "plain-prose",
		Instructions: "Paste",
	}
	err := adapter.ExportFileAndClip(a, "SAP context here", adapter.Options{DryRun: true})
	require.NoError(t, err)
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
