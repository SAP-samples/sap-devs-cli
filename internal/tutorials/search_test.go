package tutorials_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func fixtureIndex() []tutorials.TutorialMeta {
	return []tutorials.TutorialMeta{
		{Slug: "cap-getting-started", Title: "Getting Started with CAP", Description: "Learn CAP basics", Level: "beginner", Tags: []string{"tutorial>beginner", "software-product>cap"}},
		{Slug: "fiori-elements-create", Title: "Create a Fiori Elements App", Description: "Build Fiori UI", Level: "intermediate", Tags: []string{"tutorial>intermediate", "topic>fiori"}},
		{Slug: "abap-rap-bo", Title: "RAP Business Object", Description: "Create a RAP BO", Level: "advanced", Tags: []string{"tutorial>advanced", "topic>abap", "topic>rap"}},
	}
}

func TestSearch_TitleMatch(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "Fiori")
	require.Len(t, got, 1)
	assert.Equal(t, "fiori-elements-create", got[0].Slug)
}

func TestSearch_DescriptionMatch(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "RAP BO")
	require.Len(t, got, 1)
	assert.Equal(t, "abap-rap-bo", got[0].Slug)
}

func TestSearch_SlugMatch(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "cap-getting")
	require.Len(t, got, 1)
}

func TestSearch_TagMatch(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "rap")
	require.Len(t, got, 1)
	assert.Equal(t, "abap-rap-bo", got[0].Slug)
}

func TestSearch_CaseInsensitive(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "CAP")
	assert.True(t, len(got) >= 1)
}

func TestSearch_NoResults(t *testing.T) {
	got := tutorials.Search(fixtureIndex(), "zzznomatch")
	assert.Empty(t, got)
}

func TestFilterByLevel(t *testing.T) {
	got := tutorials.FilterByLevel(fixtureIndex(), "beginner")
	require.Len(t, got, 1)
	assert.Equal(t, "cap-getting-started", got[0].Slug)
}

func TestFilterByTags(t *testing.T) {
	got := tutorials.FilterByTags(fixtureIndex(), []string{"fiori"})
	require.Len(t, got, 1)
	assert.Equal(t, "fiori-elements-create", got[0].Slug)
}

func TestFindBySlug(t *testing.T) {
	got := tutorials.FindBySlug(fixtureIndex(), "abap-rap-bo")
	require.NotNil(t, got)
	assert.Equal(t, "RAP Business Object", got.Title)
}

func TestFindBySlug_NotFound(t *testing.T) {
	got := tutorials.FindBySlug(fixtureIndex(), "nonexistent")
	assert.Nil(t, got)
}
