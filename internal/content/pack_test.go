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

	pack, err := content.LoadPack(dir, "")
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

	pack, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "abap", pack.ID)
	assert.Empty(t, pack.ContextMD)
	assert.Empty(t, pack.Tips)
}

func TestLoadPack_LocaleContextFile(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: test\nname: Test Pack\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.md"), []byte("English context"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.de.md"), []byte("German context"), 0644))

	// German: locale file used
	p, err := content.LoadPack(dir, "de")
	require.NoError(t, err)
	assert.Equal(t, "German context", p.ContextMD)

	// French: no locale file, falls back to base
	p, err = content.LoadPack(dir, "fr")
	require.NoError(t, err)
	assert.Equal(t, "English context", p.ContextMD)

	// Empty lang: base file used
	p, err = content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "English context", p.ContextMD)

	// lang="en": base file used (no context.en.md attempted)
	p, err = content.LoadPack(dir, "en")
	require.NoError(t, err)
	assert.Equal(t, "English context", p.ContextMD)
}

func TestLoadPack_LocaleTipsFile(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: test\nname: Test Pack\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tips.md"), []byte("## English tip\nEnglish content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tips.de.md"), []byte("## German tip\nGerman content"), 0644))

	// German: locale tips file used
	p, err := content.LoadPack(dir, "de")
	require.NoError(t, err)
	require.Len(t, p.Tips, 1)
	assert.Equal(t, "German tip", p.Tips[0].Title)

	// English: base tips file used
	p, err = content.LoadPack(dir, "")
	require.NoError(t, err)
	require.Len(t, p.Tips, 1)
	assert.Equal(t, "English tip", p.Tips[0].Title)
}

func TestLoadPack_LocaleMetadata(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: test
name: Test Pack
description: A test pack
tags: []
profiles: []
weight: 0
locales:
  de:
    name: Testpaket
    description: Ein Testpaket
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	// German locale
	p, err := content.LoadPack(dir, "de")
	require.NoError(t, err)
	assert.Equal(t, "Testpaket", p.Name)
	assert.Equal(t, "Ein Testpaket", p.Description)

	// English (no locale block)
	p, err = content.LoadPack(dir, "en")
	require.NoError(t, err)
	assert.Equal(t, "Test Pack", p.Name)
	assert.Equal(t, "A test pack", p.Description)
}

func TestLoadPack_MalformedLocales(t *testing.T) {
	dir := t.TempDir()
	// locales value is a string instead of a map — invalid YAML for the field
	yaml := "id: test\nname: Test\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\nlocales: not-a-map\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	_, err := content.LoadPack(dir, "de")
	assert.Error(t, err, "malformed locales block should return an error")
}

func TestLoadPack_PrefersExpandedOverBase(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.md"), []byte("static content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.expanded.md"), []byte("expanded content"), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "expanded content", p.ContextMD)
}

func TestLoadPack_FallsBackToBaseWhenNoExpanded(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.md"), []byte("static content"), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "static content", p.ContextMD)
}

func TestLoadPack_LocaleBeatsExpanded(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.md"), []byte("base"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.de.md"), []byte("german"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.expanded.md"), []byte("expanded"), 0644))

	// German locale wins over expanded
	p, err := content.LoadPack(dir, "de")
	require.NoError(t, err)
	assert.Equal(t, "german", p.ContextMD)

	// No locale → expanded wins
	p, err = content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "expanded", p.ContextMD)

	// "en" also skips locale branch → expanded wins
	p, err = content.LoadPack(dir, "en")
	require.NoError(t, err)
	assert.Equal(t, "expanded", p.ContextMD)
}

func TestLoadPack_OverlapsField(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: btp-core
name: BTP Core
description: BTP basics
tags: []
profiles: []
weight: 80
overlaps:
  - cap
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"cap"}, p.Overlaps)
}

func TestLoadPack_NoOverlaps(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Empty(t, p.Overlaps)
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
