package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestLoadPack_ParsesAllFiles(t *testing.T) {
	dir := makeTempPack(t, "cap", `
id: cap
name: SAP CAP
description: Cloud Application Programming Model
tags: [cloud, node, java]
profiles: [cap-developer]
weight: 100
`, "# CAP Context\nUse CDS for data modelling.", `
- id: cap/docs
  title: CAP Docs
  url: https://cap.cloud.sap
  type: official-docs
`, "## Tip One\nTags: cap,nodejs\nUse cds watch for local development.")

	pack, err := content.LoadPack(dir)
	require.NoError(t, err)
	assert.Equal(t, "cap", pack.ID)
	assert.Equal(t, "SAP CAP", pack.Name)
	assert.Contains(t, pack.ContextMD, "CDS")
	assert.Len(t, pack.Resources, 1)
	assert.Equal(t, "cap/docs", pack.Resources[0].ID)
	assert.Len(t, pack.Tips, 1)
	assert.Contains(t, pack.Tips[0].Content, "cds watch")
}

func TestLoadPack_MissingOptionalFilesOK(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: abap\nname: ABAP\ndescription: ABAP Cloud\ntags: []\nprofiles: []\nweight: 90\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	pack, err := content.LoadPack(dir)
	require.NoError(t, err)
	assert.Equal(t, "abap", pack.ID)
	assert.Empty(t, pack.ContextMD)
	assert.Empty(t, pack.Tips)
}

func makeTempPack(t *testing.T, id, packYAML, contextMD, resourcesYAML, tipsMD string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"pack.yaml":      packYAML,
		"context.md":     contextMD,
		"resources.yaml": resourcesYAML,
		"tips.md":        tipsMD,
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
	}
	return dir
}
