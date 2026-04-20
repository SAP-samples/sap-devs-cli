package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
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
	assert.Contains(t, pack.Context.Core, "CDS")
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
	assert.Equal(t, content.VerbositySections{}, pack.Context)
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
	assert.Equal(t, "German context", p.Context.Core)

	// French: no locale file, falls back to base
	p, err = content.LoadPack(dir, "fr")
	require.NoError(t, err)
	assert.Equal(t, "English context", p.Context.Core)

	// Empty lang: base file used
	p, err = content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "English context", p.Context.Core)

	// lang="en": base file used (no context.en.md attempted)
	p, err = content.LoadPack(dir, "en")
	require.NoError(t, err)
	assert.Equal(t, "English context", p.Context.Core)
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
	assert.Equal(t, "expanded content", p.Context.Core)
}

func TestLoadPack_FallsBackToBaseWhenNoExpanded(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: Test\ntags: []\nprofiles: []\nweight: 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.md"), []byte("static content"), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "static content", p.Context.Core)
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
	assert.Equal(t, "german", p.Context.Core)

	// No locale → expanded wins
	p, err = content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "expanded", p.Context.Core)

	// "en" also skips locale branch → expanded wins
	p, err = content.LoadPack(dir, "en")
	require.NoError(t, err)
	assert.Equal(t, "expanded", p.Context.Core)
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

func TestLoadPack_AdditiveFields(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: desc\ntags: []\nprofiles: []\nweight: 0\nadditive: true\nadditive_position: before\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	pack, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.True(t, pack.Additive)
	assert.Equal(t, "before", pack.AdditivePosition)
}

func TestLoadPack_AdditiveDefaults(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: desc\ntags: []\nprofiles: []\nweight: 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	pack, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.False(t, pack.Additive)
	assert.Equal(t, "", pack.AdditivePosition)
}

func TestLoadPack_PreambleMDLoadedWhenPresent(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: base\nname: Base\ndescription: Base pack\ntags: []\nprofiles: []\nweight: 0\nbase: true\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "preamble.md"), []byte("> Prefer sap-devs commands."), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "> Prefer sap-devs commands.", p.PreambleMD)
}

func TestLoadPack_PreambleMDEmptyWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: base\nname: Base\ndescription: Base pack\ntags: []\nprofiles: []\nweight: 0\nbase: true\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	// No preamble.md file created

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Empty(t, p.PreambleMD)
}

func TestLoadPack_HooksLoadedWhenPresent(t *testing.T) {
	dir := t.TempDir()
	packYAML := "id: base\nname: Base\ndescription: Base pack\ntags: []\nprofiles: []\nweight: 0\nbase: true\n"
	hookYAML := `- id: tip-on-session-start
  event: sessionStart
  command: "sap-devs tip --markdown"
  tools:
    - claude-code
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(packYAML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hook.yaml"), []byte(hookYAML), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	require.Len(t, p.Hooks, 1)
	assert.Equal(t, "tip-on-session-start", p.Hooks[0].ID)
	assert.Equal(t, "sessionStart", p.Hooks[0].Event)
	assert.Equal(t, "sap-devs tip --markdown", p.Hooks[0].Command)
	assert.Equal(t, []string{"claude-code"}, p.Hooks[0].Tools)
	assert.Equal(t, "base", p.Hooks[0].PackID)
}

func TestLoadPack_HooksEmptyWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	packYAML := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(packYAML), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Empty(t, p.Hooks)
}

func TestLoadPack_YouTubeSourcesLoaded(t *testing.T) {
	dir := t.TempDir()
	packYAML := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	youtubeYAML := `- id: cap-tutorials
  type: playlist
  name: CAP Tutorial Series
  playlist_id: PLxxxxxxxxxxx
  tags: [tutorial, cap]
- id: cap-walkthrough
  type: video
  name: CAP Full Walkthrough
  video_id: dQw4w9WgXcQ
  tags: [tutorial]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(packYAML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "youtube.yaml"), []byte(youtubeYAML), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	require.Len(t, p.YouTubeSources, 2)
	assert.Equal(t, "cap-tutorials", p.YouTubeSources[0].ID)
	assert.Equal(t, "playlist", p.YouTubeSources[0].Type)
	assert.Equal(t, "PLxxxxxxxxxxx", p.YouTubeSources[0].PlaylistID)
	assert.Equal(t, "cap", p.YouTubeSources[0].PackID)
	assert.Equal(t, "cap-walkthrough", p.YouTubeSources[1].ID)
	assert.Equal(t, "video", p.YouTubeSources[1].Type)
	assert.Equal(t, "dQw4w9WgXcQ", p.YouTubeSources[1].VideoID)
}

func TestLoadPack_YouTubeSourcesEmptyWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	packYAML := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(packYAML), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Empty(t, p.YouTubeSources)
}

func TestLoadPack_ConstraintsLoadedWhenPresent(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "constraints.md"), []byte("1. Never write raw SQL"), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "1. Never write raw SQL", p.Constraints.Core)
}

func TestLoadPack_ConstraintsEmptyWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))

	p, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, content.VerbositySections{}, p.Constraints)
}

func TestLoadPack_ConstraintsLocaleVariant(t *testing.T) {
	dir := t.TempDir()
	yaml := "id: cap\nname: CAP\ndescription: CAP\ntags: []\nprofiles: []\nweight: 100\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "constraints.md"), []byte("English constraints"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "constraints.de.md"), []byte("German constraints"), 0644))

	p, err := content.LoadPack(dir, "de")
	require.NoError(t, err)
	assert.Equal(t, "German constraints", p.Constraints.Core)

	p, err = content.LoadPack(dir, "")
	require.NoError(t, err)
	assert.Equal(t, "English constraints", p.Constraints.Core)

	p, err = content.LoadPack(dir, "en")
	require.NoError(t, err)
	assert.Equal(t, "English constraints", p.Constraints.Core)
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
