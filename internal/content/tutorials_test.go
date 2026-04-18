package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func fixtureTutorialPacks() []*content.Pack {
	return []*content.Pack{
		{
			ID: "cap",
			TutorialRefs: []content.TutorialRef{
				{Slug: "cap-getting-started", Featured: true, PackID: "cap"},
				{Slug: "cap-deploy-cf", Featured: false, PackID: "cap"},
			},
		},
		{
			ID: "abap",
			TutorialRefs: []content.TutorialRef{
				{Slug: "abap-rap-create", Featured: true, PackID: "abap"},
			},
		},
	}
}

func TestFlattenTutorialRefs(t *testing.T) {
	got := content.FlattenTutorialRefs(fixtureTutorialPacks())
	require.Len(t, got, 3)
	assert.Equal(t, "cap-getting-started", got[0].Slug)
	assert.Equal(t, "cap", got[0].PackID)
}

func TestFlattenTutorialRefs_NilInput(t *testing.T) {
	got := content.FlattenTutorialRefs(nil)
	assert.Empty(t, got)
}

func TestFilterTutorialRefsByPack(t *testing.T) {
	got := content.FilterTutorialRefsByPack(fixtureTutorialPacks(), "cap")
	require.Len(t, got, 2)
	assert.Equal(t, "cap-getting-started", got[0].Slug)
}

func TestFilterTutorialRefsByPack_NotFound(t *testing.T) {
	got := content.FilterTutorialRefsByPack(fixtureTutorialPacks(), "nonexistent")
	assert.Nil(t, got)
}

func TestFindTutorialRef(t *testing.T) {
	got := content.FindTutorialRef(fixtureTutorialPacks(), "abap-rap-create")
	require.NotNil(t, got)
	assert.True(t, got.Featured)
}

func TestFindTutorialRef_NotFound(t *testing.T) {
	got := content.FindTutorialRef(fixtureTutorialPacks(), "nonexistent")
	assert.Nil(t, got)
}
