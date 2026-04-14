package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

// fixture builds an in-memory pack slice for testing without touching the filesystem.
func fixturePacks() []*content.Pack {
	return []*content.Pack{
		{
			ID: "cap",
			Resources: []content.Resource{
				{ID: "cap/docs", Title: "CAP Documentation", URL: "https://cap.cloud.sap/docs", Type: "official-docs", Tags: []string{"reference"}, PackID: "cap"},
				{ID: "cap/samples", Title: "CAP Samples on GitHub", URL: "https://github.com/SAP-samples/cap", Type: "sample", Tags: []string{"examples", "reference"}, PackID: "cap"},
			},
		},
		{
			ID: "abap",
			Resources: []content.Resource{
				{ID: "abap/adt", Title: "ABAP Development Tools", URL: "https://tools.hana.ondemand.com", Type: "tool", Tags: []string{"ide"}, PackID: "abap"},
			},
		},
	}
}

func TestFlattenResources(t *testing.T) {
	got := content.FlattenResources(fixturePacks())
	require.Len(t, got, 3)
	assert.Equal(t, "cap/docs", got[0].ID)
	assert.Equal(t, "cap", got[0].PackID)
	assert.Equal(t, "cap/samples", got[1].ID)
	assert.Equal(t, "cap", got[1].PackID)
	assert.Equal(t, "abap/adt", got[2].ID)
	assert.Equal(t, "abap", got[2].PackID)
}

func TestFlattenResources_NilInput(t *testing.T) {
	got := content.FlattenResources(nil)
	assert.Empty(t, got)
}

func TestFilterResources_TitleMatch(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FilterResources(resources, "documentation")
	require.Len(t, got, 1)
	assert.Equal(t, "cap/docs", got[0].ID)
}

func TestFilterResources_TagMatch(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FilterResources(resources, "ide")
	require.Len(t, got, 1)
	assert.Equal(t, "abap/adt", got[0].ID)
}

func TestFilterResources_TypeMatch(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FilterResources(resources, "sample")
	require.Len(t, got, 1)
	assert.Equal(t, "cap/samples", got[0].ID)
}

func TestFilterResources_CaseInsensitive(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	// "CAP" matches "CAP Documentation" and "CAP Samples on GitHub"
	got := content.FilterResources(resources, "CAP")
	assert.Len(t, got, 2)
}

func TestFilterResources_NoMatch(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FilterResources(resources, "zzznomatch")
	assert.Empty(t, got)
}

func TestFindResource_Found(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FindResource(resources, "cap/samples")
	require.NotNil(t, got)
	assert.Equal(t, "CAP Samples on GitHub", got.Title)
}

func TestFindResource_NotFound(t *testing.T) {
	resources := content.FlattenResources(fixturePacks())
	got := content.FindResource(resources, "nonexistent/id")
	assert.Nil(t, got)
}

// TestLoadPackSetsPackID verifies that LoadPack populates PackID on each resource.
func TestLoadPackSetsPackID(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(`
id: mypak
name: My Pack
description: Test pack
`), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.yaml"), []byte(`
- id: mypak/link
  title: My Link
  url: https://example.com
  type: official-docs
  tags: [test]
`), 0644))

	pack, err := content.LoadPack(dir, "")
	require.NoError(t, err)
	require.Len(t, pack.Resources, 1)
	assert.Equal(t, "mypak", pack.Resources[0].PackID)
}
